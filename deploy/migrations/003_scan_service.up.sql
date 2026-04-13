-- scan-account-service tables

CREATE TABLE IF NOT EXISTS scan_watch_addresses (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  model TEXT NOT NULL DEFAULT 'account',
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  address TEXT NOT NULL,
  contract_address TEXT NOT NULL DEFAULT '',
  min_confirmations BIGINT NOT NULL DEFAULT 1,
  unlock_confirmations BIGINT NOT NULL DEFAULT 1,
  treasury_account_id TEXT NOT NULL DEFAULT 'treasury-main',
  cold_account_id TEXT NOT NULL DEFAULT '',
  auto_sweep BOOLEAN NOT NULL DEFAULT FALSE,
  sweep_threshold TEXT NOT NULL DEFAULT '0',
  hot_balance_cap TEXT NOT NULL DEFAULT '0',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (model, chain, coin, network, address, tenant_id, account_id)
);

CREATE TABLE IF NOT EXISTS scan_checkpoints (
  tenant_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  model TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  address TEXT NOT NULL,
  cursor TEXT NOT NULL DEFAULT '',
  last_tx_hash TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, account_id, model, chain, coin, network, address)
);

CREATE TABLE IF NOT EXISTS scan_chain_checkpoints (
  model TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  cursor TEXT NOT NULL DEFAULT '',
  last_tx_hash TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (model, chain, coin, network)
);

CREATE TABLE IF NOT EXISTS scan_seen_events (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  model TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  address TEXT NOT NULL,
  tx_hash TEXT NOT NULL,
  event_index BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'PENDING',
  confirmations BIGINT NOT NULL DEFAULT 0,
  amount TEXT NOT NULL DEFAULT '0',
  from_address TEXT NOT NULL DEFAULT '',
  to_address TEXT NOT NULL DEFAULT '',
  not_found_count BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, account_id, model, chain, coin, network, address, tx_hash, event_index)
);

CREATE TABLE IF NOT EXISTS scan_event_outbox (
  id BIGSERIAL PRIMARY KEY,
  event_key TEXT NOT NULL UNIQUE,
  tenant_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'PENDING',
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 12,
  next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scan_block_hashes (
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  block_number BIGINT NOT NULL,
  block_hash TEXT NOT NULL,
  parent_hash TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (chain, network, block_number)
);

CREATE TABLE IF NOT EXISTS scan_outgoing_watch (
  kind TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  order_id TEXT NOT NULL,
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  tx_hash TEXT NOT NULL DEFAULT '',
  not_found_count BIGINT NOT NULL DEFAULT 0,
  first_not_found_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_not_found_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (kind, tenant_id, order_id, tx_hash)
);

CREATE TABLE IF NOT EXISTS withdraw_tx_attempts (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  order_id TEXT NOT NULL,
  tx_hash TEXT NOT NULL,
  unsigned_tx TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'BROADCASTED',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, tx_hash)
);

CREATE INDEX IF NOT EXISTS idx_scan_watch_addr ON scan_watch_addresses (model, chain, coin, network, address);
CREATE UNIQUE INDEX IF NOT EXISTS uq_scan_watch_addr_tenant ON scan_watch_addresses (model, chain, coin, network, address, tenant_id, account_id);
CREATE INDEX IF NOT EXISTS idx_scan_outbox_pending ON scan_event_outbox (status, next_retry_at, id);
CREATE INDEX IF NOT EXISTS idx_scan_seen_reorg ON scan_seen_events (status, updated_at, chain, network);
CREATE INDEX IF NOT EXISTS idx_scan_outgoing_watch_tx ON scan_outgoing_watch (chain, network, tx_hash, updated_at);
