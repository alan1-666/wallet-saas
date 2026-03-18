# Wallet-SaaS Testnet 6-Chain Runbook (HD / Deposit / Withdraw / Sweep)

目标：在测试网先跑通以下链路：
- 链：`ethereum(sepolia)` `binance(testnet)` `polygon(amoy)` `arbitrum(sepolia)` `solana(devnet)` `tron(nile)`
- 功能：HD 地址创建、充值入账、提现、归集（sweep）

## 1. 准备 RPC 控制面数据

```bash
psql "$WALLET_DB_DSN" -f deploy/seed_rpc_testnet.sql
```

## 2. 启动服务（确保 WALLET_DB_DSN 可用）

至少启动：
- `services/sign-service`
- `services/chain-gateway`
- `services/wallet-core`
- （可选）`services/api-gateway`

## 3. 创建账户

```bash
curl -s -X POST http://127.0.0.1:8081/v1/account/upsert \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"t1","account_id":"u1001","status":"ACTIVE"}'

curl -s -X POST http://127.0.0.1:8081/v1/account/upsert \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"t1","account_id":"treasury-main","status":"ACTIVE"}'
```

## 4. HD 地址创建（示例）

> EVM（eth/bnb/polygon/arbitrum）用 `ecdsa`；Solana 用 `eddsa`；Tron 用 `ecdsa`。

```bash
# ETH sepolia
curl -s -X POST http://127.0.0.1:8081/v1/address/create -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","chain":"ethereum","coin":"ETH","network":"sepolia","sign_type":"ecdsa","model":"account"
}'

# BNB testnet
curl -s -X POST http://127.0.0.1:8081/v1/address/create -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","chain":"binance","coin":"BNB","network":"testnet","sign_type":"ecdsa","model":"account"
}'

# Polygon amoy
curl -s -X POST http://127.0.0.1:8081/v1/address/create -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","chain":"polygon","coin":"MATIC","network":"amoy","sign_type":"ecdsa","model":"account"
}'

# Arbitrum sepolia
curl -s -X POST http://127.0.0.1:8081/v1/address/create -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","chain":"arbitrum","coin":"ETH","network":"sepolia","sign_type":"ecdsa","model":"account"
}'

# Solana devnet
curl -s -X POST http://127.0.0.1:8081/v1/address/create -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","chain":"solana","coin":"SOL","network":"devnet","sign_type":"eddsa","model":"account"
}'

# Tron nile
curl -s -X POST http://127.0.0.1:8081/v1/address/create -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","chain":"tron","coin":"TRX","network":"nile","sign_type":"ecdsa","model":"account"
}'
```

## 5. 充值入账（deposit notify）

```bash
curl -s -X POST http://127.0.0.1:8081/v1/deposit/notify -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","order_id":"dep_eth_001","chain":"ethereum","network":"sepolia","coin":"ETH",
  "amount":"10000000000000000","tx_hash":"0xabc...","from_address":"0xfrom...","to_address":"0xto...",
  "confirmations":2,"required_confirmations":1,"status":"CONFIRMED"
}'
```

## 6. 提现（withdraw）

> `key_id` 来自 `/v1/address/create` 返回。

```bash
curl -s -X POST http://127.0.0.1:8081/v1/withdraw -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","account_id":"u1001","order_id":"wd_eth_001","chain":"ethereum","network":"sepolia","coin":"ETH",
  "key_id":"hd:ecdsa:ethereum:...","sign_type":"ecdsa","from":"0xfrom...","to":"0xto...","amount":"1000000000000000"
}'
```

## 7. 归集（sweep）

```bash
curl -s -X POST http://127.0.0.1:8081/v1/sweep/run -H 'Content-Type: application/json' -d '{
  "tenant_id":"t1","from_account_id":"u1001","treasury_account_id":"treasury-main",
  "chain":"ethereum","network":"sepolia","asset":"ETH","amount":"1000000000000000","sweep_order_id":"sw_eth_001"
}'
```

## 8. 核验

- 查询余额：`GET /v1/balance?tenant_id=t1&account_id=u1001&asset=ETH`
- 查询提现状态：`GET /v1/withdraw/status?tenant_id=t1&order_id=wd_eth_001`
- 查询账户地址：`GET /v1/account/addresses?tenant_id=t1&account_id=u1001`

---

> 备注：当前项目允许直接改代码、无需兼容老版本。该 runbook 基于测试网配置，适合联调与简历演示。