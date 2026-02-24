# wallet-core

State orchestration and domain logic.

## Endpoints
- `GET /healthz`
- `POST /v1/withdraw`
- `GET /v1/withdraw/status?tenant_id=...&order_id=...`
- `POST /v1/deposit/notify`
- `POST /v1/sweep/run`
- `GET /v1/balance?tenant_id=...&account_id=...&asset=...`
- `POST /v1/address/create`
- `POST /v1/account/upsert`
- `GET /v1/account/get?tenant_id=...&account_id=...`
- `GET /v1/account/list?tenant_id=...`
- `GET /v1/account/addresses?tenant_id=...&account_id=...`
- `GET /v1/account/assets?tenant_id=...&account_id=...`

## Security / Idempotency
- Header `Authorization: Bearer <api_token>`
- Header `X-Request-ID: <unique-request-id>` (idempotency key)
- Body `tenant_id` must match token tenant
- Account status must be `ACTIVE` for withdraw/deposit_notify/sweep/address_create

### Request example
```json
{
  "tenant_id": "t1",
  "account_id": "a1",
  "order_id": "o1",
  "key_id": "<public_key>",
  "key_ids": ["<pubkey-input-0>", "<pubkey-input-1>"],
  "sign_type": "ecdsa",
  "chain": "ethereum",
  "network": "mainnet",
  "coin": "ETH",
  "from": "0xfrom",
  "to": "0xto",
  "amount": "1000"
}
```

### UTXO request extension
- `fee`: transaction fee string
- `vin[]`: `{hash,index,amount,address}`
- `vout[]`: `{address,amount,index}`

### Address create request example
```json
{
  "tenant_id": "t1",
  "account_id": "a1",
  "chain": "ethereum",
  "coin": "ETH",
  "network": "mainnet",
  "sign_type": "ecdsa",
  "address_type": "",
  "model": "account",
  "min_confirmations": 1,
  "auto_sweep": false,
  "sweep_threshold": "1",
  "treasury_account_id": "treasury-main"
}
```

### Deposit notify request (state machine)
```json
{
  "tenant_id": "t1",
  "account_id": "a1",
  "order_id": "dep_0xhash_0_a1",
  "chain": "ethereum",
  "coin": "ETH",
  "amount": "1000000000000000",
  "tx_hash": "0xhash",
  "from_address": "0xfrom",
  "to_address": "0xto",
  "confirmations": 2,
  "required_confirmations": 6,
  "status": "PENDING"
}
```
- status transitions supported: `PENDING -> CONFIRMED -> REVERTED`
- `CONFIRMED` credits balance once; `REVERTED` debits it back once (idempotent)

### Current limitation
- If `key_ids` length is less than input count, last key will be reused for remaining inputs.

### Ledger flow
- before sign/broadcast: `FreezeWithdraw`
- broadcast success: `ConfirmWithdraw`
- broadcast failure (or sign/build failure): `ReleaseWithdraw`

### Risk rules
- Table: `risk_rules`
- Match priority: `tenant_id > chain > coin > priority` (supports `*` wildcard).
- Default rule: `(*,*,*)` uses `RISK_MAX_WITHDRAW_AMOUNT`.
- Account-level table: `account_risk_limits` (`tenant_id + account_id + chain + coin`) can further narrow per-account withdraw limits.

## New tables
- `api_tokens`, `tenant_keys`, `audit_logs`
- `idem_requests`
- `deposit_events`, `sweep_orders`
- `ledger_balances`, `ledger_journals`
- `wallet_accounts`, `wallet_addresses`
- `vault_balances`, `vault_journals`
- `risk_rules`, `account_risk_limits`, `risk_events`

## Env
- `WALLET_CORE_HOST` (default `0.0.0.0`)
- `WALLET_CORE_PORT` (default `8081`)
- `SIGN_SERVICE_ADDR` (default `127.0.0.1:9091`)
- `CHAIN_GATEWAY_HTTP_ADDR` (default `http://127.0.0.1:8082`)
- `CHAIN_GATEWAY_GRPC_ADDR` (default `127.0.0.1:9082`, internal call path)
- `WALLET_DB_DSN` (optional, enables postgres risk+ledger adapters)
- `RISK_MAX_WITHDRAW_AMOUNT` (default `1000000000000`)
