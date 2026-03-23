# Control Plane Schema (chain + network)

This project now treats `chain + network` as required fields in runtime flow:
- address registration
- watch rows / scan cursor
- deposit/sweep/withdraw order processing
- chain-gateway endpoint selection

## Core tables

### 1) `rpc_endpoints` (chain-gateway control plane)
Used by `chain-gateway` to select RPC endpoints by `chain + network + model`.

```sql
CREATE TABLE IF NOT EXISTS rpc_endpoints (
  id BIGSERIAL PRIMARY KEY,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  model TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  weight INT NOT NULL DEFAULT 100,
  timeout_ms INT NOT NULL DEFAULT 10000,
  priority INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  fail_count BIGINT NOT NULL DEFAULT 0,
  success_count BIGINT NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  last_heartbeat_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (chain, network, model, endpoint_url)
);

CREATE INDEX IF NOT EXISTS idx_rpc_endpoints_lookup
ON rpc_endpoints (chain, network, model, status, priority, weight);
```

### 2) `chain_policies` (wallet-core policy plane)
Defines finality/reorg strategy by `chain + network`.

- `required_confirmations`: deposit credit threshold
- `safe_depth`: deposit unlock threshold for withdraw (`scan_watch_addresses.unlock_confirmations` is seeded from this field)
- `reorg_window`: fallback reconciliation window / not-found monitoring horizon

```sql
CREATE TABLE IF NOT EXISTS chain_policies (
  id BIGSERIAL PRIMARY KEY,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  required_confirmations BIGINT NOT NULL DEFAULT 1,
  safe_depth BIGINT NOT NULL DEFAULT 1,
  reorg_window BIGINT NOT NULL DEFAULT 6,
  fee_policy TEXT NOT NULL DEFAULT '{}',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (chain, network)
);
```

### 3) `tenant_chain_policies` (api-gateway tenant isolation)
Enforces tenant-level deposit/withdraw/sweep permissions and limits.

```sql
CREATE TABLE IF NOT EXISTS tenant_chain_policies (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  allow_deposit BOOLEAN NOT NULL DEFAULT FALSE,
  allow_withdraw BOOLEAN NOT NULL DEFAULT FALSE,
  allow_sweep BOOLEAN NOT NULL DEFAULT FALSE,
  max_withdraw_amount TEXT NOT NULL DEFAULT '0',
  priority INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, chain, network)
);
```

## Minimal seed example (Sepolia test)

```sql
-- RPC endpoint for EVM account model
INSERT INTO rpc_endpoints (chain, network, model, endpoint_url, weight, timeout_ms, priority, status)
VALUES
('ethereum', 'sepolia', 'account', 'https://eth-sepolia.g.alchemy.com/v2/<KEY>', 100, 10000, 100, 'ACTIVE')
ON CONFLICT (chain, network, model, endpoint_url) DO UPDATE
SET status='ACTIVE', weight=EXCLUDED.weight, timeout_ms=EXCLUDED.timeout_ms, priority=EXCLUDED.priority, updated_at=NOW();

-- finality policy
INSERT INTO chain_policies (chain, network, required_confirmations, safe_depth, reorg_window, fee_policy, enabled)
VALUES ('ethereum', 'sepolia', 1, 1, 12, '{}', TRUE)
ON CONFLICT (chain, network) DO UPDATE
SET required_confirmations=EXCLUDED.required_confirmations,
    safe_depth=EXCLUDED.safe_depth,
    reorg_window=EXCLUDED.reorg_window,
    enabled=EXCLUDED.enabled,
    updated_at=NOW();

-- tenant permission
INSERT INTO tenant_chain_policies (tenant_id, chain, network, allow_deposit, allow_withdraw, allow_sweep, max_withdraw_amount, priority, status)
VALUES ('t1', 'ethereum', 'sepolia', TRUE, TRUE, TRUE, '100000000000000000000', 100, 'ACTIVE')
ON CONFLICT (tenant_id, chain, network) DO UPDATE
SET allow_deposit=EXCLUDED.allow_deposit,
    allow_withdraw=EXCLUDED.allow_withdraw,
    allow_sweep=EXCLUDED.allow_sweep,
    max_withdraw_amount=EXCLUDED.max_withdraw_amount,
    priority=EXCLUDED.priority,
    status=EXCLUDED.status,
    updated_at=NOW();
```

## Outbox events (scan-service)

- `scan_event_outbox` stores scan-produced events with retry metadata.
- idempotency key format:
  - deposit: `dep:{tenant}:{chain}:{network}:{txHash}:{index}`
  - sweep: `sweep:{tenant}:{chain}:{network}:{txHash}:{index}`
- dispatcher marks `DONE` on success, retries with exponential backoff on failure.
