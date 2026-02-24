package auth

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type PostgresAuth struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*PostgresAuth, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	a := &PostgresAuth{db: db}
	if err := a.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return a, nil
}

func (a *PostgresAuth) Close() error {
	if a == nil || a.db == nil {
		return nil
	}
	return a.db.Close()
}

func (a *PostgresAuth) ValidateToken(ctx context.Context, token string) (ports.AuthScope, error) {
	var out ports.AuthScope
	err := a.db.QueryRowContext(ctx, `
SELECT tenant_id, can_withdraw, can_deposit, can_sweep
FROM api_tokens
WHERE token = $1 AND status='ACTIVE'
`, token).Scan(&out.TenantID, &out.CanWithdraw, &out.CanDeposit, &out.CanSweep)
	if err != nil {
		return ports.AuthScope{}, err
	}
	return out, nil
}

func (a *PostgresAuth) CheckSignPermission(ctx context.Context, tenantID, keyID string) (bool, error) {
	var exists int
	err := a.db.QueryRowContext(ctx, `
SELECT 1 FROM tenant_keys
WHERE tenant_id=$1 AND key_id=$2 AND status='ACTIVE'
LIMIT 1
`, tenantID, keyID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (a *PostgresAuth) BindTenantKey(ctx context.Context, tenantID, keyID string) error {
	_, err := a.db.ExecContext(ctx, `
INSERT INTO tenant_keys (tenant_id, key_id, status)
VALUES ($1,$2,'ACTIVE')
ON CONFLICT (tenant_id, key_id)
DO UPDATE SET status='ACTIVE', updated_at=NOW()
`, tenantID, keyID)
	return err
}

func (a *PostgresAuth) Audit(ctx context.Context, tenantID, action, requestID, detail string) error {
	_, err := a.db.ExecContext(ctx, `
INSERT INTO audit_logs (tenant_id, action, request_id, detail)
VALUES ($1,$2,$3,$4)
`, tenantID, action, requestID, detail)
	return err
}

func (a *PostgresAuth) ensureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS api_tokens (
id BIGSERIAL PRIMARY KEY,
token TEXT NOT NULL UNIQUE,
tenant_id TEXT NOT NULL,
can_withdraw BOOLEAN NOT NULL DEFAULT FALSE,
can_deposit BOOLEAN NOT NULL DEFAULT FALSE,
can_sweep BOOLEAN NOT NULL DEFAULT FALSE,
status TEXT NOT NULL DEFAULT 'ACTIVE',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`,
		`CREATE TABLE IF NOT EXISTS tenant_keys (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
key_id TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'ACTIVE',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, key_id)
);`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
action TEXT NOT NULL,
request_id TEXT NOT NULL,
detail TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`,
	}
	for _, stmt := range stmts {
		if _, err := a.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("auth schema init failed: %w", err)
		}
	}
	return nil
}
