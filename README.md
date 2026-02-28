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

## Engineering conventions
- Service package layout and directory rules: `docs/SERVICE_LAYOUT.md`
- Chain/network control plane schema: `docs/CONTROL_PLANE_SCHEMA.md`

## Quick start
```bash
cd wallet-saas-v2
go work sync
```

Each service is intentionally lightweight and can run independently for incremental migration.
