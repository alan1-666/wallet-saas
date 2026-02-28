# Service Layout (Go Best Practice)

## Goals

- Keep each service package tree minimal and readable.
- Avoid template placeholder directories.
- Keep domain code under `internal/` with clear boundaries.

## Standard service skeleton

```text
services/<service-name>/
  cmd/main.go
  internal/
    bootstrap/    # app wiring and dependency graph
    config/       # env/config parsing
    transport/
      http/       # http routes + handlers
      grpc/       # grpc server + handlers
    ...           # domain-specific packages only
  go.mod
  README.md
```

## Domain package rules

- Add package folders only when there is real code.
- Use `internal/adapters/*` only for ports implementation.
- Use `internal/ports` only in core orchestration services.
- Generated protobuf code stays under `internal/pb`.

## Current cleaned structure

- `api-gateway`: `bootstrap/config/transport/http`
- `sign-service`: `bootstrap/config/crypto/keystore/pb/transport/grpc`
- `scan-service`: `bootstrap/config/client/store/worker/pb`
- `chain-gateway`: `bootstrap/config/ports/dispatcher/service/adapters/clients/normalize/pb/transport/http/transport/grpc`
- `wallet-core`: `bootstrap/config/ports/orchestrator/adapters/pb/transport/http`

## Notes

- Empty placeholder folders were removed across all services.
- Further refactor should prefer package rename/move only when it improves ownership boundaries, not to force symmetry.
