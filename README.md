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

## Testnet integration runbook (6 chains)
See: `docs/TESTNET_6CHAINS_RUNBOOK.md`
- Chains: ethereum(sepolia), binance(testnet), polygon(amoy), arbitrum(sepolia), solana(devnet), tron(nile)
- Flows: HD address create, deposit credit, withdraw, sweep

## Production Docker deployment
Use the production compose file with multi-stage Docker builds:

```bash
cd deploy
docker compose -f docker-compose.prod.yml up -d --build
```

Log files are written to a unified fixed directory inside containers:
- root: `/var/log/wallet-saas`
- per service: `/var/log/wallet-saas/<service>/`
- filename: `<service>-YYYYMMDD.log`
- current symlink: `/var/log/wallet-saas/<service>/current.log`

Host path is fixed by default to `/data/wallet-saas/logs` (override with `WALLET_LOGS_DIR` in environment).
