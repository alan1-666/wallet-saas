package controlplane

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

func (p *Postgres) ListActiveRPCEndpoints(ctx context.Context) ([]RPCEndpoint, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT id, chain, network, model, endpoint_url, weight, timeout_ms, priority, status
FROM rpc_endpoints
WHERE status = 'ACTIVE'
ORDER BY chain, network, model, priority DESC, weight DESC, id ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RPCEndpoint, 0, 32)
	for rows.Next() {
		var item RPCEndpoint
		if err := rows.Scan(&item.ID, &item.Chain, &item.Network, &item.Model, &item.URL, &item.Weight, &item.TimeoutMS, &item.Priority, &item.Status); err != nil {
			return nil, err
		}
		item.Chain = strings.ToLower(strings.TrimSpace(item.Chain))
		item.Network = strings.ToLower(strings.TrimSpace(item.Network))
		item.Model = strings.ToLower(strings.TrimSpace(item.Model))
		item.URL = strings.TrimSpace(item.URL)
		item.Status = strings.ToUpper(strings.TrimSpace(item.Status))
		if item.Chain == "" || item.Network == "" || item.Model == "" || item.URL == "" {
			continue
		}
		if item.Weight <= 0 {
			item.Weight = 1
		}
		if item.TimeoutMS <= 0 {
			item.TimeoutMS = 10000
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Postgres) ensureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS rpc_endpoints (
id BIGSERIAL PRIMARY KEY,
chain TEXT NOT NULL,
network TEXT NOT NULL,
model TEXT NOT NULL,
endpoint_url TEXT NOT NULL,
weight INT NOT NULL DEFAULT 100,
timeout_ms INT NOT NULL DEFAULT 10000,
priority INT NOT NULL DEFAULT 0,
status TEXT NOT NULL DEFAULT 'ACTIVE',
fail_count BIGINT NOT NULL DEFAULT 0,
success_count BIGINT NOT NULL DEFAULT 0,
last_error TEXT NOT NULL DEFAULT '',
last_heartbeat_at TIMESTAMPTZ NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (chain, network, model, endpoint_url)
)`,
		`CREATE INDEX IF NOT EXISTS idx_rpc_endpoints_lookup
ON rpc_endpoints (chain, network, model, status, priority, weight)`,
		`CREATE TABLE IF NOT EXISTS chain_policies (
id BIGSERIAL PRIMARY KEY,
chain TEXT NOT NULL,
network TEXT NOT NULL,
required_confirmations BIGINT NOT NULL DEFAULT 1,
safe_depth BIGINT NOT NULL DEFAULT 1,
reorg_window BIGINT NOT NULL DEFAULT 6,
fee_policy TEXT NOT NULL DEFAULT '{}',
enabled BOOLEAN NOT NULL DEFAULT TRUE,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (chain, network)
)`,
	}
	for _, stmt := range stmts {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("control-plane schema init failed: %w", err)
		}
	}
	return nil
}
