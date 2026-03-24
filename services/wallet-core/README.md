# wallet-core

State orchestration and domain logic.

## Endpoints
- `GET /healthz`
- `POST /v1/withdraw`
- `GET /v1/withdraw/status?tenant_id=...&order_id=...`
- `POST /v1/deposit/notify`
- `POST /v1/sweep/run`
- `POST /v1/treasury/transfer`
- `GET /v1/treasury/transfer/status?tenant_id=...&transfer_order_id=...`
- `GET /v1/treasury/waterline?tenant_id=...&hot_account_id=...&cold_account_id=...&asset=...&hot_balance_cap=...&hot_balance_floor=...`
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
  "treasury_account_id": "treasury-main",
  "cold_account_id": "treasury-cold",
  "hot_balance_cap": "1000000000000000000"
}
```
- Response includes `key_id`, `derivation_path`, `change_index`, `address_index`.
- Same `tenant_id + account_id + chain + network` reuses the same derived address. Registering another asset on that chain only adds a new watch row.
- `treasury_account_id` is the hot-wallet destination for auto sweep. If `cold_account_id` and `hot_balance_cap` are configured, sweep routing moves overflow to cold once the projected hot vault balance exceeds the cap.

### Treasury transfer request example
```json
{
  "tenant_id": "t1",
  "transfer_order_id": "rebalance-hot-1",
  "from_account_id": "treasury-cold",
  "to_account_id": "treasury-main",
  "chain": "ethereum",
  "network": "sepolia",
  "asset": "ETH",
  "amount": "500000000000000000",
  "source_tier": "COLD",
  "destination_tier": "HOT"
}
```
- treasury transfers are intended for cold -> hot and hot -> cold rebalancing
- source vault balance is reserved before broadcast, so repeated retries cannot oversubscribe the same hot/cold pool
- on confirm, the destination vault is credited; on failure, the source vault reservation is rolled back

### Treasury waterline monitoring
- waterline reads current `vault_available` from both hot and cold accounts and compares the hot wallet against `hot_balance_floor` / `hot_balance_cap`
- returned `recommended_action` will be one of `NONE`, `COLD_TO_HOT`, `HOT_TO_COLD`
- `suggested_transfer_amount` is the deficit/excess needed to bring hot vault back inside the configured range

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
  "unlock_confirmations": 12,
  "scan_status": "CONFIRMED",
  "status": "PENDING"
}
```
- status transitions supported: `PENDING -> CONFIRMED -> FINALIZED -> REVERTED`
- `CONFIRMED` credits balance once and increases `withdraw_locked`
- `FINALIZED` unlocks the credited amount for withdraw by decreasing `withdraw_locked`
- `REVERTED` debits the credited balance back once (idempotent), and also unwinds any remaining `withdraw_locked`
- state regression is blocked (`CONFIRMED` will not be downgraded back to `PENDING`, `FINALIZED` is sticky until `REVERTED`)

### Current limitation
- If `key_ids` length is less than input count, last key will be reused for remaining inputs.

### Ledger flow
- withdraw create now enqueues a job and freezes funds in one transaction
- sweep and treasury transfer broadcasts now use the signer/chain path without creating withdraw freezes, so internal treasury motion no longer mutates user ledger balances
- dispatcher status flow is `QUEUED -> PROCESSING -> BROADCASTED -> CONFIRMED`, with failure path `PROCESSING -> QUEUED(retry)` and terminal failure `PROCESSING -> RELEASED/FAILED`
- stale `PROCESSING` jobs are re-claimed after lease timeout; retries use exponential backoff until `WALLET_WITHDRAW_DISPATCH_MAX_ATTEMPTS`
- same `from_address` is serialized only while a job is actively `PROCESSING`; once a tx is `BROADCASTED`, the next queued job for that address can be picked and will rely on the chain gateway's pending nonce resolution
- dispatcher can process different source addresses in parallel via `WALLET_WITHDRAW_DISPATCH_PARALLELISM`
- EVM `BROADCASTED` withdraws with `0` confirmations can be auto-accelerated: the latest fixed-nonce unsigned payload is persisted, gas is bumped, and a replacement tx is broadcast with the same nonce
- replacement tx hashes are tracked in `withdraw_tx_attempts`; scan-service watches both current and replaced hashes so an older hash confirming first will still settle the withdraw correctly
- broadcast success: `ConfirmWithdraw`
- broadcast failure (or sign/build failure): `ReleaseWithdraw`
- withdraw status query now includes queue-side `queue_status`, `attempt_count`, `last_error` for debugging queued dispatch failures
- on-chain confirmation threshold for withdraw/sweep is taken from `chain_metadata.min_confirmations`
- deposit credit threshold comes from `required_confirmations`; unlock threshold comes from `unlock_confirmations` and defaults to `chain_policies.safe_depth`
- business risk controls are intentionally out of scope for wallet-core; projects should approve/deny withdrawals before calling the SaaS APIs

## New tables
- `api_tokens`, `tenant_keys`, `audit_logs`
- `idem_requests`
- `deposit_events`, `sweep_orders`, `withdraw_jobs`
- `treasury_transfer_orders`
- `withdraw_tx_attempts`
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
- `WALLET_WITHDRAW_DISPATCH_INTERVAL_MS` background withdraw dispatcher poll interval (default `1000`)
- `WALLET_WITHDRAW_DISPATCH_BATCH` background withdraw dispatcher batch size (default `8`)
- `WALLET_WITHDRAW_DISPATCH_PARALLELISM` max concurrent withdraw executions per dispatcher loop across different `from_address` values (default `4`)
- `WALLET_WITHDRAW_DISPATCH_MAX_ATTEMPTS` max automatic withdraw dispatch attempts before release/fail (default `5`)
- `WALLET_WITHDRAW_DISPATCH_BASE_BACKOFF_MS` first retry delay for dispatcher failures (default `1000`)
- `WALLET_WITHDRAW_DISPATCH_MAX_BACKOFF_MS` max retry delay cap for dispatcher failures (default `30000`)
- `WALLET_WITHDRAW_ACCELERATE_BATCH` max number of stale `BROADCASTED` EVM withdraws to examine per loop (default `8`)
- `WALLET_WITHDRAW_ACCELERATE_AFTER_MS` age threshold before a zero-confirmation EVM withdraw is considered stuck and eligible for replacement (default `60000`)
- `WALLET_WITHDRAW_ACCELERATE_MAX_ATTEMPTS` max automatic replacement broadcasts per withdraw order (default `3`)
- `WALLET_WITHDRAW_ACCELERATE_GAS_BUMP_BPS` gas bump used for replacement txs, in basis points (default `2000`, i.e. `20%`)
