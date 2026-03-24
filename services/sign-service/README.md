# sign-service

Signing boundary service.

## Responsibility boundary
- owns HD master material and signing
- exposes public derivation material for address creation
- enforces internal caller auth, request rate limiting, and audit logging before signing
- legacy random key generation / loose private-key lookup are intentionally removed

## Runtime env
- `SIGN_GRPC_HOST` (default `0.0.0.0`)
- `SIGN_GRPC_PORT` (default `9091`)
- `SIGN_LEVELDB_PATH` (default `./data/sign-leveldb`)
- `SIGN_AUTH_TOKEN` shared internal auth token (default `dev-sign-token`)
- `SIGN_RATE_LIMIT_WINDOW_SECONDS` fixed rate-limit window in seconds (default `60`)
- `SIGN_RATE_LIMIT_MAX_REQUESTS` max derive/sign requests per token+operation per window (default `300`)
- `SIGN_CUSTODY_PROVIDER` custody backend selector (default `local-hsm`)
- `SIGN_CUSTODY_SCHEME` custody label returned in gRPC responses (default `local-hsm-slot`)
- `SIGN_HSM_BACKEND` lower-level HSM backend selector (default `software`)
- `SIGN_HSM_SLOT_PREFIX` logical HSM slot prefix for HD master material (default `master`)
- `SIGN_ALLOWED_TENANTS` optional comma-separated signer allowlist; when set, derive/sign is denied for tenants outside the list
- `SIGN_SOFTWARE_VAULT_PASSWORD` unlock password for the local encrypted vault when `SIGN_HSM_BACKEND=software`
- `SIGN_SOFTWARE_VAULT_PASSWORD_FILE` optional file path containing the local vault unlock password
- `SIGN_SOFTWARE_VAULT_AUTO_CREATE` whether missing tenant/sign-type slots may auto-generate new master seeds (default `false`)
- `SIGN_SOFTWARE_VAULT_BOOTSTRAP_FILE` optional JSON file used to provision tenant master seeds at startup
- `SIGN_CLOUDHSM_CLUSTER_ID` CloudHSM cluster id when `SIGN_HSM_BACKEND=cloudhsm`
- `SIGN_CLOUDHSM_REGION` AWS region for CloudHSM backend
- `SIGN_CLOUDHSM_USER` CloudHSM crypto user (CU) name
- `SIGN_CLOUDHSM_PIN` CloudHSM CU password; the runtime PKCS#11 login string is built as `user:password`
- `SIGN_CLOUDHSM_PKCS11_LIB` local PKCS#11 library path for CloudHSM client

## Current model
- `sign-service` now depends on a custody provider interface; the current implementation is `local-hsm`
- underneath the custody provider there is now a separate HSM backend interface; the current implementations are `software` and `cloudhsm`
- the `software` backend is now an encrypted local vault: each tenant/sign-type slot is stored encrypted at rest and must be unlocked at startup with a vault password
- local vault slots can be provisioned explicitly from a bootstrap file, and auto-creation can be disabled to force per-tenant seed management under ops control
- logical HSM slots are now scoped as `slot_prefix:tenant_id:sign_type`, so each tenant gets an isolated HD master root per signing scheme
- the `cloudhsm` backend now uses a PKCS#11 provider boundary; on `linux+cgo` it loads a real PKCS#11 module and persists tenant seeds as token-backed data objects, while local dev environments use a clear unsupported-platform stub or fake session provider in tests
- the `software` backend keeps the same contract and stores tenant-scoped slots locally, so it can be swapped for CloudHSM without changing the gRPC contract
- `DeriveKey` returns child public key material for the requested `key_id`
- ECDSA chains additionally return account-level public derivation material (`account xpub` equivalent: compressed public key + chain code), so callers can derive child public keys without asking the signer to materialize child private keys
- EdDSA chains remain direct-derive only because the current hardened derivation path does not support public-only child derivation
- signer responses return `custody_scheme`, so upstream services can tell they are talking to the signer boundary backed by the current custody implementation
- derive/sign requests must carry tenant metadata (`x-tenant-id`) so the signer can route to the correct tenant master slot

## Security TODO
- move auth token management to a stronger internal identity mechanism
- add richer policy rules such as destination whitelist / approval tiers

## Vault ops
- the binary now supports local vault seed tooling through `sign-service vault ...`
- `go run ./cmd vault import --tenant t1 --sign-type ecdsa --seed-hex <hex>` provisions a new tenant/sign-type slot
- `go run ./cmd vault export --tenant t1 --sign-type ecdsa` prints the decrypted seed hex for the resolved slot
- `go run ./cmd vault rotate --tenant t1 --sign-type ecdsa --seed-hex <hex>` replaces the stored seed for that slot
- `--slot-id` can be used on any vault subcommand to target an explicit slot instead of deriving `slot_prefix:tenant_id:sign_type`
- vault subcommands reuse the same backend selection, vault unlock password resolution, and bootstrap-file provisioning logic as the gRPC server

## How To Test
- local development does not require any AWS resource: keep `SIGN_HSM_BACKEND=software`
- local development with the compose file uses an encrypted software vault plus `SIGN_SOFTWARE_VAULT_AUTO_CREATE=true` for convenience; production should prefer explicit seed bootstrap and disabled auto-create
- if neither `SIGN_SOFTWARE_VAULT_PASSWORD` nor `SIGN_SOFTWARE_VAULT_PASSWORD_FILE` is set, startup will prompt for the local vault password on stdin
- local unit tests cover tenant-scoped slot isolation, the `cloudhsm` backend with a fake PKCS#11 session provider, and the unsupported-platform fallback
- startup now validates backend-specific env upfront, so `SIGN_HSM_BACKEND=cloudhsm` without the required CloudHSM variables will fail fast before the gRPC server starts
- `SIGN_HSM_BACKEND=cloudhsm` on non-`linux+cgo` environments will fail at runtime with a clear unsupported-platform error; this is expected for macOS development machines
- real CloudHSM integration testing will require:
  - an AWS account with a CloudHSM cluster
  - a VPC-reachable client host/container
  - a configured crypto user and pin
  - the CloudHSM PKCS#11 library installed on the runtime host
