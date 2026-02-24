# Chain Gateway Refactor (v2)

## Goal
Remove runtime dependency on legacy process orchestration and keep all multi-chain routing logic in new `chain-gateway` codebase.

## Layered design
- `internal/ports`: chain abstraction, request/response core types.
- `internal/adapters/evm`: account-model adapter implementation.
- `internal/adapters/utxo`: utxo-model adapter implementation.
- `internal/dispatcher`: route by chain type (UTXO vs account).
- `internal/service`: use-case orchestration (build/send/convert).
- `internal/handler`: HTTP I/O only.

## Current migrated core flow
- `POST /v1/chain/convert-address`
- `POST /v1/chain/support-chains`
- `POST /v1/chain/valid-address`
- `POST /v1/chain/fee`
- `POST /v1/chain/account`
- `POST /v1/chain/tx-by-hash`
- `POST /v1/chain/tx-by-address`
- `POST /v1/chain/unspent-outputs`
- `POST /v1/chain/build-unsigned`
- `POST /v1/chain/send-tx`

These routes now call `service.ChainService`, not direct grpc clients.

## Strategy
- Keep external gateway surface minimal and stable.
- Route all chain logic through `service -> dispatcher -> adapters`.
- Add new endpoints only when they fit domain use-cases.

## Next migration list
1. unify tx/unspent response schema (remove model-specific output differences).
2. policy hooks (risk/tenant whitelist) before `SendTx`.
3. retry/circuit-breaker for grpc clients.
4. phase out external account/utxo rpc dependency by in-process chain adapters.
