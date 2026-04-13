# scan-account-service 按链分片 + 多实例设计

## 背景

当前 scan-account-service 是单进程扫所有链，存在三个问题：
1. **单点故障** — 进程挂了全链停扫
2. **链间干扰** — 某条链 RPC 慢会占满信号量，拖慢其他链
3. **参数不灵活** — 高频链（BSC）和低频链（Tron）共享同一套并发/轮询参数

目标：通过环境变量指定每个实例负责的链，实现按链分片部署，多实例互不干扰。

---

## 部署模型

```
┌─────────────────────────────────────────────────────────┐
│  scan-evm-main                                          │
│  SCAN_ALLOWED_CHAINS=binance,ethereum,polygon,arbitrum  │
│  SCAN_ADDR_CONCURRENCY=6                                │
│  SCAN_INTERVAL_SECONDS=3                                │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────┼──────────────────────────────────┐
│  scan-solana         │                                  │
│  SCAN_ALLOWED_CHAINS=solana                             │
│  SCAN_ADDR_CONCURRENCY=4                                │
│  SCAN_INTERVAL_SECONDS=2                                │
└──────────────────────┼──────────────────────────────────┘
                       │
┌──────────────────────┼──────────────────────────────────┐
│  scan-misc                                              │
│  SCAN_ALLOWED_CHAINS=tron,cosmos                        │
│  SCAN_ADDR_CONCURRENCY=2                                │
│  SCAN_INTERVAL_SECONDS=10                               │
└──────────────────────┬──────────────────────────────────┘
                       │
                       ▼
              chain-gateway × N（无状态，多实例）
                       │
                       ▼
                  RPC endpoints
```

### 分组原则

| 实例 | 链 | 理由 |
|------|---|------|
| scan-evm-main | BSC、ETH、Polygon、Arbitrum | EVM 同族，共享 eth_getLogs 扫描逻辑，业务量大，需要高并发 |
| scan-solana | Solana | 非 EVM，RPC 模型不同，独立调参 |
| scan-misc | Tron、Cosmos 等 | 用量少，合并跑一个实例节省资源 |

低频链合一个实例，高频链独立或同族合并，按实际业务量灵活调整。

---

## 改动点

### 1. Config 层：新增 `SCAN_ALLOWED_CHAINS`

```go
// config.go
type Config struct {
    // ... 现有字段 ...
    AllowedChains []string // env: SCAN_ALLOWED_CHAINS, 逗号分隔, 空=全部（向后兼容）
}
```

解析逻辑：
- `SCAN_ALLOWED_CHAINS=binance,ethereum,polygon` → `["binance","ethereum","polygon"]`
- 不设或为空 → 不过滤，扫全部链（**向后兼容，现有部署不用改**）
- 统一转小写比较

### 2. Store 层：三处查询加 chain 过滤

所有改动都是在 WHERE 子句追加条件，不改表结构。

#### 2a. ListWatchAddresses

```sql
-- 现有
WHERE sw.active = TRUE AND sw.model = $1
-- 加上
  AND (cardinality($3::text[]) = 0 OR LOWER(sw.chain) = ANY($3::text[]))
```

`$3` 传入 `AllowedChains`，空数组时条件恒真，不影响现有行为。

#### 2b. ListReorgCandidates

```sql
-- 现有
WHERE se.status NOT IN ('REORGED','REVERTED')
-- 加上
  AND (cardinality($N::text[]) = 0 OR LOWER(se.chain) = ANY($N::text[]))
```

#### 2c. ListPendingOutboxEvents

```sql
-- 现有
WHERE status='PENDING' AND next_retry_at <= NOW()
-- 加上
  AND (cardinality($N::text[]) = 0 OR LOWER(chain) = ANY($N::text[]))
```

### 3. Scanner 层：传递 chain 过滤

```go
// scanner.go - Run() 方法
func (s *Scanner) Run(ctx context.Context) {
    // 启动时打印分片信息
    if len(s.AllowedChains) > 0 {
        log.Infof("chain shard mode: scanning %v only", s.AllowedChains)
    } else {
        log.Info("scanning all chains (no shard filter)")
    }
    // tick loop 不变
}

// scanner_deposit.go - scanModel()
func (s *Scanner) scanModel(ctx context.Context, model string) {
    watches, err := s.Store.ListWatchAddresses(ctx, model, s.WatchLimit, s.AllowedChains)
    // 后续分组逻辑不变，因为拿到的 watches 已经过滤过了
}
```

改动路径：`Config → Scanner struct → Store 方法参数`，三层透传即可。

---

## 数据库层：无需改动

现有表已经天然按链隔离：

| 表 | chain 在 key 中 | 说明 |
|---|---|---|
| `scan_watch_addresses` | `(tenant_id, account_id, chain, coin, network, address)` | 已隔离 |
| `scan_checkpoints` | `(tenant_id, account_id, model, chain, coin, network, address)` | 已隔离 |
| `scan_chain_checkpoints` | `(model, chain, coin, network)` | 已隔离 |
| `scan_seen_events` | `(..., chain, coin, network, address, tx_hash, event_index)` | 已隔离 |
| `scan_event_outbox` | `event_key` 包含 chain，`chain` 列可过滤 | 已隔离 |

多个实例读写同一个 DB，各自只操作自己负责的链的行，不会冲突。

---

## 安全性分析

### 同一条链会不会被两个实例同时扫？

| 场景 | 风险 | 应对 |
|------|------|------|
| 运维配错，两个实例都配了 BSC | 重复扫链，浪费 RPC 调用 | outbox 幂等（event_key UNIQUE）+ account CreditBalance refID 幂等，**不会重复入账** |
| 某条链没有任何实例认领 | 该链停止扫描 | 启动时日志打印认领的链 + 运维检查脚本 |
| 不设 SCAN_ALLOWED_CHAINS | 退化为现有行为，扫全部链 | 向后兼容，灰度期可以先不改 |

### 重复扫描为什么不会出事？

```
实例 A 扫到 tx_hash=0xabc → INSERT scan_seen_events → INSERT scan_event_outbox (event_key=dep:t1:bsc:...:0xabc:0)
实例 B 也扫到 tx_hash=0xabc → INSERT scan_seen_events (ON CONFLICT DO NOTHING) → INSERT outbox (UNIQUE 冲突，跳过)
```

三层幂等保障：
1. `scan_seen_events` 复合唯一键 → 不会重复记录
2. `scan_event_outbox.event_key` UNIQUE → 不会重复派发
3. `account.CreditBalance` refType+refID → 不会重复入账

---

## chain-gateway 多实例

chain-gateway 是无状态 RPC 代理，直接多实例部署：

```yaml
# docker-compose 示例
chain-gateway-1:
  image: wallet-saas/chain-gateway
  environment:
    - CHAIN_GATEWAY_GRPC_PORT=9082
  
chain-gateway-2:
  image: wallet-saas/chain-gateway
  environment:
    - CHAIN_GATEWAY_GRPC_PORT=9082
```

scan-service 侧 gRPC 客户端改为多地址：
```
CHAIN_GATEWAY_GRPC_ADDR=chain-gateway-1:9082,chain-gateway-2:9082
```

gRPC 客户端配置 round-robin 负载均衡即可。如果用 K8s 部署，直接 Service 负载均衡，不需要改代码。

---

## 灰度方案

```
阶段 1: 加 SCAN_ALLOWED_CHAINS 参数，不设默认扫全部（合并代码，不改部署）
阶段 2: 新起一个 scan-solana 实例，SCAN_ALLOWED_CHAINS=solana
         原实例加 SCAN_ALLOWED_CHAINS=binance,ethereum,polygon,arbitrum,tron,cosmos
         观察两个实例各自扫链正常，无重复入账
阶段 3: 按需拆分更多实例（EVM 主力 / 低频链合并）
阶段 4: chain-gateway 扩到 2 实例
```

---

## 改动文件清单

| 文件 | 改动 | 估计行数 |
|------|------|---------|
| `internal/config/config.go` | 新增 AllowedChains 字段 + 解析 | ~15 行 |
| `internal/store/postgres.go` | 3 处查询加 chain 过滤参数 | ~20 行 |
| `internal/store/store.go` | Store 接口方法签名加参数 | ~5 行 |
| `internal/worker/scanner.go` | Scanner struct 加 AllowedChains 字段 | ~5 行 |
| `internal/worker/scanner_deposit.go` | scanModel 传 AllowedChains 到 Store | ~3 行 |
| `internal/worker/scanner_reorg.go` | ListReorgCandidates 传 AllowedChains | ~3 行 |
| `internal/worker/scanner_outbox.go` | ListPendingOutboxEvents 传 AllowedChains | ~3 行 |
| `internal/bootstrap/bootstrap.go` | 从 Config 传 AllowedChains 到 Scanner | ~3 行 |

**总计约 60 行改动，不改表结构，不改业务逻辑，向后兼容。**
