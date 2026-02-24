# wallet-saas-v2

A rewrite scaffold for centralized wallet SaaS.

## Services
- `services/api-gateway`: external REST entry, auth/rate limit/idempotency gate.
- `services/wallet-core`: orchestration, ledger/risk/sign/chain ports.
- `services/sign-service`: signing boundary service (HSM/MPC ready).
- `services/chain-gateway`: unified chain adapter gateway.

## Shared
- `shared/proto`: proto contracts.
- `shared/pkg`: reusable utilities.

## Quick start
```bash
cd wallet-saas-v2
go work sync
```

Each service is intentionally lightweight and can run independently for incremental migration.
