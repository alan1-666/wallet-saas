# scan-account-service

Dedicated account-model scanner.

- Start command: `go run ./cmd`
- Runtime mode: `account`
- Responsibilities:
  - scan `account` model deposits
  - reconcile reorg candidates
  - dispatch staged outbox events to wallet-core / project callbacks
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
- `SCAN_ACCOUNT_MAX_EMPTY_PAGES` stop early after consecutive empty pages with advancing cursors (default: `2`)
- `SCAN_ACCOUNT_CURSOR_STALL_GUARD` stop when the same cursor reappears in one scan loop (default: `1`)
- `SCAN_WATCH_LIMIT` max watched rows per tick (default: `500`)
- `SCAN_ADDR_CONCURRENCY` concurrent scan workers (default: `8`)
- `SCAN_CHAIN_DEFAULT_QPS` default per-chain request rate to chain-gateway (default: `2`)
- `SCAN_CHAIN_DEFAULT_CONCURRENCY` default per-chain concurrent requests to chain-gateway (default: `2`)
- `SCAN_CHAIN_QPS_MAP` per-chain QPS override, format `chain=qps` (e.g. `ethereum=3,solana=1.5`)
- `SCAN_CHAIN_CONCURRENCY_MAP` per-chain concurrency override, format `chain=n` (e.g. `ethereum=3,solana=1`)
- `SCAN_CHAIN_RETRY_MAX_ATTEMPTS` chain-gateway retry attempts for transient errors / 429 (default: `3`)
- `SCAN_CHAIN_RETRY_BASE_MS` base retry delay in ms (default: `250`)
- `SCAN_CHAIN_RETRY_MAX_MS` max retry delay in ms (default: `4000`)
- `SCAN_REORG_WINDOW` reorg reconcile window (default: `6`)
- `SCAN_REORG_CANDIDATE_LIMIT` max reorg candidates per tick (default: `500`)
- `SCAN_REORG_NOT_FOUND_THRESHOLD` tx not-found threshold before internal state flips to `REORGED` (default: `3`)
- `SCAN_SWEEP_MIN_BALANCE` auto-sweep threshold, strict `>` compare (default: `50`)
- `SCAN_WALLET_CORE_TIMEOUT_MS` API timeout (default: `10000`)
- `SCAN_CHAIN_GATEWAY_TIMEOUT_MS` chain-gateway timeout (default: `10000`)

## Storage
- `scan_watch_addresses`: watch targets (`model=account`)
- `scan_checkpoints`: cursor/last tx
- `scan_seen_events`: event state + dedup key. Internal status flow is `SEEN -> PENDING -> CONFIRMED -> FINALIZED`, with reorg rollback to `REORGED`.
- `scan_event_outbox`: pending/failed staged outbound events. Event types include `DEPOSIT_NOTIFY`, `PROJECT_DEPOSIT_NOTIFY`, `SWEEP_TRIGGER`, `SWEEP_RUN`.

## Notes
- EVM native assets use chain-wide block-window scans with shared checkpoints; Solana / Tron stay on address-history style scans.
- `auto_sweep=false` now really disables sweep triggering for that watch address.
- Deposit ledger notify is independent from project callback / sweep execution. A sweep failure will no longer block the deposit event from reaching `DONE`.
