# chain-gateway

Multi-chain aggregation gateway (plugin routing by `chain + network + model`).

## Runtime mode
- Read path (`ListIncomingTransfers`, `GetTxFinality`, `GetBalance`) for EVM/Solana chains is pure JSON-RPC.
- Endpoint routing comes from DB control plane table `rpc_endpoints`.

## Upstream dependencies
- no runtime dependency on legacy account/utxo dispatchers

## Plugin model
- model abstraction: `account` / `utxo`
- each chain binds to adapter plugins through router registration
- normalized capabilities exposed to upstream services:
  - `ConvertAddress`
  - `BuildUnsignedTx`
  - `SendTx`
  - `ListIncomingTransfers`
  - `GetTxFinality`
  - `GetBalance`

## Endpoints
- `GET /healthz`
- `POST /v1/chain/convert-address`
- `POST /v1/chain/list-incoming-transfers`
- `POST /v1/chain/tx-finality`
- `POST /v1/chain/balance`
- `POST /v1/chain/send-tx`
- `POST /v1/chain/build-unsigned`

## Build unsigned payloads
- Account model: `chain`, `network`, `base64_tx`
- UTXO model: `chain`, `network`, `fee`, `vin[]`, `vout[]`

UTXO response includes:
- `unsigned_tx` (base64 of `tx_data` bytes)
- `sign_hashes` (hex list)

## UTXO send modes
- Raw mode: provide `raw_tx`.
- Assemble mode: provide `unsigned_tx` (base64), `signatures[]` (hex/base64), `public_keys[]` (hex/base64).
  Gateway will call legacy `buildSignedTransaction` then `sendTx`.

## Env
- `CHAIN_GATEWAY_ADDR` (default `:8082`)
- `CHAIN_GATEWAY_GRPC_ADDR` (default `:9082`)
- `CHAIN_GATEWAY_DB_DSN` (fallback: `WALLET_DB_DSN`)
- `CHAIN_GATEWAY_ENDPOINT_REFRESH_SECONDS` (default `15`)
- `CHAIN_GATEWAY_ENDPOINT_PROBE_SECONDS` (default `20`)
- `CHAIN_GATEWAY_ENDPOINT_FAIL_THRESHOLD` (default `3`)
- `CHAIN_GATEWAY_ENDPOINT_OPEN_SECONDS` (default `30`)
- `CHAIN_GATEWAY_ACCOUNT_*_NETWORK` (optional per-chain network preference when loading account RPC from DB)
  - e.g. `CHAIN_GATEWAY_ACCOUNT_SOL_NETWORK=devnet`
  - e.g. `CHAIN_GATEWAY_ACCOUNT_TRON_NETWORK=nile`

## Control plane
- SQL schema and seed examples: `/docs/CONTROL_PLANE_SCHEMA.md`
- endpoint selection key: `chain + network + model`
- no implicit `mainnet` fallback in API/runtime request path

## Internal gRPC
- Service: `wallet.chaingateway.ChainGatewayService`
- Methods:
  - `ConvertAddress`
  - `BuildUnsignedTx`
  - `SendTx`
  - `ListIncomingTransfers`
  - `GetTxFinality`
  - `GetBalance`
