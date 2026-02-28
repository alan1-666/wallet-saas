# Unified Core + Chain Plugin Refactor Plan

## Objective

Build a clear two-layer architecture:

1. Core layer (`wallet-core` + `scan-service`):
- only business orchestration
- no chain-specific branching

2. Chain layer (`chain-gateway`):
- model abstraction: `account` / `utxo`
- per-chain plugin implementation
- normalized outputs for core services

## Target Responsibilities

### A. Unified Core (business orchestration only)

- address registration
- scan scheduling
- deposit state machine (`PENDING` / `CONFIRMED` / `REVERTED`)
- ledger and journal writes
- idempotency and anti-replay
- risk checks
- sweep orchestration

### B. Chain Adapter Plugins

Each chain plugin implements a unified capability set:

- `ListIncomingTransfers`
- `GetTxFinality`
- `BuildUnsigned`
- `Broadcast`
- `GetBalance`
- `ConvertAddress`

Adding a new chain must only add/modify plugin code in `chain-gateway`.

## Current Gaps (to be fixed)

- `scan-service` has chain-specific parsing and ETH-specific fast path.
- `wallet-core` still infers model by chain in handler.
- `chain-gateway` routing is mostly account-vs-utxo split, not chain+network plugin registration.
- some conversion logic still has network hardcoded.

## Refactor Phases

### Phase 1: Chain plugin registry foundation

- Introduce chain plugin registration in `chain-gateway` by `chain + network` with fallback.
- Keep current endpoints compatible.
- Add normalized internal DTOs for incoming transfer/finality/balance.

### Phase 2: Standardized chain endpoints

- Add/enable normalized endpoints in `chain-gateway`:
  - `/v1/chain/list-incoming-transfers`
  - `/v1/chain/tx-finality`
  - `/v1/chain/balance`
- Use plugin normalization inside gateway instead of service-side ad-hoc parsing.

### Phase 3: Scan service migration

- `scan-service` consumes normalized endpoint(s) only.
- remove chain-specific parsing branches.
- remove ETH-only special scanning branch after finality API is stable.

### Phase 4: Wallet core decoupling

- remove `inferModel(chain)` in handler path.
- explicit model at registration or resolve from metadata table.
- keep business state machine and ledger logic unchanged.

### Phase 5: Metadata and guardrails

- add `chain_metadata` table/config:
  - `chain`, `network`, `model`, `native_asset`, `min_confirmations`, `enabled`
- enforce validation in registration/scanning paths.

## Acceptance Criteria

1. No chain-specific `if chain == ...` in `wallet-core` and `scan-service` main flow.
2. Deposit/withdraw/sweep flows still work end-to-end.
3. New chain integration requires plugin registration + plugin code only.
4. Existing API callers remain compatible during migration.

## Rollout Strategy

1. Keep compatibility endpoints during migration window.
2. Switch `scan-service` first, then prune old parsing paths.
3. Remove deprecated paths only after e2e verification on testnet.

## Risks and Mitigation

- Response shape differences across chains:
  - solved by per-plugin normalization and contract tests.
- Migration breakage risk:
  - solved by compatibility mode and phased rollout.
- Finality semantics mismatch:
  - solved by chain/network metadata and per-plugin finality policy.
