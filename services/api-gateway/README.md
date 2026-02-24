# api-gateway

External API ingress.

## Endpoints
- `GET /healthz`
- `POST /v1/withdraw` (proxy to wallet-core)
- `GET /v1/withdraw/status` (proxy to wallet-core)
- `POST /v1/deposit/notify` (proxy to wallet-core)
- `POST /v1/sweep/run` (proxy to wallet-core)
- `GET /v1/balance` (proxy to wallet-core)
- `POST /v1/address/create` (proxy to wallet-core)
- `POST /v1/account/upsert` (proxy to wallet-core)
- `GET /v1/account/get` (proxy to wallet-core)
- `GET /v1/account/list` (proxy to wallet-core)
- `GET /v1/account/addresses` (proxy to wallet-core)
- `GET /v1/account/assets` (proxy to wallet-core)

## Env
- `API_GATEWAY_ADDR` (default `:8080`)
- `WALLET_CORE_HTTP_ADDR` (default `http://127.0.0.1:8081`)
