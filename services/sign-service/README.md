# sign-service

Signing boundary service (migrated batch-1).

## Migrated from legacy
- gRPC interface compatibility from `wallet-sign-go/protobuf/wallet.proto`
- methods: `getSupportSignWay`, `exportPublicKeyList`, `signTxMessage`
- local key store logic (LevelDB)
- ECDSA/EdDSA key generation and signing

## Runtime env
- `SIGN_GRPC_HOST` (default `0.0.0.0`)
- `SIGN_GRPC_PORT` (default `9091`)
- `SIGN_LEVELDB_PATH` (default `./data/sign-leveldb`)

## Security TODO (next batch)
- replace plaintext private key persistence with HSM/MPC-backed signing
- enforce consumer token authz/authn
- add rate limiting and audit logs
