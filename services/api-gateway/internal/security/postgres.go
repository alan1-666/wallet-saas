package security

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strings"

	_ "github.com/lib/pq"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	p := &Postgres{db: db}
	if err := p.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return p, nil
}

func (p *Postgres) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *Postgres) ValidateToken(ctx context.Context, token string) (Scope, error) {
	var out Scope
	err := p.db.QueryRowContext(ctx, `
SELECT tenant_id, can_withdraw, can_deposit, can_sweep
FROM api_tokens
WHERE token = $1 AND status='ACTIVE'
`, token).Scan(&out.TenantID, &out.CanWithdraw, &out.CanDeposit, &out.CanSweep)
	if err != nil {
		return Scope{}, err
	}
	return out, nil
}

func (p *Postgres) CheckSignPermission(ctx context.Context, tenantID, keyID string) (bool, error) {
	var exists int
	err := p.db.QueryRowContext(ctx, `
SELECT 1
FROM tenant_keys
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

func (p *Postgres) CheckTenantChainPolicy(ctx context.Context, tenantID, chain, network, operation, amount string) error {
	tenantID = strings.TrimSpace(tenantID)
	chain = strings.ToLower(strings.TrimSpace(chain))
	network = strings.ToLower(strings.TrimSpace(network))
	op := strings.ToLower(strings.TrimSpace(operation))
	if tenantID == "" || chain == "" || network == "" {
		return fmt.Errorf("tenant_id/chain/network are required")
	}
	if op == "" {
		return fmt.Errorf("operation is required")
	}

	var (
		allowDeposit  bool
		allowWithdraw bool
		allowSweep    bool
		maxWithdraw   string
	)
	err := p.db.QueryRowContext(ctx, `
SELECT allow_deposit, allow_withdraw, allow_sweep, max_withdraw_amount
FROM tenant_chain_policies
WHERE status='ACTIVE'
  AND tenant_id IN ($1, '*')
  AND chain IN ($2, '*')
  AND network IN ($3, '*')
ORDER BY
  CASE WHEN tenant_id=$1 THEN 1 ELSE 0 END DESC,
  CASE WHEN chain=$2 THEN 1 ELSE 0 END DESC,
  CASE WHEN network=$3 THEN 1 ELSE 0 END DESC,
  priority DESC,
  id DESC
LIMIT 1
`, tenantID, chain, network).Scan(&allowDeposit, &allowWithdraw, &allowSweep, &maxWithdraw)
	if err == sql.ErrNoRows {
		return fmt.Errorf("tenant chain policy denied")
	}
	if err != nil {
		return err
	}

	switch op {
	case "deposit":
		if !allowDeposit {
			return fmt.Errorf("deposit not allowed on chain/network")
		}
	case "withdraw":
		if !allowWithdraw {
			return fmt.Errorf("withdraw not allowed on chain/network")
		}
		if strings.TrimSpace(amount) != "" && strings.TrimSpace(maxWithdraw) != "" && strings.TrimSpace(maxWithdraw) != "0" {
			amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
			if !ok {
				return fmt.Errorf("invalid withdraw amount")
			}
			maxAmt, ok := new(big.Int).SetString(strings.TrimSpace(maxWithdraw), 10)
			if !ok {
				return fmt.Errorf("invalid policy max_withdraw_amount")
			}
			if amt.Cmp(maxAmt) > 0 {
				return fmt.Errorf("withdraw amount exceeds tenant chain policy")
			}
		}
	case "sweep":
		if !allowSweep {
			return fmt.Errorf("sweep not allowed on chain/network")
		}
	default:
		return fmt.Errorf("unsupported operation")
	}
	return nil
}

func (p *Postgres) Audit(ctx context.Context, tenantID, action, requestID, detail string) error {
	_, err := p.db.ExecContext(ctx, `
INSERT INTO audit_logs (tenant_id, action, request_id, detail)
VALUES ($1,$2,$3,$4)
`, tenantID, action, requestID, detail)
	return err
}

func (p *Postgres) Reserve(ctx context.Context, tenantID, requestID, operation, requestHash string) (IdemResult, error) {
	_, err := p.db.ExecContext(ctx, `
INSERT INTO idem_requests (tenant_id, request_id, operation, request_hash, status)
VALUES ($1,$2,$3,$4,'PENDING')
ON CONFLICT (tenant_id, request_id, operation) DO NOTHING
`, tenantID, requestID, operation, requestHash)
	if err != nil {
		return IdemResult{}, err
	}

	var hash, status, response string
	err = p.db.QueryRowContext(ctx, `
SELECT request_hash, status, response
FROM idem_requests
WHERE tenant_id=$1 AND request_id=$2 AND operation=$3
`, tenantID, requestID, operation).Scan(&hash, &status, &response)
	if err != nil {
		return IdemResult{}, err
	}

	if hash != requestHash {
		return IdemResult{State: "CONFLICT"}, nil
	}
	if status == "SUCCESS" {
		return IdemResult{State: "REPLAY", Response: response}, nil
	}
	if status == "FAILED" {
		return IdemResult{State: "REJECTED", Response: response}, nil
	}
	return IdemResult{State: "NEW"}, nil
}

func (p *Postgres) Commit(ctx context.Context, tenantID, requestID, operation, response string) error {
	_, err := p.db.ExecContext(ctx, `
UPDATE idem_requests
SET status='SUCCESS', response=$1, updated_at=NOW()
WHERE tenant_id=$2 AND request_id=$3 AND operation=$4
`, response, tenantID, requestID, operation)
	return err
}

func (p *Postgres) Reject(ctx context.Context, tenantID, requestID, operation, reason string) error {
	_, err := p.db.ExecContext(ctx, `
UPDATE idem_requests
SET status='FAILED', response=$1, updated_at=NOW()
WHERE tenant_id=$2 AND request_id=$3 AND operation=$4
`, reason, tenantID, requestID, operation)
	return err
}

func (p *Postgres) ensureSchema(ctx context.Context) error {
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
		`CREATE TABLE IF NOT EXISTS idem_requests (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
request_id TEXT NOT NULL,
operation TEXT NOT NULL,
request_hash TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'PENDING',
response TEXT NOT NULL DEFAULT '',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, request_id, operation)
);`,
		`CREATE TABLE IF NOT EXISTS tenant_chain_policies (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
chain TEXT NOT NULL,
network TEXT NOT NULL,
allow_deposit BOOLEAN NOT NULL DEFAULT FALSE,
allow_withdraw BOOLEAN NOT NULL DEFAULT FALSE,
allow_sweep BOOLEAN NOT NULL DEFAULT FALSE,
max_withdraw_amount TEXT NOT NULL DEFAULT '0',
priority INT NOT NULL DEFAULT 0,
status TEXT NOT NULL DEFAULT 'ACTIVE',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, chain, network)
);`,
	}
	for _, stmt := range stmts {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("security schema init failed: %w", err)
		}
	}
	if _, err := p.db.ExecContext(ctx, `
INSERT INTO api_tokens (token, tenant_id, can_withdraw, can_deposit, can_sweep, status)
VALUES ('token_t1_full','t1',TRUE,TRUE,TRUE,'ACTIVE')
ON CONFLICT (token) DO NOTHING
`); err != nil {
		return fmt.Errorf("seed api token failed: %w", err)
	}
	if _, err := p.db.ExecContext(ctx, `
INSERT INTO tenant_chain_policies (tenant_id, chain, network, allow_deposit, allow_withdraw, allow_sweep, max_withdraw_amount, priority, status) VALUES
('t1','ethereum','sepolia',TRUE,TRUE,TRUE,'100000000000000000000',100,'ACTIVE'),
('t1','ethereum','mainnet',FALSE,FALSE,FALSE,'0',100,'ACTIVE'),
('*','*','*',TRUE,FALSE,FALSE,'0',0,'ACTIVE')
ON CONFLICT (tenant_id, chain, network) DO NOTHING
`); err != nil {
		return fmt.Errorf("seed tenant chain policy failed: %w", err)
	}
	return nil
}
