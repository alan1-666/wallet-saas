# scan-service

Unified scan worker for two chain models:
- `account` model: call `ListIncomingTransfers` (gRPC)
- `utxo` model: call `ListIncomingTransfers` (gRPC)

For each observed event, scanner maintains state and notifies wallet-core:
- `PENDING` (confirmations below threshold)
- `CONFIRMED` (meets threshold or unknown confirmations)
- `REVERTED` (adapter returns failed/reverted status)

Sweep triggering rules:
- only when deposit status is `CONFIRMED`
- address balance must be `> SCAN_SWEEP_MIN_BALANCE` (default `50`)
- skip when `account_id == treasury_account_id`

Event dispatch model:
- scanner writes deposit/sweep tasks into `scan_event_outbox`
- outbox worker delivers to wallet-core with retry/backoff
- event key is idempotent by `tenant + chain + network + tx_hash + index`

## Env
- `SCAN_DB_DSN` PostgreSQL DSN
- `WALLET_CORE_HTTP_ADDR` API entry URL (recommended `api-gateway`, not direct wallet-core)
- `CHAIN_GATEWAY_GRPC_ADDR` chain-gateway gRPC addr (default `127.0.0.1:9082`)
- `SCAN_API_TOKEN` API token (sent as Bearer token)
- `SCAN_INTERVAL_SECONDS` poll interval (default: `5`)
- `SCAN_ACCOUNT_PAGE_SIZE` account scan page size (default: `50`)
- `SCAN_ACCOUNT_MAX_PAGES` account scan max pages per tick (default: `2`)
- `SCAN_WATCH_LIMIT` max watch addresses per model each tick (default: `500`)
- `SCAN_ADDR_CONCURRENCY` concurrent address scan workers (default: `8`)
- `SCAN_SWEEP_MIN_BALANCE` auto-sweep threshold, strict `>` compare (default: `50`)
- `SCAN_WALLET_CORE_TIMEOUT_MS` API timeout (default: `10000`)
- `SCAN_CHAIN_GATEWAY_TIMEOUT_MS` chain-gateway gRPC timeout (default: `10000`)

## Storage
- `scan_watch_addresses`: watch targets (`model=account|utxo`)
- `scan_checkpoints`: account cursor checkpoint / last tx
- `scan_seen_events`: event state `(status,confirmations)` + dedup key `(tenant,account,model,chain,coin,network,address,tx_hash,event_index)`
- `scan_event_outbox`: pending/failed outbound events for wallet-core notify/sweep

## Notes
- Scanner only depends on normalized gRPC contract from `chain-gateway`.
- Required confirmations come from watch rows, sourced from `chain_metadata.min_confirmations`.
- Requests with missing `network` are rejected (no implicit `mainnet` default).
