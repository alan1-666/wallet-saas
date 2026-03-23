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

## Responsibility boundary
- `wallet-core` focuses on orchestration/state transitions/ledger.
- auth, tenant isolation, sign-key permission, idempotency, anti-replay, audit are handled by `api-gateway`.
- account status still enforced in `wallet-core` (`ACTIVE` required for withdraw/deposit_notify/sweep/address_create).

### Request example
```json
{
  "tenant_id": "t1",
  "account_id": "a1",
  "order_id": "o1",
  "key_id": "hd:ecdsa:ethereum:12:0:0",
  "key_ids": ["hd:ecdsa:bitcoin:12:0:0", "hd:ecdsa:bitcoin:12:0:1"],
  "sign_type": "ecdsa",
  "chain": "ethereum",
  "network": "mainnet",
  "coin": "ETH",
  "from": "0xfrom",
  "to": "0xto",
  "amount": "1000"
}
```
- `key_id` / `key_ids` are HD derivation key IDs, not raw public keys.
- `wallet-core` resolves the matching public key from registry before broadcast.
- for ECDSA chains, address creation now prefers account-level public derivation material from `sign-service` and derives child public keys locally instead of asking the signer to derive a child private key just to export its public key.
- for EdDSA chains, address creation still uses direct child public-key derivation from `sign-service` because the current hardened derivation path does not support public-only child derivation.
- when a brand-new address is derived, the response also includes `custody_scheme` from `sign-service`, so callers can see which signer custody backend is serving the request.

### Solana token withdraw request example
```json
{
  "tenant_id": "t1",
  "account_id": "a1",
  "order_id": "sol-usdc-1",
  "key_id": "hd:eddsa:solana:12:0:0",
  "sign_type": "eddsa",
  "chain": "solana",
  "network": "devnet",
  "coin": "USDC",
  "from": "FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M",
  "to": "7nTj8m6P6xZgJ2EJgYQW1R6cM4zKq8GxSxXy5pX1b2YV",
  "amount": "1500000",
  "contract_address": "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
  "amount_unit": "raw",
  "token_decimals": 6
}
```
- `contract_address`: SPL mint address.
- `amount_unit`: `raw` means smallest unit amount; `display` means UI amount like `1.5`.
- `token_decimals`: optional override. If omitted, `chain-gateway` reads mint decimals on chain.

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
- Response includes `key_id`, `derivation_path`, `change_index`, `address_index`.
- Same `tenant_id + account_id + chain + network` reuses the same derived address. Registering another asset on that chain only adds a new watch row.

### Deposit notify request (state machine)
```json
{
  "tenant_id": "t1",
  "account_id": "a1",
  "order_id": "dep_0xhash_0_a1",
  "chain": "ethereum",
  "network": "sepolia",
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
- state regression is blocked (`CONFIRMED` will not be downgraded back to `PENDING`)
- compatibility aliases: inbound `FINALIZED` is treated as `CONFIRMED`, and `REORGED` is treated as `REVERTED`

### Current limitation
- If `key_ids` length is less than input count, last key will be reused for remaining inputs.

### Ledger flow
- before sign/broadcast: `FreezeWithdraw`
- broadcast success: `ConfirmWithdraw`
- broadcast failure (or sign/build failure): `ReleaseWithdraw`
- on-chain confirmation threshold for withdraw/sweep is taken from `chain_metadata.min_confirmations`
- business risk controls are intentionally out of scope for wallet-core; projects should approve/deny withdrawals before calling the SaaS APIs

## New tables
- `api_tokens`, `tenant_keys`, `audit_logs`
- `idem_requests`
- `deposit_events`, `sweep_orders`
- `ledger_audit_events`
- `ledger_balances`, `ledger_journals`
- `wallet_accounts`, `wallet_addresses`
- `vault_balances`, `vault_journals`

## Env
- `WALLET_CORE_HOST` (default `0.0.0.0`)
- `WALLET_CORE_PORT` (default `8081`)
- `SIGN_SERVICE_ADDR` (default `127.0.0.1:9091`)
- `SIGN_SERVICE_TOKEN` shared internal auth token for `sign-service` (default `dev-sign-token`)
- `CHAIN_GATEWAY_GRPC_ADDR` (default `127.0.0.1:9082`, internal call path)
- `WALLET_DB_DSN` (optional, enables postgres-backed ledger/auth/registry adapters)
