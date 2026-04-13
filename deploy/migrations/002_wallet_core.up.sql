-- wallet-core tables: addresses, ledger, auth, idempotency

CREATE TABLE IF NOT EXISTS wallet_addresses (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  address TEXT NOT NULL,
  address_type TEXT NOT NULL DEFAULT 'deposit',
  hd_path TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, account_id, chain, network, address)
);

CREATE TABLE IF NOT EXISTS wallet_balances (
  tenant_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  balance TEXT NOT NULL DEFAULT '0',
  frozen TEXT NOT NULL DEFAULT '0',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, account_id, chain, coin, network)
);

CREATE TABLE IF NOT EXISTS wallet_ledger (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  order_id TEXT NOT NULL DEFAULT '',
  tx_hash TEXT NOT NULL DEFAULT '',
  entry_type TEXT NOT NULL,
  amount TEXT NOT NULL DEFAULT '0',
  balance_before TEXT NOT NULL DEFAULT '0',
  balance_after TEXT NOT NULL DEFAULT '0',
  note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wallet_ledger_tenant ON wallet_ledger (tenant_id, account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS wallet_withdraw_orders (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  order_id TEXT NOT NULL UNIQUE,
  account_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  to_address TEXT NOT NULL,
  amount TEXT NOT NULL,
  fee TEXT NOT NULL DEFAULT '0',
  status TEXT NOT NULL DEFAULT 'QUEUED',
  tx_hash TEXT NOT NULL DEFAULT '',
  error_msg TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallet_sweep_orders (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  sweep_order_id TEXT NOT NULL UNIQUE,
  from_account_id TEXT NOT NULL,
  to_account_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  amount TEXT NOT NULL DEFAULT '0',
  status TEXT NOT NULL DEFAULT 'QUEUED',
  tx_hash TEXT NOT NULL DEFAULT '',
  error_msg TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallet_auth_tokens (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  token_prefix TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL DEFAULT 'admin',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallet_idem (
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
