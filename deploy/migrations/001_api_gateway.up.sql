-- api-gateway tables: tokens, policies, idempotency, audit

CREATE TABLE IF NOT EXISTS api_tokens (
  id BIGSERIAL PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  token_prefix TEXT NOT NULL DEFAULT '',
  tenant_id TEXT NOT NULL,
  permissions TEXT NOT NULL DEFAULT 'full',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant_chain_policies (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  min_confirmations BIGINT NOT NULL DEFAULT 1,
  max_withdraw TEXT NOT NULL DEFAULT '0',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, chain, network)
);

CREATE TABLE IF NOT EXISTS idem_requests (
  tenant_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  request_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'PENDING',
  response TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, request_id, operation)
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  action TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL DEFAULT '',
  token_prefix TEXT NOT NULL DEFAULT '',
  status_code INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_created ON audit_logs (tenant_id, created_at DESC);
