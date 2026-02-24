# Bootstrap SQL

## 1) API token

```sql
INSERT INTO api_tokens (token, tenant_id, can_withdraw, can_deposit, can_sweep, status)
VALUES
  ('token_t1_full', 't1', TRUE, TRUE, TRUE, 'ACTIVE'),
  ('token_t1_read', 't1', FALSE, FALSE, FALSE, 'ACTIVE')
ON CONFLICT (token) DO NOTHING;
```

## 2) Sign key permission

```sql
INSERT INTO tenant_keys (tenant_id, key_id, status)
VALUES
  ('t1', 'pubkey_t1_main', 'ACTIVE'),
  ('t1', 'pubkey_t1_utxo_1', 'ACTIVE')
ON CONFLICT (tenant_id, key_id) DO NOTHING;
```

## 3) Risk rules

```sql
INSERT INTO risk_rules (tenant_id, chain, coin, max_amount, priority, status)
VALUES
  ('t1', 'bitcoin', 'BTC', '500000000', 20, 'ACTIVE'),
  ('t1', 'ethereum', 'ETH', '1000000000000000000', 20, 'ACTIVE'),
  ('*', '*', '*', '1000000000000', 0, 'ACTIVE')
ON CONFLICT (tenant_id, chain, coin) DO UPDATE
SET max_amount = EXCLUDED.max_amount,
    priority = EXCLUDED.priority,
    status = EXCLUDED.status,
    updated_at = NOW();
```

## 4) Verify

```sql
SELECT * FROM api_tokens ORDER BY id DESC;
SELECT * FROM tenant_keys ORDER BY id DESC;
SELECT * FROM risk_rules ORDER BY id DESC;
```
