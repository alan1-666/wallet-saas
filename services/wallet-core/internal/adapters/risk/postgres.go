package risk

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	_ "github.com/lib/pq"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type PostgresRisk struct {
	db        *sql.DB
	maxAmount int64
}

func NewPostgres(dsn string, maxAmount string) (*PostgresRisk, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	limit, err := strconv.ParseInt(maxAmount, 10, 64)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("invalid RISK_MAX_WITHDRAW_AMOUNT: %w", err)
	}

	r := &PostgresRisk{db: db, maxAmount: limit}
	if err := r.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *PostgresRisk) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *PostgresRisk) CheckWithdraw(ctx context.Context, tenantID, accountID, orderID, chain, coin, amount string) (string, error) {
	var existDecision string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT decision FROM risk_events WHERE tenant_id = $1 AND order_id = $2`,
		tenantID,
		orderID,
	).Scan(&existDecision)
	if err == nil {
		return existDecision, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	limit, err := r.resolveMaxAmount(ctx, tenantID, chain, coin)
	if err != nil {
		return "", err
	}
	accountLimit, hasAccountLimit, err := r.resolveAccountMaxAmount(ctx, tenantID, accountID, chain, coin)
	if err != nil {
		return "", err
	}
	if hasAccountLimit && accountLimit < limit {
		limit = accountLimit
	}

	amt, err := strconv.ParseInt(amount, 10, 64)
	if err != nil || amt <= 0 {
		decision := "REJECT_INVALID_AMOUNT"
		if ierr := r.insertDecision(ctx, tenantID, accountID, orderID, chain, coin, amount, strconv.FormatInt(limit, 10), decision); ierr != nil {
			return "", ierr
		}
		return decision, nil
	}

	decision := "ALLOW"
	if amt > limit {
		if hasAccountLimit {
			decision = "REJECT_ACCOUNT_LIMIT"
		} else {
			decision = "REJECT_LIMIT"
		}
	}

	if err := r.insertDecision(ctx, tenantID, accountID, orderID, chain, coin, amount, strconv.FormatInt(limit, 10), decision); err != nil {
		return "", err
	}
	return decision, nil
}

func (r *PostgresRisk) GetWithdrawDecision(ctx context.Context, tenantID, orderID string) (ports.RiskDecision, error) {
	var out ports.RiskDecision
	err := r.db.QueryRowContext(ctx, `
SELECT decision, amount, chain, coin, rule_limit
FROM risk_events
WHERE tenant_id=$1 AND order_id=$2
`, tenantID, orderID).Scan(&out.Decision, &out.Amount, &out.Chain, &out.Coin, &out.RuleLimit)
	if err != nil {
		return ports.RiskDecision{}, err
	}
	return out, nil
}

func (r *PostgresRisk) resolveMaxAmount(ctx context.Context, tenantID, chain, coin string) (int64, error) {
	var limitStr string
	err := r.db.QueryRowContext(ctx, `
SELECT max_amount
FROM risk_rules
WHERE status='ACTIVE'
  AND (tenant_id = $1 OR tenant_id = '*')
  AND (chain = $2 OR chain = '*')
  AND (coin = $3 OR coin = '*')
ORDER BY
  CASE WHEN tenant_id = $1 THEN 1 ELSE 0 END DESC,
  CASE WHEN chain = $2 THEN 1 ELSE 0 END DESC,
  CASE WHEN coin = $3 THEN 1 ELSE 0 END DESC,
  priority DESC,
  id DESC
LIMIT 1
`, tenantID, chain, coin).Scan(&limitStr)
	if err == sql.ErrNoRows {
		return r.maxAmount, nil
	}
	if err != nil {
		return 0, err
	}
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return limit, nil
}

func (r *PostgresRisk) resolveAccountMaxAmount(ctx context.Context, tenantID, accountID, chain, coin string) (int64, bool, error) {
	if accountID == "" {
		return 0, false, nil
	}
	var limitStr string
	err := r.db.QueryRowContext(ctx, `
SELECT max_amount
FROM account_risk_limits
WHERE status='ACTIVE'
  AND (tenant_id = $1 OR tenant_id = '*')
  AND (account_id = $2 OR account_id = '*')
  AND (chain = $3 OR chain = '*')
  AND (coin = $4 OR coin = '*')
ORDER BY
  CASE WHEN tenant_id = $1 THEN 1 ELSE 0 END DESC,
  CASE WHEN account_id = $2 THEN 1 ELSE 0 END DESC,
  CASE WHEN chain = $3 THEN 1 ELSE 0 END DESC,
  CASE WHEN coin = $4 THEN 1 ELSE 0 END DESC,
  priority DESC,
  id DESC
LIMIT 1
`, tenantID, accountID, chain, coin).Scan(&limitStr)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return limit, true, nil
}

func (r *PostgresRisk) insertDecision(ctx context.Context, tenantID, accountID, orderID, chain, coin, amount, ruleLimit, decision string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO risk_events (tenant_id, account_id, order_id, chain, coin, amount, rule_limit, decision)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (tenant_id, order_id) DO NOTHING
`, tenantID, accountID, orderID, chain, coin, amount, ruleLimit, decision)
	return err
}

func (r *PostgresRisk) ensureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS risk_events (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL DEFAULT '',
order_id TEXT NOT NULL,
chain TEXT NOT NULL DEFAULT '',
coin TEXT NOT NULL DEFAULT '',
amount TEXT NOT NULL,
rule_limit TEXT NOT NULL DEFAULT '',
decision TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, order_id)
);`)
	if err != nil {
		return fmt.Errorf("risk schema init failed: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS risk_rules (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
chain TEXT NOT NULL,
coin TEXT NOT NULL,
max_amount TEXT NOT NULL,
priority INT NOT NULL DEFAULT 0,
status TEXT NOT NULL DEFAULT 'ACTIVE',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, chain, coin)
);`)
	if err != nil {
		return fmt.Errorf("risk rules schema init failed: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS account_risk_limits (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
chain TEXT NOT NULL,
coin TEXT NOT NULL,
max_amount TEXT NOT NULL,
priority INT NOT NULL DEFAULT 0,
status TEXT NOT NULL DEFAULT 'ACTIVE',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, account_id, chain, coin)
);`)
	if err != nil {
		return fmt.Errorf("account risk rules schema init failed: %w", err)
	}
	alterStmts := []string{
		`ALTER TABLE risk_events ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE risk_events ADD COLUMN IF NOT EXISTS chain TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE risk_events ADD COLUMN IF NOT EXISTS coin TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE risk_events ADD COLUMN IF NOT EXISTS rule_limit TEXT NOT NULL DEFAULT '';`,
	}
	for _, stmt := range alterStmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("risk schema alter failed: %w", err)
		}
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO risk_rules (tenant_id, chain, coin, max_amount, priority, status)
VALUES ('*','*','*',$1,0,'ACTIVE')
ON CONFLICT (tenant_id, chain, coin) DO NOTHING
`, strconv.FormatInt(r.maxAmount, 10))
	if err != nil {
		return fmt.Errorf("insert default risk rule failed: %w", err)
	}
	return nil
}
