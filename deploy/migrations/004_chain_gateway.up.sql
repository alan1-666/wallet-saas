-- chain-gateway tables: RPC endpoint registry

CREATE TABLE IF NOT EXISTS rpc_endpoints (
  id BIGSERIAL PRIMARY KEY,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  model TEXT NOT NULL DEFAULT 'account',
  endpoint_url TEXT NOT NULL,
  weight INT NOT NULL DEFAULT 100,
  timeout_ms INT NOT NULL DEFAULT 10000,
  priority INT NOT NULL DEFAULT 100,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rpc_endpoints_chain ON rpc_endpoints (chain, network, model, status);
