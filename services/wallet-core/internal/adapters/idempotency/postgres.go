package idempotency

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &PostgresStore{db: db}
	if err := s.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) Reserve(ctx context.Context, tenantID, requestID, operation, requestHash string) (ports.IdemResult, error) {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO idem_requests (tenant_id, request_id, operation, request_hash, status)
VALUES ($1,$2,$3,$4,'PENDING')
ON CONFLICT (tenant_id, request_id, operation) DO NOTHING
`, tenantID, requestID, operation, requestHash)
	if err != nil {
		return ports.IdemResult{}, err
	}

	var hash, status, response string
	err = s.db.QueryRowContext(ctx, `
SELECT request_hash, status, response
FROM idem_requests
WHERE tenant_id=$1 AND request_id=$2 AND operation=$3
`, tenantID, requestID, operation).Scan(&hash, &status, &response)
	if err != nil {
		return ports.IdemResult{}, err
	}

	if hash != requestHash {
		return ports.IdemResult{State: "CONFLICT", RequestID: requestID}, nil
	}
	if status == "SUCCESS" {
		return ports.IdemResult{State: "REPLAY", Response: response, RequestID: requestID}, nil
	}
	if status == "FAILED" {
		return ports.IdemResult{State: "REJECTED", Response: response, RequestID: requestID}, nil
	}
	return ports.IdemResult{State: "NEW", RequestID: requestID}, nil
}

func (s *PostgresStore) Commit(ctx context.Context, tenantID, requestID, operation, response string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE idem_requests
SET status='SUCCESS', response=$1, updated_at=NOW()
WHERE tenant_id=$2 AND request_id=$3 AND operation=$4
`, response, tenantID, requestID, operation)
	return err
}

func (s *PostgresStore) Reject(ctx context.Context, tenantID, requestID, operation, reason string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE idem_requests
SET status='FAILED', response=$1, updated_at=NOW()
WHERE tenant_id=$2 AND request_id=$3 AND operation=$4
`, reason, tenantID, requestID, operation)
	return err
}

func (s *PostgresStore) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS idem_requests (
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
);`)
	if err != nil {
		return fmt.Errorf("idempotency schema init failed: %w", err)
	}
	return nil
}
