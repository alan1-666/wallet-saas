# chain-gateway

Multi-chain aggregation gateway (reusing legacy chain RPC services).

## Upstream dependencies
- legacy account/utxo chain code is embedded in-process under `/legacy/*`

## Endpoints
- `GET /healthz`
- `POST /v1/chain/convert-address`
- `POST /v1/chain/support-chains`
- `POST /v1/chain/valid-address`
- `POST /v1/chain/fee`
- `POST /v1/chain/account`
- `POST /v1/chain/tx-by-hash`
- `POST /v1/chain/tx-by-address`
- `POST /v1/chain/unspent-outputs` (utxo chain only)
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
- `CHAIN_ACCOUNT_CONFIG_PATH` (default `/app/legacy/wallet-chain-account/config.yml`)
- `CHAIN_UTXO_CONFIG_PATH` (default `/app/legacy/wallet-chain-utxo/config.yml`)

## Internal gRPC
- Service: `wallet.chaingateway.ChainGatewayService`
- Methods:
  - `ConvertAddress`
  - `BuildUnsignedTx`
  - `SendTx`
