# Chain Gateway Refactor (v2)

## Goal
Remove runtime dependency on legacy process orchestration and keep all multi-chain routing logic in new `chain-gateway` codebase.

## Layered design
- `internal/ports`: chain abstraction, request/response core types.
- `internal/adapters/evm`: account-model adapter implementation.
- `internal/adapters/utxo`: utxo-model adapter implementation.
- `internal/dispatcher`: plugin router by `chain+network` (with wildcard fallback).
- `internal/service`: use-case orchestration (address/build/send/read).
- `internal/transport/http`: HTTP I/O.
- `internal/transport/grpc`: internal gRPC surface.

## Current migrated core flow
- `POST /v1/chain/convert-address`
- `POST /v1/chain/list-incoming-transfers`
- `POST /v1/chain/tx-finality`
- `POST /v1/chain/balance`
- `POST /v1/chain/build-unsigned`
- `POST /v1/chain/send-tx`

These routes call `service.ChainService` and use `chain+network` plugin routing.

## Newly completed in this batch
1. `chain+network` strict requirement in routing/transport (no implicit mainnet fallback).
2. DB control plane table `rpc_endpoints` with runtime refresh.
3. endpoint manager with timeout/failure tracking, circuit-open cooldown, weighted selection.
4. EVM + Solana read path (`list/finality/balance`) switched to pure JSON-RPC primary flow.
5. legacy dispatcher dependency removed from chain-gateway runtime.
6. legacy adapter chain implementations moved in-process to:
   - `services/chain-gateway/internal/migrated/accountlegacy`
   - `services/chain-gateway/internal/migrated/utxolegacy`
7. old source directories removed:
   - `legacy/wallet-chain-account`
   - `legacy/wallet-chain-utxo`

## Internal gRPC core flow
- `ConvertAddress`
- `BuildUnsignedTx`
- `SendTx`
- `ListIncomingTransfers`
- `GetTxFinality`
- `GetBalance`

## Strategy
- Keep external gateway surface minimal and stable.
- Route all chain logic through `service -> dispatcher -> adapters`.
- Add retry wrapper for transient upstream errors.
- Add new endpoints only when they fit domain use-cases.

## Next migration list
1. chain metadata driven routing/rules from config db (per tenant allowlist optional).
2. policy hooks (risk/tenant whitelist) before `SendTx`.
3. retry/circuit-breaker for upstream chain clients.
4. enrich UTXO in-process plugins (current implementation is skeleton/noop).
