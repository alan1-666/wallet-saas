# Migration Plan (Bootstrap)

## Reuse-first mapping
- Old `wallet-sign-go` -> new `services/sign-service`.
- Old `wallet-chain-account` + `wallet-chain-utxo` -> new `services/chain-gateway`.
- Old `multichain-sync-account/services` + `worker` -> new `services/wallet-core`.
- Old protobuf -> move/select into `shared/proto`.

## Principles
1. Keep service boundaries stable, move business logic later.
2. Replace panic paths with explicit errors during migration.
3. Keep signing isolation strict (no plaintext key persistence).
