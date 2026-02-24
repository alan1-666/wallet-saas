# scan-service

Unified scan worker for two chain models:
- `account` model: query `/v1/chain/tx-by-address`
- `utxo` model: query `/v1/chain/unspent-outputs`

For each observed event, scanner maintains state and notifies wallet-core:
- `PENDING` (confirmations below threshold)
- `CONFIRMED` (meets threshold or unknown confirmations)
- `REVERTED` (adapter returns failed/reverted status)

Sweep triggering rules:
- only when deposit status is `CONFIRMED`
- `auto_sweep=false` (recommended default)
- amount >= `sweep_threshold`
- skip when `account_id == treasury_account_id`

## Env
- `SCAN_DB_DSN` PostgreSQL DSN
- `WALLET_CORE_HTTP_ADDR` wallet-core base URL
- `CHAIN_GATEWAY_HTTP_ADDR` chain-gateway base URL
- `SCAN_API_TOKEN` wallet-core API token
- `SCAN_INTERVAL_SECONDS` poll interval (default: `5`)
- `SCAN_MIN_CONFIRMATIONS` global min confirmations (default: `1`)
- `SCAN_ACCOUNT_PAGE_SIZE` account scan page size (default: `50`)
- `SCAN_ACCOUNT_MAX_PAGES` account scan max pages per tick (default: `2`)
- `SCAN_WATCH_LIMIT` max watch addresses per model each tick (default: `500`)
- `SCAN_ETH_RPC_URL` Ethereum JSON-RPC URL (enables pure-RPC ETH scanning)
- `SCAN_ETH_LOOKBACK_BLOCKS` first-run lookback window (default: `300`)
- `SCAN_ETH_MAX_BLOCKS_PER_TICK` max ETH blocks scanned per tick (default: `80`)

## Storage
- `scan_watch_addresses`: watch targets (`model=account|utxo`)
- `scan_checkpoints`: account cursor checkpoint / last tx
- `scan_seen_events`: event state `(status,confirmations)` + dedup key `(tenant,account,model,chain,coin,network,address,tx_hash,event_index)`

## Notes
- Account-model parser now has chain parsers:
  - EVM (`tx[].froms/tos/values`)
  - TRON (same shape as EVM in legacy adapter)
  - SOL (`tx[].from/to/value`)
  - generic fallback for other chains
- ETH can run in pure-RPC scan mode (block scanning by `eth_getBlockByNumber`) when `SCAN_ETH_RPC_URL` is set.
- If account tx response does not include confirmations, scanner treats it as unknown and will not block credit on confirmations check.
