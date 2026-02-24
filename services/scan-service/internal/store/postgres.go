package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type Postgres struct {
	db *sql.DB
}

type WatchAddress struct {
	TenantID          string
	AccountID         string
	Model             string
	Chain             string
	Coin              string
	Network           string
	Address           string
	MinConfirmations  int64
	TreasuryAccountID string
	AutoSweep         bool
	SweepThreshold    string
}

func NewPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	p := &Postgres{db: db}
	if err := p.initSchema(context.Background()); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Postgres) Close() error {
	return p.db.Close()
}

func (p *Postgres) ListWatchAddresses(ctx context.Context, model string, limit int) ([]WatchAddress, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT tenant_id, account_id, model, chain, coin, network, address, min_confirmations, treasury_account_id, auto_sweep, sweep_threshold
FROM scan_watch_addresses
WHERE active = TRUE
  AND model = $1
ORDER BY id ASC
LIMIT $2
`, strings.ToLower(strings.TrimSpace(model)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]WatchAddress, 0, limit)
	for rows.Next() {
		var w WatchAddress
		if err := rows.Scan(&w.TenantID, &w.AccountID, &w.Model, &w.Chain, &w.Coin, &w.Network, &w.Address, &w.MinConfirmations, &w.TreasuryAccountID, &w.AutoSweep, &w.SweepThreshold); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Postgres) GetCheckpoint(ctx context.Context, w WatchAddress) (string, error) {
	var cursor string
	err := p.db.QueryRowContext(ctx, `
SELECT cursor
FROM scan_checkpoints
WHERE tenant_id=$1 AND account_id=$2 AND model=$3 AND chain=$4 AND coin=$5 AND network=$6 AND address=$7
`, w.TenantID, w.AccountID, w.Model, w.Chain, w.Coin, w.Network, w.Address).Scan(&cursor)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return cursor, nil
}

func (p *Postgres) UpsertCheckpoint(ctx context.Context, w WatchAddress, cursor, lastTxHash string) error {
	_, err := p.db.ExecContext(ctx, `
INSERT INTO scan_checkpoints (tenant_id, account_id, model, chain, coin, network, address, cursor, last_tx_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (tenant_id, account_id, model, chain, coin, network, address)
DO UPDATE SET cursor=EXCLUDED.cursor, last_tx_hash=EXCLUDED.last_tx_hash, updated_at=NOW()
`, w.TenantID, w.AccountID, w.Model, w.Chain, w.Coin, w.Network, w.Address, cursor, lastTxHash)
	return err
}

func (p *Postgres) UpsertSeenEvent(ctx context.Context, w WatchAddress, txHash string, eventIndex int64, status string, confirmations int64) (bool, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var oldStatus string
	var oldConfirm int64
	err = tx.QueryRowContext(ctx, `
SELECT status, confirmations
FROM scan_seen_events
WHERE tenant_id=$1 AND account_id=$2 AND model=$3 AND chain=$4 AND coin=$5 AND network=$6 AND address=$7 AND tx_hash=$8 AND event_index=$9
FOR UPDATE
`, w.TenantID, w.AccountID, w.Model, w.Chain, w.Coin, w.Network, w.Address, txHash, eventIndex).Scan(&oldStatus, &oldConfirm)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
INSERT INTO scan_seen_events (tenant_id, account_id, model, chain, coin, network, address, tx_hash, event_index, status, confirmations)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`, w.TenantID, w.AccountID, w.Model, w.Chain, w.Coin, w.Network, w.Address, txHash, eventIndex, status, confirmations)
		if err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}

	shouldNotify := false
	if strings.ToUpper(status) != strings.ToUpper(oldStatus) {
		shouldNotify = true
	}
	if confirmations > oldConfirm {
		shouldNotify = true
	}
	if !shouldNotify {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}
	_, err = tx.ExecContext(ctx, `
UPDATE scan_seen_events
SET status=$1, confirmations=$2
WHERE tenant_id=$3 AND account_id=$4 AND model=$5 AND chain=$6 AND coin=$7 AND network=$8 AND address=$9 AND tx_hash=$10 AND event_index=$11
`, status, confirmations, w.TenantID, w.AccountID, w.Model, w.Chain, w.Coin, w.Network, w.Address, txHash, eventIndex)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (p *Postgres) initSchema(ctx context.Context) error {
	createStmts := []string{
		`CREATE TABLE IF NOT EXISTS scan_watch_addresses (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  model TEXT NOT NULL DEFAULT 'account',
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL DEFAULT 'mainnet',
  address TEXT NOT NULL,
  min_confirmations BIGINT NOT NULL DEFAULT 1,
  treasury_account_id TEXT NOT NULL DEFAULT 'treasury-main',
  auto_sweep BOOLEAN NOT NULL DEFAULT FALSE,
  sweep_threshold TEXT NOT NULL DEFAULT '0',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (model, chain, coin, network, address, tenant_id, account_id)
)`,
		`CREATE TABLE IF NOT EXISTS scan_checkpoints (
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
)`,
		`CREATE TABLE IF NOT EXISTS scan_seen_events (
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
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, account_id, model, chain, coin, network, address, tx_hash, event_index)
)`,
	}

	for _, stmt := range createStmts {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema failed: %w", err)
		}
	}

	alterStmts := []string{
		`ALTER TABLE scan_watch_addresses ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT 'account'`,
		`ALTER TABLE scan_watch_addresses ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'mainnet'`,
		`ALTER TABLE scan_watch_addresses ALTER COLUMN auto_sweep SET DEFAULT FALSE`,
		`ALTER TABLE scan_seen_events ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'PENDING'`,
		`ALTER TABLE scan_seen_events ADD COLUMN IF NOT EXISTS confirmations BIGINT NOT NULL DEFAULT 0`,
	}
	for _, stmt := range alterStmts {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("alter schema failed: %w", err)
		}
	}
	_, _ = p.db.ExecContext(ctx, `
UPDATE scan_watch_addresses
SET model='account'
WHERE model = '' OR model IS NULL`)
	_, _ = p.db.ExecContext(ctx, `
UPDATE scan_watch_addresses
SET network='mainnet'
WHERE network = '' OR network IS NULL`)

	_, err := p.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_scan_watch_addr ON scan_watch_addresses (model, chain, coin, network, address)`)
	if err != nil {
		return fmt.Errorf("create index failed: %w", err)
	}
	_, err = p.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS uq_scan_watch_addr_tenant ON scan_watch_addresses (model, chain, coin, network, address, tenant_id, account_id)`)
	if err != nil {
		return fmt.Errorf("create unique index failed: %w", err)
	}
	return nil
}
