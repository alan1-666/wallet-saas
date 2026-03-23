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
- `SIGN_CLOUDHSM_CLUSTER_ID` CloudHSM cluster id when `SIGN_HSM_BACKEND=cloudhsm`
- `SIGN_CLOUDHSM_REGION` AWS region for CloudHSM backend
- `SIGN_CLOUDHSM_USER` CloudHSM crypto user
- `SIGN_CLOUDHSM_PIN` CloudHSM user pin/password
- `SIGN_CLOUDHSM_PKCS11_LIB` local PKCS#11 library path for CloudHSM client

## Current model
- `sign-service` now depends on a custody provider interface; the current implementation is `local-hsm`
- underneath the custody provider there is now a separate HSM backend interface; the current implementations are `software` and a `cloudhsm` placeholder backend
- the `cloudhsm` backend now also includes a PKCS#11 session/provider skeleton, so local tests can exercise session lifecycle and slot persistence without talking to a real cluster
- master material is loaded from a logical HSM slot per sign type; the current implementation uses a local software-backed slot store as the backend, so it can later be replaced by CloudHSM/PKCS#11 style backends without changing the gRPC contract
- `DeriveKey` returns child public key material for the requested `key_id`
- ECDSA chains additionally return account-level public derivation material (`account xpub` equivalent: compressed public key + chain code), so callers can derive child public keys without asking the signer to materialize child private keys
- EdDSA chains remain direct-derive only because the current hardened derivation path does not support public-only child derivation
- signer responses return `custody_scheme`, so upstream services can tell they are talking to the signer boundary backed by the current custody implementation

## Security TODO
- implement real CloudHSM / PKCS#11 session management and slot-backed seed handling
- move auth token management to a stronger internal identity mechanism
- add richer policy rules such as destination whitelist / approval tiers

## How To Test
- local development does not require any AWS resource: keep `SIGN_HSM_BACKEND=software`
- local unit tests already cover the `cloudhsm` placeholder backend and a fake PKCS#11 session provider
- startup now validates backend-specific env upfront, so `SIGN_HSM_BACKEND=cloudhsm` without the required CloudHSM variables will fail fast before the gRPC server starts
- real CloudHSM integration testing will require:
  - an AWS account with a CloudHSM cluster
  - a VPC-reachable client host/container
  - a configured crypto user and pin
  - the CloudHSM PKCS#11 library installed on the runtime host
