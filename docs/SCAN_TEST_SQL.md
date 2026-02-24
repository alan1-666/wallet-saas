# Scan Service Test SQL

## 1) Add account-model watch address

```sql
INSERT INTO scan_watch_addresses (
  tenant_id,
  account_id,
  model,
  chain,
  coin,
  network,
  address,
  min_confirmations,
  treasury_account_id,
  auto_sweep,
  sweep_threshold,
  active
) VALUES (
  't1',
  'a1',
  'account',
  'ethereum',
  'ETH',
  'mainnet',
  '0xdeposit_a1',
  1,
  'treasury-main',
  TRUE,
  '1',
  TRUE
)
ON CONFLICT (model, chain, coin, network, address, tenant_id, account_id)
DO UPDATE SET
  model = EXCLUDED.model,
  min_confirmations = EXCLUDED.min_confirmations,
  network = EXCLUDED.network,
  treasury_account_id = EXCLUDED.treasury_account_id,
  auto_sweep = EXCLUDED.auto_sweep,
  sweep_threshold = EXCLUDED.sweep_threshold,
  active = EXCLUDED.active,
  updated_at = NOW();
```

## 2) (Optional) Add utxo-model watch address

```sql
INSERT INTO scan_watch_addresses (
  tenant_id, account_id, model, chain, coin, network, address,
  min_confirmations, treasury_account_id, auto_sweep, sweep_threshold, active
) VALUES (
  't1', 'a2', 'utxo', 'bitcoin', 'BTC', 'mainnet', 'bc1qxxxx',
  1, 'treasury-main', TRUE, '1', TRUE
)
ON CONFLICT (model, chain, coin, network, address, tenant_id, account_id)
DO UPDATE SET updated_at = NOW(), active = EXCLUDED.active;
```

## 3) Verify scan checkpoints + dedup + ledger results

```sql
SELECT *
FROM scan_checkpoints
ORDER BY updated_at DESC
LIMIT 20;

SELECT id, tenant_id, account_id, model, chain, coin, network, address, tx_hash, event_index, created_at
FROM scan_seen_events
ORDER BY id DESC
LIMIT 20;

SELECT id, tenant_id, account_id, order_id, tx_hash, amount, status, created_at
FROM deposit_events
ORDER BY id DESC
LIMIT 10;

SELECT id, tenant_id, sweep_order_id, from_account_id, treasury_account_id, asset, amount, status, created_at
FROM sweep_orders
ORDER BY id DESC
LIMIT 10;

SELECT tenant_id, account_id, asset, available, frozen, updated_at
FROM ledger_balances
WHERE tenant_id = 't1'
ORDER BY account_id, asset;
```
