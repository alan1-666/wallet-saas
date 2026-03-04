# scan-account-service

Dedicated account-model scanner.

- Start command: `go run ./cmd`
- Runtime mode: `account`
- Responsibilities:
  - scan `account` model deposits
  - reconcile reorg candidates
  - dispatch outbox events to wallet-core
  - check outgoing tx finality (withdraw/sweep)

## Env
- `SCAN_DB_DSN` PostgreSQL DSN
- `WALLET_CORE_HTTP_ADDR` API entry URL (recommended `api-gateway`)
- `CHAIN_GATEWAY_GRPC_ADDR` chain-gateway gRPC addr (default: `127.0.0.1:9082`)
- `SCAN_API_TOKEN` API token (Bearer)
- `PROJECT_NOTIFY_BASE_URL` project callback base URL (e.g. `http://fundex-gateway:8083`)
- `PROJECT_NOTIFY_TOKEN` optional callback token (sent via `x-deposit-token`)
- `PROJECT_NOTIFY_TIMEOUT_MS` callback timeout ms (default: `5000`)
- `PROJECT_NOTIFY_CHAIN_ID_MAP` chain mapping, format `chain:network=chain_id` (e.g. `sol:mainnet=100000`)
- `PROJECT_NOTIFY_DEFAULT_CHAIN_ID` fallback chain id when map miss (default: `0` disabled)
- `SCAN_INTERVAL_SECONDS` poll interval (default: `5`)
- `SCAN_ACCOUNT_PAGE_SIZE` account scan page size (default: `50`)
- `SCAN_ACCOUNT_MAX_PAGES` account scan max pages per tick (default: `2`)
- `SCAN_WATCH_LIMIT` max watched rows per tick (default: `500`)
- `SCAN_ADDR_CONCURRENCY` concurrent scan workers (default: `8`)
- `SCAN_REORG_WINDOW` reorg reconcile window (default: `6`)
- `SCAN_REORG_CANDIDATE_LIMIT` max reorg candidates per tick (default: `500`)
- `SCAN_REORG_NOT_FOUND_THRESHOLD` tx not-found threshold before `REVERTED` (default: `3`)
- `SCAN_SWEEP_MIN_BALANCE` auto-sweep threshold, strict `>` compare (default: `50`)
- `SCAN_WALLET_CORE_TIMEOUT_MS` API timeout (default: `10000`)
- `SCAN_CHAIN_GATEWAY_TIMEOUT_MS` chain-gateway timeout (default: `10000`)

## Storage
- `scan_watch_addresses`: watch targets (`model=account`)
- `scan_checkpoints`: cursor/last tx
- `scan_seen_events`: event state + dedup key
- `scan_event_outbox`: pending/failed outbound events
