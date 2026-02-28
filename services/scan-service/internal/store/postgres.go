package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

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

type PendingWithdraw struct {
	TenantID      string
	OrderID       string
	Chain         string
	Network       string
	TxHash        string
	Confirmations int64
	RequiredConfs int64
}

type PendingSweep struct {
	TenantID      string
	SweepOrderID  string
	Chain         string
	Network       string
	TxHash        string
	Confirmations int64
	RequiredConfs int64
}

type SeenEventChange struct {
	Notify      bool
	Inserted    bool
	OldStatus   string
	NewStatus   string
	OldConfirms int64
	NewConfirms int64
}

type OutboxEvent struct {
	ID          int64
	EventKey    string
	TenantID    string
	Chain       string
	Network     string
	EventType   string
	Payload     string
	Attempt     int
	MaxAttempts int
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
	const queryWithRegistry = `
SELECT sw.tenant_id, sw.account_id, sw.model, sw.chain, sw.coin, sw.network, sw.address, sw.min_confirmations, sw.treasury_account_id, sw.auto_sweep, sw.sweep_threshold
FROM scan_watch_addresses sw
WHERE sw.active = TRUE
  AND sw.model = $1
  AND (
    sw.model <> 'account'
    OR EXISTS (
      SELECT 1
      FROM wallet_addresses wa
      WHERE wa.tenant_id = sw.tenant_id
        AND wa.account_id = sw.account_id
        AND LOWER(wa.chain) = LOWER(sw.chain)
        AND UPPER(wa.coin) = UPPER(sw.coin)
        AND LOWER(wa.network) = LOWER(sw.network)
        AND LOWER(wa.address) = LOWER(sw.address)
        AND UPPER(COALESCE(wa.status, '')) = 'ACTIVE'
    )
  )
ORDER BY sw.id ASC
LIMIT $2
`
	const queryFallback = `
SELECT tenant_id, account_id, model, chain, coin, network, address, min_confirmations, treasury_account_id, auto_sweep, sweep_threshold
FROM scan_watch_addresses
WHERE active = TRUE
  AND model = $1
ORDER BY id ASC
LIMIT $2
`
	rows, err := p.db.QueryContext(ctx, queryWithRegistry, strings.ToLower(strings.TrimSpace(model)), limit)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "wallet_addresses") {
		rows, err = p.db.QueryContext(ctx, queryFallback, strings.ToLower(strings.TrimSpace(model)), limit)
	}
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
WHERE NOT (
  EXCLUDED.cursor ~ '^[0-9]+$'
  AND scan_checkpoints.cursor ~ '^[0-9]+$'
)
OR EXCLUDED.cursor::NUMERIC >= scan_checkpoints.cursor::NUMERIC
`, w.TenantID, w.AccountID, w.Model, w.Chain, w.Coin, w.Network, w.Address, cursor, lastTxHash)
	return err
}

func (p *Postgres) GetChainCheckpoint(ctx context.Context, model, chain, coin, network string) (string, string, error) {
	var cursor string
	var lastTxHash string
	err := p.db.QueryRowContext(ctx, `
SELECT cursor, last_tx_hash
FROM scan_chain_checkpoints
WHERE model=$1 AND chain=$2 AND coin=$3 AND network=$4
`, model, chain, coin, network).Scan(&cursor, &lastTxHash)
	if err == nil {
		return cursor, lastTxHash, nil
	}
	if err != sql.ErrNoRows {
		return "", "", err
	}
	// Fallback to most recent address-level checkpoint for backward compatibility.
	err = p.db.QueryRowContext(ctx, `
SELECT cursor, last_tx_hash
FROM scan_checkpoints
WHERE model=$1 AND chain=$2 AND coin=$3 AND network=$4
ORDER BY updated_at DESC
LIMIT 1
`, model, chain, coin, network).Scan(&cursor, &lastTxHash)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return cursor, lastTxHash, nil
}

func (p *Postgres) UpsertChainCheckpoint(ctx context.Context, model, chain, coin, network, cursor, lastTxHash string) error {
	_, err := p.db.ExecContext(ctx, `
INSERT INTO scan_chain_checkpoints (model, chain, coin, network, cursor, last_tx_hash)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (model, chain, coin, network)
DO UPDATE SET cursor=EXCLUDED.cursor, last_tx_hash=EXCLUDED.last_tx_hash, updated_at=NOW()
WHERE NOT (
  EXCLUDED.cursor ~ '^[0-9]+$'
  AND scan_chain_checkpoints.cursor ~ '^[0-9]+$'
)
OR EXCLUDED.cursor::NUMERIC >= scan_chain_checkpoints.cursor::NUMERIC
`, model, chain, coin, network, cursor, lastTxHash)
	return err
}

func (p *Postgres) UpsertSeenEvent(ctx context.Context, w WatchAddress, txHash string, eventIndex int64, status string, confirmations int64) (SeenEventChange, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return SeenEventChange{}, err
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
			return SeenEventChange{}, err
		}
		if err := tx.Commit(); err != nil {
			return SeenEventChange{}, err
		}
		return SeenEventChange{
			Notify:      true,
			Inserted:    true,
			OldStatus:   "",
			NewStatus:   status,
			OldConfirms: 0,
			NewConfirms: confirmations,
		}, nil
	}
	if err != nil {
		return SeenEventChange{}, err
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
			return SeenEventChange{}, err
		}
		return SeenEventChange{
			Notify:      false,
			Inserted:    false,
			OldStatus:   oldStatus,
			NewStatus:   oldStatus,
			OldConfirms: oldConfirm,
			NewConfirms: oldConfirm,
		}, nil
	}
	_, err = tx.ExecContext(ctx, `
UPDATE scan_seen_events
SET status=$1, confirmations=$2
WHERE tenant_id=$3 AND account_id=$4 AND model=$5 AND chain=$6 AND coin=$7 AND network=$8 AND address=$9 AND tx_hash=$10 AND event_index=$11
`, status, confirmations, w.TenantID, w.AccountID, w.Model, w.Chain, w.Coin, w.Network, w.Address, txHash, eventIndex)
	if err != nil {
		return SeenEventChange{}, err
	}
	if err := tx.Commit(); err != nil {
		return SeenEventChange{}, err
	}
	return SeenEventChange{
		Notify:      true,
		Inserted:    false,
		OldStatus:   oldStatus,
		NewStatus:   status,
		OldConfirms: oldConfirm,
		NewConfirms: confirmations,
	}, nil
}

func (p *Postgres) UpsertOutboxEvent(ctx context.Context, ev OutboxEvent) error {
	if strings.TrimSpace(ev.EventKey) == "" || strings.TrimSpace(ev.EventType) == "" {
		return fmt.Errorf("event_key/event_type are required")
	}
	if strings.TrimSpace(ev.TenantID) == "" || strings.TrimSpace(ev.Chain) == "" || strings.TrimSpace(ev.Network) == "" {
		return fmt.Errorf("tenant_id/chain/network are required")
	}
	if strings.TrimSpace(ev.Payload) == "" {
		return fmt.Errorf("payload is required")
	}
	if ev.MaxAttempts <= 0 {
		ev.MaxAttempts = 12
	}
	_, err := p.db.ExecContext(ctx, `
INSERT INTO scan_event_outbox (event_key, tenant_id, chain, network, event_type, payload, status, attempt_count, max_attempts, next_retry_at)
VALUES ($1,$2,$3,$4,$5,CAST($6 AS JSONB),'PENDING',0,$7,NOW())
ON CONFLICT (event_key)
DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  chain = EXCLUDED.chain,
  network = EXCLUDED.network,
  event_type = EXCLUDED.event_type,
  payload = EXCLUDED.payload,
  status = 'PENDING',
  attempt_count = 0,
  max_attempts = EXCLUDED.max_attempts,
  next_retry_at = NOW(),
  last_error = '',
  updated_at = NOW()
`, ev.EventKey, ev.TenantID, ev.Chain, ev.Network, ev.EventType, ev.Payload, ev.MaxAttempts)
	return err
}

func (p *Postgres) ListPendingOutboxEvents(ctx context.Context, limit int) ([]OutboxEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := p.db.QueryContext(ctx, `
SELECT id, event_key, tenant_id, chain, network, event_type, payload, attempt_count, max_attempts
FROM scan_event_outbox
WHERE status='PENDING'
  AND next_retry_at <= NOW()
ORDER BY id ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OutboxEvent, 0, limit)
	for rows.Next() {
		var item OutboxEvent
		if err := rows.Scan(&item.ID, &item.EventKey, &item.TenantID, &item.Chain, &item.Network, &item.EventType, &item.Payload, &item.Attempt, &item.MaxAttempts); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (p *Postgres) MarkOutboxEventDone(ctx context.Context, id int64) error {
	_, err := p.db.ExecContext(ctx, `
UPDATE scan_event_outbox
SET status='DONE', updated_at=NOW()
WHERE id=$1
`, id)
	return err
}

func (p *Postgres) MarkOutboxEventRetry(ctx context.Context, id int64, currentAttempt, maxAttempts int, reason string) error {
	nextAttempt := currentAttempt + 1
	if maxAttempts <= 0 {
		maxAttempts = 12
	}
	status := "PENDING"
	if nextAttempt >= maxAttempts {
		status = "FAILED"
	}
	backoff := time.Second * time.Duration(1<<min(nextAttempt, 6))
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}
	_, err := p.db.ExecContext(ctx, `
UPDATE scan_event_outbox
SET status=$1,
    attempt_count=$2,
    last_error=$3,
    next_retry_at=NOW() + $4::interval,
    updated_at=NOW()
WHERE id=$5
`, status, nextAttempt, strings.TrimSpace(reason), fmt.Sprintf("%d seconds", int(backoff.Seconds())), id)
	return err
}

func (p *Postgres) ListPendingWithdraws(ctx context.Context, limit int) ([]PendingWithdraw, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT tenant_id, order_id, chain, network, tx_hash, confirmations, required_confirmations
FROM ledger_freezes
WHERE status='BROADCASTED'
  AND tx_hash <> ''
ORDER BY updated_at ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PendingWithdraw, 0, limit)
	for rows.Next() {
		var it PendingWithdraw
		if err := rows.Scan(&it.TenantID, &it.OrderID, &it.Chain, &it.Network, &it.TxHash, &it.Confirmations, &it.RequiredConfs); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (p *Postgres) ListPendingSweeps(ctx context.Context, limit int) ([]PendingSweep, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT tenant_id, sweep_order_id, chain, network, tx_hash, confirmations, required_confirmations
FROM sweep_orders
WHERE status='BROADCASTED'
  AND tx_hash <> ''
ORDER BY updated_at ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PendingSweep, 0, limit)
	for rows.Next() {
		var it PendingSweep
		if err := rows.Scan(&it.TenantID, &it.SweepOrderID, &it.Chain, &it.Network, &it.TxHash, &it.Confirmations, &it.RequiredConfs); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
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
  network TEXT NOT NULL,
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
		`CREATE TABLE IF NOT EXISTS scan_chain_checkpoints (
  model TEXT NOT NULL,
  chain TEXT NOT NULL,
  coin TEXT NOT NULL,
  network TEXT NOT NULL,
  cursor TEXT NOT NULL DEFAULT '',
  last_tx_hash TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (model, chain, coin, network)
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
		`CREATE TABLE IF NOT EXISTS scan_event_outbox (
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
)`,
	}

	for _, stmt := range createStmts {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema failed: %w", err)
		}
	}

	alterStmts := []string{
		`ALTER TABLE scan_watch_addresses ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT 'account'`,
		`ALTER TABLE scan_watch_addresses ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'unknown'`,
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
SET network='unknown'
WHERE network = '' OR network IS NULL`)
	_, _ = p.db.ExecContext(ctx, `ALTER TABLE scan_watch_addresses ALTER COLUMN network DROP DEFAULT`)

	_, err := p.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_scan_watch_addr ON scan_watch_addresses (model, chain, coin, network, address)`)
	if err != nil {
		return fmt.Errorf("create index failed: %w", err)
	}
	_, err = p.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS uq_scan_watch_addr_tenant ON scan_watch_addresses (model, chain, coin, network, address, tenant_id, account_id)`)
	if err != nil {
		return fmt.Errorf("create unique index failed: %w", err)
	}
	_, err = p.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_scan_outbox_pending ON scan_event_outbox (status, next_retry_at, id)`)
	if err != nil {
		return fmt.Errorf("create outbox index failed: %w", err)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
