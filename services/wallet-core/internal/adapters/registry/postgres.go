package registry

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type PostgresRegistry struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*PostgresRegistry, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	r := &PostgresRegistry{db: db}
	if err := r.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *PostgresRegistry) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *PostgresRegistry) UpsertWatchAddress(ctx context.Context, in ports.WatchAddressInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.upsertAccountTx(ctx, tx, in.TenantID, in.AccountID, "ACTIVE", ""); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO wallet_addresses (
  tenant_id, account_id, model, chain, coin, network, address, public_key, sign_type, status
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ACTIVE')
ON CONFLICT (tenant_id, account_id, chain, coin, network, address)
DO UPDATE SET
  model=EXCLUDED.model,
  public_key=EXCLUDED.public_key,
  sign_type=EXCLUDED.sign_type,
  status='ACTIVE',
  updated_at=NOW()
`, in.TenantID, in.AccountID, in.Model, in.Chain, in.Coin, in.Network, in.Address, in.PublicKey, in.SignType)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO scan_watch_addresses (
  tenant_id, account_id, model, chain, coin, network, address,
  min_confirmations, treasury_account_id, auto_sweep, sweep_threshold, active
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,TRUE)
ON CONFLICT (model, chain, coin, network, address, tenant_id, account_id)
DO UPDATE SET
  min_confirmations=EXCLUDED.min_confirmations,
  treasury_account_id=EXCLUDED.treasury_account_id,
  auto_sweep=EXCLUDED.auto_sweep,
  sweep_threshold=EXCLUDED.sweep_threshold,
  active=TRUE,
  updated_at=NOW()
`, in.TenantID, in.AccountID, in.Model, in.Chain, in.Coin, in.Network, in.Address, in.MinConfirmations, in.TreasuryAccountID, in.AutoSweep, in.SweepThreshold)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresRegistry) UpsertAccount(ctx context.Context, in ports.WalletAccount) (ports.WalletAccount, error) {
	if strings.TrimSpace(in.TenantID) == "" || strings.TrimSpace(in.AccountID) == "" {
		return ports.WalletAccount{}, fmt.Errorf("tenant_id/account_id is required")
	}
	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if status == "" {
		status = "ACTIVE"
	}
	if status != "ACTIVE" && status != "DISABLED" {
		return ports.WalletAccount{}, fmt.Errorf("invalid status")
	}
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO wallet_accounts (tenant_id, account_id, account_tag, status, remark)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (tenant_id, account_id)
DO UPDATE SET account_tag=EXCLUDED.account_tag, status=EXCLUDED.status, remark=EXCLUDED.remark, updated_at=NOW()
`, in.TenantID, in.AccountID, in.AccountTag, status, in.Remark); err != nil {
		return ports.WalletAccount{}, err
	}
	return r.GetAccount(ctx, in.TenantID, in.AccountID)
}

func (r *PostgresRegistry) GetAccount(ctx context.Context, tenantID, accountID string) (ports.WalletAccount, error) {
	var out ports.WalletAccount
	err := r.db.QueryRowContext(ctx, `
SELECT tenant_id, account_id, account_tag, status, remark,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       TO_CHAR(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM wallet_accounts
WHERE tenant_id=$1 AND account_id=$2
`, tenantID, accountID).Scan(&out.TenantID, &out.AccountID, &out.AccountTag, &out.Status, &out.Remark, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r *PostgresRegistry) ListAccounts(ctx context.Context, tenantID string, limit, offset int) ([]ports.WalletAccount, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT tenant_id, account_id, account_tag, status, remark,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       TO_CHAR(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM wallet_accounts
WHERE tenant_id=$1
ORDER BY id DESC
LIMIT $2 OFFSET $3
`, tenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ports.WalletAccount, 0, limit)
	for rows.Next() {
		var item ports.WalletAccount
		if err := rows.Scan(&item.TenantID, &item.AccountID, &item.AccountTag, &item.Status, &item.Remark, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRegistry) ListAccountAddresses(ctx context.Context, tenantID, accountID string) ([]ports.WalletAddress, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT tenant_id, account_id, model, chain, coin, network, address, public_key, sign_type, status,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       TO_CHAR(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM wallet_addresses
WHERE tenant_id=$1 AND account_id=$2
ORDER BY id DESC
`, tenantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ports.WalletAddress, 0, 8)
	for rows.Next() {
		var item ports.WalletAddress
		if err := rows.Scan(&item.TenantID, &item.AccountID, &item.Model, &item.Chain, &item.Coin, &item.Network, &item.Address, &item.PublicKey, &item.SignType, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRegistry) upsertAccountTx(ctx context.Context, tx *sql.Tx, tenantID, accountID, status, remark string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO wallet_accounts (tenant_id, account_id, status, remark)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, account_id) DO NOTHING
`, tenantID, accountID, status, remark)
	return err
}

func (r *PostgresRegistry) ensureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS wallet_accounts (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
account_tag TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'ACTIVE',
remark TEXT NOT NULL DEFAULT '',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, account_id)
);`,
		`CREATE TABLE IF NOT EXISTS wallet_addresses (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
model TEXT NOT NULL DEFAULT 'account',
chain TEXT NOT NULL,
coin TEXT NOT NULL,
network TEXT NOT NULL DEFAULT 'mainnet',
address TEXT NOT NULL,
public_key TEXT NOT NULL,
sign_type TEXT NOT NULL DEFAULT 'ecdsa',
status TEXT NOT NULL DEFAULT 'ACTIVE',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, account_id, chain, coin, network, address)
);`,
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
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("registry schema init failed: %w", err)
		}
	}
	_, err := r.db.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS uq_scan_watch_addr_tenant
ON scan_watch_addresses (model, chain, coin, network, address, tenant_id, account_id)
`)
	if err != nil {
		return fmt.Errorf("registry schema index failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE scan_watch_addresses ALTER COLUMN auto_sweep SET DEFAULT FALSE`); err != nil {
		return fmt.Errorf("registry schema alter default failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
ALTER TABLE wallet_accounts ADD COLUMN IF NOT EXISTS account_tag TEXT NOT NULL DEFAULT ''
`); err != nil {
		return fmt.Errorf("registry schema alter account_tag failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
ALTER TABLE wallet_accounts ADD COLUMN IF NOT EXISTS remark TEXT NOT NULL DEFAULT ''
`); err != nil {
		return fmt.Errorf("registry schema alter remark failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO wallet_accounts (tenant_id, account_id, status, remark)
SELECT DISTINCT tenant_id, account_id, 'ACTIVE', ''
FROM wallet_addresses
ON CONFLICT (tenant_id, account_id) DO NOTHING
`); err != nil {
		return fmt.Errorf("registry backfill accounts from wallet_addresses failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO wallet_accounts (tenant_id, account_id, status, remark)
SELECT DISTINCT tenant_id, account_id, 'ACTIVE', ''
FROM scan_watch_addresses
ON CONFLICT (tenant_id, account_id) DO NOTHING
`); err != nil {
		return fmt.Errorf("registry backfill accounts from scan_watch_addresses failed: %w", err)
	}
	return nil
}
