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
  ('t1', 'hd:ecdsa:ethereum:1:0:0', 'ACTIVE'),
  ('t1', 'hd:ecdsa:bitcoin:1:0:0', 'ACTIVE')
ON CONFLICT (tenant_id, key_id) DO NOTHING;
```

## 3) Verify

```sql
SELECT * FROM api_tokens ORDER BY id DESC;
SELECT * FROM tenant_keys ORDER BY id DESC;
```
