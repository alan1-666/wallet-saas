# api-gateway

External API ingress.  
This layer is the single public entry for:
- token authentication
- tenant isolation
- sign-key permission check (`/v1/withdraw`)
- request idempotency / anti-replay
- audit log write

## Endpoints
- `GET /healthz`
- `POST /v1/withdraw`
- `GET /v1/withdraw/status`
- `POST /v1/withdraw/onchain/notify`
- `POST /v1/deposit/notify`
- `POST /v1/sweep/run`
- `POST /v1/sweep/onchain/notify`
- `GET /v1/balance`
- `POST /v1/address/create`
- `POST /v1/account/upsert`
- `GET /v1/account/get`
- `GET /v1/account/list`
- `GET /v1/account/addresses`
- `GET /v1/account/assets`

## Env
- `API_GATEWAY_ADDR` (default `:8080`)
- `WALLET_CORE_HTTP_ADDR` (default `http://127.0.0.1:8081`)
- `API_GATEWAY_DB_DSN` (fallback to `WALLET_DB_DSN`)
- `API_GATEWAY_UPSTREAM_TIMEOUT_MS` (default `10000`)

## Idempotency operations
- `withdraw`
- `deposit_notify`
- `sweep_run`

Idempotency key source: `X-Request-ID` header (gateway auto-generates if absent).
