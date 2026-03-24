package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type PostgresLedger struct {
	db *sql.DB
}

const withdrawJobLease = 30 * time.Second

func NewPostgres(dsn string) (*PostgresLedger, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	l := &PostgresLedger{db: db}
	if err := l.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return l, nil
}

func (l *PostgresLedger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

func (l *PostgresLedger) FreezeWithdraw(ctx context.Context, tenantID, accountID, orderID, chain, network, asset, amount string, requiredConfs int64) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := l.freezeWithdrawTx(ctx, tx, tenantID, accountID, orderID, chain, network, asset, amount, requiredConfs); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) QueueWithdraw(ctx context.Context, in ports.WithdrawQueueInput) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := l.freezeWithdrawTx(ctx, tx, in.TenantID, in.AccountID, in.OrderID, in.Tx.Chain, in.Tx.Network, in.Tx.Coin, in.Tx.Amount, in.RequiredConfs); err != nil {
		return err
	}

	vinJSON, err := json.Marshal(in.Tx.Vin)
	if err != nil {
		return err
	}
	voutJSON, err := json.Marshal(in.Tx.Vout)
	if err != nil {
		return err
	}
	signersJSON, err := json.Marshal(in.Signers)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO withdraw_jobs (
  tenant_id, account_id, order_id, chain, network, coin, from_address, to_address, amount,
  contract_address, amount_unit, token_decimals, base64_tx, fee, vin_json, vout_json,
  signers_json, sign_type, required_confirmations, status, next_retry_at, processing_started_at, replacement_count
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,
  $10,$11,$12,$13,$14,CAST($15 AS JSONB),CAST($16 AS JSONB),
  CAST($17 AS JSONB),$18,$19,'QUEUED',NOW(),NULL,0
)
ON CONFLICT (tenant_id, order_id)
DO UPDATE SET
  account_id=EXCLUDED.account_id,
  chain=EXCLUDED.chain,
  network=EXCLUDED.network,
  coin=EXCLUDED.coin,
  from_address=EXCLUDED.from_address,
  to_address=EXCLUDED.to_address,
  amount=EXCLUDED.amount,
  contract_address=EXCLUDED.contract_address,
  amount_unit=EXCLUDED.amount_unit,
  token_decimals=EXCLUDED.token_decimals,
  base64_tx=EXCLUDED.base64_tx,
  fee=EXCLUDED.fee,
  vin_json=EXCLUDED.vin_json,
  vout_json=EXCLUDED.vout_json,
  signers_json=EXCLUDED.signers_json,
  sign_type=EXCLUDED.sign_type,
  required_confirmations=EXCLUDED.required_confirmations,
  status=CASE
    WHEN withdraw_jobs.status IN ('QUEUED', 'PROCESSING', 'BROADCASTED', 'DONE')
      THEN withdraw_jobs.status
    ELSE 'QUEUED'
  END,
  next_retry_at=NOW(),
  processing_started_at=NULL,
  last_error='',
  replacement_count=CASE
    WHEN withdraw_jobs.status IN ('BROADCASTED', 'DONE')
      THEN withdraw_jobs.replacement_count
    ELSE 0
  END,
  updated_at=NOW()
`, in.TenantID, in.AccountID, in.OrderID, in.Tx.Chain, in.Tx.Network, in.Tx.Coin, in.Tx.From, in.Tx.To, in.Tx.Amount,
		in.Tx.ContractAddress, in.Tx.AmountUnit, int64(in.Tx.TokenDecimals), in.Tx.Base64Tx, in.Tx.Fee, string(vinJSON), string(voutJSON),
		string(signersJSON), in.SignType, maxInt64(in.RequiredConfs, 1))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) freezeWithdrawTx(ctx context.Context, tx *sql.Tx, tenantID, accountID, orderID, chain, network, asset, amount string, requiredConfs int64) error {
	if asset == "" || chain == "" || network == "" {
		return fmt.Errorf("chain/network/asset are required")
	}
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	var existingStatus string
	err := tx.QueryRowContext(ctx, `
SELECT status FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, tenantID, orderID).Scan(&existingStatus)
	if err == nil {
		if existingStatus == "FROZEN" || existingStatus == "BROADCASTED" || existingStatus == "CONFIRMED" {
			return nil
		}
		return fmt.Errorf("withdraw order already exists with status=%s", existingStatus)
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	current, err := l.getBalanceTx(ctx, tx, tenantID, accountID, asset)
	if err != nil {
		return err
	}
	withdrawable, err := computeWithdrawable(current.Available, current.WithdrawLocked)
	if err != nil {
		return err
	}
	if _, err := subDecimalString(withdrawable, amount); err != nil {
		return err
	}
	avail, err := subDecimalString(current.Available, amount)
	if err != nil {
		return err
	}
	frozen, err := addDecimalString(current.Frozen, amount)
	if err != nil {
		return err
	}

	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, avail, frozen, current.WithdrawLocked); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
	INSERT INTO ledger_freezes (tenant_id, account_id, order_id, chain, network, asset, amount, status, required_confirmations, confirmations)
	VALUES ($1,$2,$3,$4,$5,$6,$7,'FROZEN',$8,0)
	ON CONFLICT (tenant_id, order_id) DO NOTHING
	`, tenantID, accountID, orderID, chain, network, asset, amount, requiredConfs); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries (tenant_id, account_id, order_id, asset, entry_type, amount, status)
VALUES ($1,$2,$3,$4,'WITHDRAW_FREEZE',$5,'PENDING')
ON CONFLICT (tenant_id, order_id, entry_type) DO NOTHING
`, tenantID, accountID, orderID, asset, amount); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'WITHDRAW',$2,$3,$4,'FREEZE',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, tenantID, orderID, accountID, asset, amount, avail); err != nil {
		return err
	}

	return nil
}

func (l *PostgresLedger) ClaimQueuedWithdraws(ctx context.Context, limit int) ([]ports.WithdrawJob, error) {
	if limit <= 0 {
		limit = 10
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
WITH eligible AS (
  SELECT
    wj.id,
    wj.created_at,
    ROW_NUMBER() OVER (
      PARTITION BY wj.tenant_id, LOWER(wj.chain), LOWER(wj.network), LOWER(wj.from_address)
      ORDER BY wj.created_at ASC, wj.id ASC
    ) AS sender_rank
  FROM withdraw_jobs wj
  JOIN ledger_freezes lf
    ON lf.tenant_id = wj.tenant_id
   AND lf.order_id = wj.order_id
  WHERE lf.status='FROZEN'
    AND (
      (wj.status='QUEUED' AND wj.next_retry_at <= NOW())
      OR (wj.status='PROCESSING' AND wj.processing_started_at IS NOT NULL AND wj.processing_started_at <= NOW() - ($2 * INTERVAL '1 millisecond'))
    )
    AND NOT EXISTS (
      SELECT 1
      FROM withdraw_jobs active
      WHERE active.tenant_id = wj.tenant_id
        AND LOWER(active.chain) = LOWER(wj.chain)
        AND LOWER(active.network) = LOWER(wj.network)
        AND LOWER(active.from_address) = LOWER(wj.from_address)
        AND active.order_id <> wj.order_id
        AND active.status IN ('PROCESSING')
    )
),
picked AS (
  SELECT wj.id
  FROM withdraw_jobs wj
  JOIN eligible e ON e.id = wj.id
  WHERE e.sender_rank = 1
  ORDER BY wj.created_at ASC, wj.id ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE withdraw_jobs w
SET status='PROCESSING',
    attempt_count=attempt_count+1,
    processing_started_at=NOW(),
    next_retry_at=NOW(),
    updated_at=NOW()
FROM picked
WHERE w.id = picked.id
RETURNING
  w.id, w.tenant_id, w.account_id, w.order_id, w.tx_hash, w.required_confirmations, w.attempt_count, w.replacement_count,
  w.chain, w.network, w.coin, w.from_address, w.to_address, w.amount,
  w.contract_address, w.amount_unit, w.token_decimals, w.base64_tx, w.fee,
  w.vin_json, w.vout_json, w.signers_json, w.sign_type
`, limit, int64(withdrawJobLease/time.Millisecond))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ports.WithdrawJob, 0, limit)
	for rows.Next() {
		var (
			job               ports.WithdrawJob
			tokenDecimals     int64
			vinJSON, voutJSON string
			signersJSON       string
		)
		if err := rows.Scan(
			&job.ID,
			&job.TenantID,
			&job.AccountID,
			&job.OrderID,
			&job.TxHash,
			&job.RequiredConfs,
			&job.AttemptCount,
			&job.ReplaceCount,
			&job.Tx.Chain,
			&job.Tx.Network,
			&job.Tx.Coin,
			&job.Tx.From,
			&job.Tx.To,
			&job.Tx.Amount,
			&job.Tx.ContractAddress,
			&job.Tx.AmountUnit,
			&tokenDecimals,
			&job.Tx.Base64Tx,
			&job.Tx.Fee,
			&vinJSON,
			&voutJSON,
			&signersJSON,
			&job.SignType,
		); err != nil {
			return nil, err
		}
		job.Tx.TokenDecimals = uint32(tokenDecimals)
		if err := json.Unmarshal([]byte(vinJSON), &job.Tx.Vin); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(voutJSON), &job.Tx.Vout); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(signersJSON), &job.Signers); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (l *PostgresLedger) MarkQueuedWithdrawDone(ctx context.Context, tenantID, orderID, txHash, unsignedTx string) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='BROADCASTED', tx_hash=$1, base64_tx=$2, last_error='', processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$3 AND order_id=$4
`, txHash, strings.TrimSpace(unsignedTx), tenantID, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO withdraw_tx_attempts (tenant_id, order_id, tx_hash, unsigned_tx, status)
VALUES ($1,$2,$3,$4,'BROADCASTED')
ON CONFLICT (tenant_id, tx_hash)
DO UPDATE SET unsigned_tx=EXCLUDED.unsigned_tx, status='BROADCASTED', updated_at=NOW()
`, tenantID, orderID, txHash, strings.TrimSpace(unsignedTx)); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) RescheduleQueuedWithdraw(ctx context.Context, tenantID, orderID, reason string, delay time.Duration) error {
	if delay < 0 {
		delay = 0
	}
	_, err := l.db.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='QUEUED',
    last_error=$1,
    next_retry_at=NOW() + ($2 * INTERVAL '1 millisecond'),
    processing_started_at=NULL,
    updated_at=NOW()
WHERE tenant_id=$3 AND order_id=$4
`, reason, int64(delay/time.Millisecond), tenantID, orderID)
	return err
}

func (l *PostgresLedger) MarkQueuedWithdrawFailed(ctx context.Context, tenantID, orderID, reason string) error {
	_, err := l.db.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='FAILED', last_error=$1, processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, reason, tenantID, orderID)
	return err
}

func (l *PostgresLedger) ClaimStaleBroadcastedWithdraws(ctx context.Context, limit int, minAge time.Duration, maxReplacements int) ([]ports.WithdrawJob, error) {
	if limit <= 0 {
		limit = 10
	}
	if minAge <= 0 {
		minAge = time.Minute
	}
	if maxReplacements <= 0 {
		maxReplacements = 3
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
WITH eligible AS (
  SELECT
    wj.id,
    wj.created_at,
    ROW_NUMBER() OVER (
      PARTITION BY wj.tenant_id, LOWER(wj.chain), LOWER(wj.network), LOWER(wj.from_address)
      ORDER BY wj.updated_at ASC, wj.id ASC
    ) AS sender_rank
  FROM withdraw_jobs wj
  JOIN ledger_freezes lf
    ON lf.tenant_id = wj.tenant_id
   AND lf.order_id = wj.order_id
  WHERE lf.status='BROADCASTED'
    AND COALESCE(lf.confirmations, 0) = 0
    AND LOWER(wj.chain) IN ('ethereum','binance','polygon','arbitrum','optimism','linea','scroll','mantle','zksync')
    AND COALESCE(wj.base64_tx, '') <> ''
    AND COALESCE(wj.replacement_count, 0) < $2
    AND (
      (wj.status='BROADCASTED' AND wj.updated_at <= NOW() - ($3 * INTERVAL '1 millisecond'))
      OR (wj.status='ACCELERATING' AND wj.processing_started_at IS NOT NULL AND wj.processing_started_at <= NOW() - ($4 * INTERVAL '1 millisecond'))
    )
    AND NOT EXISTS (
      SELECT 1
      FROM withdraw_jobs active
      WHERE active.tenant_id = wj.tenant_id
        AND LOWER(active.chain) = LOWER(wj.chain)
        AND LOWER(active.network) = LOWER(wj.network)
        AND LOWER(active.from_address) = LOWER(wj.from_address)
        AND active.order_id <> wj.order_id
        AND active.status IN ('PROCESSING', 'ACCELERATING')
    )
),
picked AS (
  SELECT wj.id
  FROM withdraw_jobs wj
  JOIN eligible e ON e.id = wj.id
  WHERE e.sender_rank = 1
  ORDER BY wj.updated_at ASC, wj.id ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE withdraw_jobs w
SET status='ACCELERATING',
    processing_started_at=NOW(),
    updated_at=NOW()
FROM picked
WHERE w.id = picked.id
RETURNING
  w.id, w.tenant_id, w.account_id, w.order_id, w.tx_hash, w.required_confirmations, w.attempt_count, w.replacement_count,
  w.chain, w.network, w.coin, w.from_address, w.to_address, w.amount,
  w.contract_address, w.amount_unit, w.token_decimals, w.base64_tx, w.fee,
  w.vin_json, w.vout_json, w.signers_json, w.sign_type
`, limit, maxReplacements, int64(minAge/time.Millisecond), int64(withdrawJobLease/time.Millisecond))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ports.WithdrawJob, 0, limit)
	for rows.Next() {
		var (
			job               ports.WithdrawJob
			tokenDecimals     int64
			vinJSON, voutJSON string
			signersJSON       string
		)
		if err := rows.Scan(
			&job.ID,
			&job.TenantID,
			&job.AccountID,
			&job.OrderID,
			&job.TxHash,
			&job.RequiredConfs,
			&job.AttemptCount,
			&job.ReplaceCount,
			&job.Tx.Chain,
			&job.Tx.Network,
			&job.Tx.Coin,
			&job.Tx.From,
			&job.Tx.To,
			&job.Tx.Amount,
			&job.Tx.ContractAddress,
			&job.Tx.AmountUnit,
			&tokenDecimals,
			&job.Tx.Base64Tx,
			&job.Tx.Fee,
			&vinJSON,
			&voutJSON,
			&signersJSON,
			&job.SignType,
		); err != nil {
			return nil, err
		}
		job.Tx.TokenDecimals = uint32(tokenDecimals)
		if err := json.Unmarshal([]byte(vinJSON), &job.Tx.Vin); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(voutJSON), &job.Tx.Vout); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(signersJSON), &job.Signers); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (l *PostgresLedger) ReplaceBroadcastedWithdraw(ctx context.Context, tenantID, orderID, oldTxHash, newTxHash, unsignedTx string) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_freezes
SET tx_hash=$1, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3 AND status='BROADCASTED'
`, newTxHash, tenantID, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='BROADCASTED',
    tx_hash=$1,
    base64_tx=$2,
    replacement_count=replacement_count+1,
    last_error='',
    processing_started_at=NULL,
    updated_at=NOW()
WHERE tenant_id=$3 AND order_id=$4
`, newTxHash, strings.TrimSpace(unsignedTx), tenantID, orderID); err != nil {
		return err
	}
	if strings.TrimSpace(oldTxHash) != "" {
		if _, err := tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='REPLACED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3 AND status='BROADCASTED'
`, tenantID, orderID, oldTxHash); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO withdraw_tx_attempts (tenant_id, order_id, tx_hash, unsigned_tx, status)
VALUES ($1,$2,$3,$4,'BROADCASTED')
ON CONFLICT (tenant_id, tx_hash)
DO UPDATE SET unsigned_tx=EXCLUDED.unsigned_tx, status='BROADCASTED', updated_at=NOW()
`, tenantID, orderID, newTxHash, strings.TrimSpace(unsignedTx)); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) ResetBroadcastedWithdraw(ctx context.Context, tenantID, orderID, reason string) error {
	_, err := l.db.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='BROADCASTED', last_error=$1, processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, reason, tenantID, orderID)
	return err
}

func (l *PostgresLedger) ConfirmWithdraw(ctx context.Context, tenantID, accountID, orderID, txHash string) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var (
		asset  string
		amount string
		status string
	)
	err = tx.QueryRowContext(ctx, `
SELECT asset, amount, status
FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, tenantID, orderID).Scan(&asset, &amount, &status)
	if err != nil {
		return err
	}
	if status == "BROADCASTED" || status == "CONFIRMED" {
		return tx.Commit()
	}
	if status != "FROZEN" {
		return fmt.Errorf("withdraw status invalid for confirm: %s", status)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_freezes
SET status='BROADCASTED', tx_hash=$1, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, txHash, tenantID, orderID); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='BROADCASTED', tx_hash=$1, last_error='', processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, txHash, tenantID, orderID)
	return tx.Commit()
}

func (l *PostgresLedger) ConfirmWithdrawOnChain(ctx context.Context, tenantID, orderID, txHash string, confirmations, requiredConfs int64) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		accountID string
		asset     string
		amount    string
		status    string
		oldConfs  int64
		reqConfs  int64
	)
	err = tx.QueryRowContext(ctx, `
SELECT account_id, asset, amount, status, confirmations, required_confirmations
FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, tenantID, orderID).Scan(&accountID, &asset, &amount, &status, &oldConfs, &reqConfs)
	if err != nil {
		return err
	}
	if requiredConfs <= 0 {
		requiredConfs = reqConfs
	}
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	if confirmations < oldConfs {
		confirmations = oldConfs
	}
	if status == "CONFIRMED" {
		_, err := tx.ExecContext(ctx, `
UPDATE ledger_freezes
SET confirmations=GREATEST(confirmations,$1), required_confirmations=$2, updated_at=NOW()
WHERE tenant_id=$3 AND order_id=$4
`, confirmations, requiredConfs, tenantID, orderID)
		if err != nil {
			return err
		}
		_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='CONFIRMED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3
`, tenantID, orderID, txHash)
		return tx.Commit()
	}
	if status != "BROADCASTED" && status != "FROZEN" {
		return fmt.Errorf("withdraw status invalid for onchain confirm: %s", status)
	}
	if confirmations < requiredConfs {
		_, err := tx.ExecContext(ctx, `
UPDATE ledger_freezes
SET status='BROADCASTED', tx_hash=$1, confirmations=GREATEST(confirmations,$2), required_confirmations=$3, updated_at=NOW()
WHERE tenant_id=$4 AND order_id=$5
`, txHash, confirmations, requiredConfs, tenantID, orderID)
		if err != nil {
			return err
		}
		_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='BROADCASTED', tx_hash=$1, last_error='', processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, txHash, tenantID, orderID)
		_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='BROADCASTED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3
`, tenantID, orderID, txHash)
		return tx.Commit()
	}

	current, err := l.getBalanceTx(ctx, tx, tenantID, accountID, asset)
	if err != nil {
		return err
	}
	frozen, err := subDecimalString(current.Frozen, amount)
	if err != nil {
		return err
	}
	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, current.Available, frozen, current.WithdrawLocked); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_freezes
SET status='CONFIRMED', tx_hash=$1, confirmations=GREATEST(confirmations,$2), required_confirmations=$3, updated_at=NOW()
WHERE tenant_id=$4 AND order_id=$5
`, txHash, confirmations, requiredConfs, tenantID, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_entries
SET status='DONE', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND entry_type='WITHDRAW_FREEZE'
`, tenantID, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries (tenant_id, account_id, order_id, asset, entry_type, amount, status)
VALUES ($1,$2,$3,$4,'WITHDRAW_CONFIRM',$5,'DONE')
ON CONFLICT (tenant_id, order_id, entry_type) DO NOTHING
`, tenantID, accountID, orderID, asset, amount); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'WITHDRAW',$2,$3,$4,'DEBIT',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, tenantID, orderID, accountID, asset, amount, current.Available); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='DONE', tx_hash=$1, last_error='', processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, txHash, tenantID, orderID)
	_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='CONFIRMED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3
`, tenantID, orderID, txHash)
	return tx.Commit()
}

func (l *PostgresLedger) FailWithdrawOnChain(ctx context.Context, tenantID, orderID, txHash, reason string, confirmations int64) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var (
		accountID string
		asset     string
		amount    string
		status    string
		currentTx string
	)
	err = tx.QueryRowContext(ctx, `
SELECT account_id, asset, amount, status, tx_hash
FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, tenantID, orderID).Scan(&accountID, &asset, &amount, &status, &currentTx)
	if err != nil {
		return err
	}
	if status == "RELEASED" {
		_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='FAILED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3
`, tenantID, orderID, txHash)
		return tx.Commit()
	}
	if status == "CONFIRMED" {
		_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='FAILED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3
`, tenantID, orderID, txHash)
		return tx.Commit()
	}
	if strings.TrimSpace(txHash) != "" && strings.TrimSpace(currentTx) != "" && !strings.EqualFold(strings.TrimSpace(currentTx), strings.TrimSpace(txHash)) {
		_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='FAILED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3
`, tenantID, orderID, txHash)
		return tx.Commit()
	}
	current, err := l.getBalanceTx(ctx, tx, tenantID, accountID, asset)
	if err != nil {
		return err
	}
	avail, err := addDecimalString(current.Available, amount)
	if err != nil {
		return err
	}
	frozen, err := subDecimalString(current.Frozen, amount)
	if err != nil {
		return err
	}
	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, avail, frozen, current.WithdrawLocked); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_freezes
SET status='RELEASED', reason=$1, confirmations=GREATEST(confirmations,$2), updated_at=NOW()
WHERE tenant_id=$3 AND order_id=$4
`, reason, confirmations, tenantID, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_entries
SET status='CANCELLED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND entry_type='WITHDRAW_FREEZE'
`, tenantID, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries (tenant_id, account_id, order_id, asset, entry_type, amount, status)
VALUES ($1,$2,$3,$4,'WITHDRAW_RELEASE',$5,'DONE')
ON CONFLICT (tenant_id, order_id, entry_type) DO NOTHING
`, tenantID, accountID, orderID, asset, amount); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'WITHDRAW',$2,$3,$4,'UNFREEZE',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, tenantID, orderID, accountID, asset, amount, avail); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='FAILED', last_error=$1, processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, reason, tenantID, orderID)
	_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_tx_attempts
SET status='FAILED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND tx_hash=$3
`, tenantID, orderID, txHash)
	return tx.Commit()
}

func (l *PostgresLedger) ReleaseWithdraw(ctx context.Context, tenantID, accountID, orderID, reason string) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var (
		asset  string
		amount string
		status string
	)
	err = tx.QueryRowContext(ctx, `
SELECT asset, amount, status
FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, tenantID, orderID).Scan(&asset, &amount, &status)
	if err != nil {
		return err
	}
	if status == "RELEASED" {
		return tx.Commit()
	}
	if status != "FROZEN" && status != "BROADCASTED" {
		return fmt.Errorf("withdraw status invalid for release: %s", status)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_freezes
SET status='RELEASED', reason=$1, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, reason, tenantID, orderID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE ledger_entries
SET status='CANCELLED', updated_at=NOW()
WHERE tenant_id=$1 AND order_id=$2 AND entry_type='WITHDRAW_FREEZE'
`, tenantID, orderID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries (tenant_id, account_id, order_id, asset, entry_type, amount, status)
VALUES ($1,$2,$3,$4,'WITHDRAW_RELEASE',$5,'DONE')
ON CONFLICT (tenant_id, order_id, entry_type) DO NOTHING
`, tenantID, accountID, orderID, asset, amount); err != nil {
		return err
	}

	current, err := l.getBalanceTx(ctx, tx, tenantID, accountID, asset)
	if err != nil {
		return err
	}
	avail, err := addDecimalString(current.Available, amount)
	if err != nil {
		return err
	}
	frozen, err := subDecimalString(current.Frozen, amount)
	if err != nil {
		return err
	}
	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, avail, frozen, current.WithdrawLocked); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'WITHDRAW',$2,$3,$4,'UNFREEZE',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, tenantID, orderID, accountID, asset, amount, avail); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `
UPDATE withdraw_jobs
SET status='RELEASED', last_error=$1, processing_started_at=NULL, updated_at=NOW()
WHERE tenant_id=$2 AND order_id=$3
`, reason, tenantID, orderID)

	return tx.Commit()
}

func (l *PostgresLedger) GetWithdrawStatus(ctx context.Context, tenantID, orderID string) (ports.LedgerStatus, error) {
	var out ports.LedgerStatus
	err := l.db.QueryRowContext(ctx, `
SELECT
  lf.status,
  lf.tx_hash,
  lf.reason,
  lf.amount,
  COALESCE(wj.status, ''),
  COALESCE(wj.attempt_count, 0),
  COALESCE(wj.last_error, '')
FROM ledger_freezes lf
LEFT JOIN withdraw_jobs wj
  ON wj.tenant_id = lf.tenant_id
 AND wj.order_id = lf.order_id
WHERE lf.tenant_id=$1 AND lf.order_id=$2
`, tenantID, orderID).Scan(&out.Status, &out.TxHash, &out.Reason, &out.Amount, &out.QueueStatus, &out.AttemptCount, &out.LastError)
	if err != nil {
		return ports.LedgerStatus{}, err
	}
	if out.Status == "FROZEN" {
		switch strings.ToUpper(strings.TrimSpace(out.QueueStatus)) {
		case "QUEUED", "PROCESSING":
			out.Status = strings.ToUpper(strings.TrimSpace(out.QueueStatus))
		}
	}
	return out, nil
}

func (l *PostgresLedger) CreditDeposit(ctx context.Context, in ports.DepositCreditInput) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	requiredConfs := in.RequiredConfs
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	unlockConfs := in.UnlockConfs
	if unlockConfs > 0 && unlockConfs < requiredConfs {
		unlockConfs = requiredConfs
	}
	targetStatus := normalizeDepositLifecycleStatus(in.ScanStatus, in.Status, in.Confirmations, requiredConfs, unlockConfs)

	var (
		oldStatus string
		oldCredit bool
		oldUnlock bool
	)
	err = tx.QueryRowContext(ctx, `
SELECT status, credited, unlocked
FROM deposit_events
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, in.TenantID, in.OrderID).Scan(&oldStatus, &oldCredit, &oldUnlock)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
INSERT INTO deposit_events (tenant_id, account_id, order_id, chain, network, coin, amount, tx_hash, from_address, to_address, confirmations, required_confirmations, unlock_confirmations, status, credited, unlocked)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,FALSE,FALSE)
`, in.TenantID, in.AccountID, in.OrderID, in.Chain, in.Network, in.Coin, in.Amount, in.TxHash, in.FromAddress, in.ToAddress, in.Confirmations, requiredConfs, maxInt64(unlockConfs, requiredConfs), targetStatus)
		if err != nil {
			return err
		}
		oldStatus = ""
		oldCredit = false
		oldUnlock = false
	} else if err != nil {
		return err
	}

	effectiveStatus := resolveEffectiveDepositStatus(oldStatus, targetStatus)

	newCredited := oldCredit
	newUnlocked := oldUnlock
	if !oldCredit && depositStatusCredits(effectiveStatus) {
		bal, err := l.getBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin)
		if err != nil {
			return err
		}
		avail, err := addDecimalString(bal.Available, in.Amount)
		if err != nil {
			return err
		}
		locked, err := addDecimalString(bal.WithdrawLocked, in.Amount)
		if err != nil {
			return err
		}
		if err := l.upsertBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin, avail, bal.Frozen, locked); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'DEPOSIT',$2,$3,$4,'CREDIT',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, in.TenantID, in.OrderID, in.AccountID, in.Coin, in.Amount, avail); err != nil {
			return err
		}
		newCredited = true
	}

	if newCredited && !oldUnlock && effectiveStatus == "FINALIZED" {
		bal, err := l.getBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin)
		if err != nil {
			return err
		}
		locked, err := subDecimalString(bal.WithdrawLocked, in.Amount)
		if err != nil {
			return err
		}
		if err := l.upsertBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin, bal.Available, bal.Frozen, locked); err != nil {
			return err
		}
		withdrawable, err := computeWithdrawable(bal.Available, locked)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'DEPOSIT',$2,$3,$4,'UNLOCK',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, in.TenantID, in.OrderID, in.AccountID, in.Coin, in.Amount, withdrawable); err != nil {
			return err
		}
		newUnlocked = true
	}

	if oldCredit && effectiveStatus == "REVERTED" {
		bal, err := l.getBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin)
		if err != nil {
			return err
		}
		avail, err := subDecimalString(bal.Available, in.Amount)
		if err != nil {
			return err
		}
		locked := bal.WithdrawLocked
		if !oldUnlock {
			locked, err = subDecimalString(bal.WithdrawLocked, in.Amount)
			if err != nil {
				return err
			}
		}
		if err := l.upsertBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin, avail, bal.Frozen, locked); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'DEPOSIT_REVERT',$2,$3,$4,'DEBIT',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, in.TenantID, in.OrderID, in.AccountID, in.Coin, in.Amount, avail); err != nil {
			return err
		}
		newCredited = false
		newUnlocked = false
		if err := l.recordReorgAuditEvent(ctx, tx, in); err != nil {
			log.Printf("ledger audit event write failed tenant=%s account=%s order=%s tx=%s err=%v",
				in.TenantID, in.AccountID, in.OrderID, in.TxHash, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
UPDATE deposit_events
SET account_id=$1,
    chain=$2,
    network=$3,
    coin=$4,
    amount=$5,
    tx_hash=$6,
    from_address=$7,
    to_address=$8,
    confirmations=GREATEST(confirmations,$9),
    required_confirmations=$10,
    unlock_confirmations=$11,
    status=$12,
    credited=$13,
    unlocked=$14,
    updated_at=NOW()
WHERE tenant_id=$15 AND order_id=$16
`, in.AccountID, in.Chain, in.Network, in.Coin, in.Amount, in.TxHash, in.FromAddress, in.ToAddress, in.Confirmations, requiredConfs, maxInt64(unlockConfs, requiredConfs), effectiveStatus, newCredited, newUnlocked, in.TenantID, in.OrderID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) recordReorgAuditEvent(ctx context.Context, tx *sql.Tx, in ports.DepositCreditInput) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO ledger_audit_events (tenant_id, account_id, order_id, chain, network, asset, amount, event_type, detail)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (tenant_id, order_id, event_type)
DO UPDATE SET
  account_id=EXCLUDED.account_id,
  chain=EXCLUDED.chain,
  network=EXCLUDED.network,
  asset=EXCLUDED.asset,
  amount=EXCLUDED.amount,
  detail=EXCLUDED.detail,
  updated_at=NOW()
`, in.TenantID, in.AccountID, in.OrderID, in.Chain, in.Network, in.Coin, in.Amount, "DEPOSIT_REORG_REVERTED", strings.TrimSpace(in.TxHash))
	return err
}

func (l *PostgresLedger) StartSweep(ctx context.Context, in ports.SweepCollectInput) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	requiredConfs := in.RequiredConfs
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO sweep_orders (tenant_id, sweep_order_id, from_account_id, treasury_account_id, chain, network, asset, amount, tx_hash, confirmations, required_confirmations, status)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,0,$10,'BROADCASTED')
ON CONFLICT (tenant_id, sweep_order_id)
DO UPDATE SET tx_hash=EXCLUDED.tx_hash, chain=EXCLUDED.chain, network=EXCLUDED.network, required_confirmations=EXCLUDED.required_confirmations, status='BROADCASTED', updated_at=NOW()
`, in.TenantID, in.SweepOrderID, in.FromAccountID, in.TreasuryAccountID, in.Chain, in.Network, in.Asset, in.Amount, in.TxHash, requiredConfs); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) ConfirmSweepOnChain(ctx context.Context, in ports.SweepConfirmInput) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var (
		treasuryAccountID string
		asset             string
		amount            string
		status            string
		oldConfirms       int64
		reqConfs          int64
	)
	err = tx.QueryRowContext(ctx, `
SELECT treasury_account_id, asset, amount, status, confirmations, required_confirmations
FROM sweep_orders
WHERE tenant_id=$1 AND sweep_order_id=$2
FOR UPDATE
`, in.TenantID, in.SweepOrderID).Scan(&treasuryAccountID, &asset, &amount, &status, &oldConfirms, &reqConfs)
	if err != nil {
		return err
	}
	if in.RequiredConfs <= 0 {
		in.RequiredConfs = reqConfs
	}
	if in.RequiredConfs <= 0 {
		in.RequiredConfs = 1
	}
	if in.Confirmations < oldConfirms {
		in.Confirmations = oldConfirms
	}
	if status == "DONE" {
		_, err := tx.ExecContext(ctx, `
UPDATE sweep_orders
SET confirmations=GREATEST(confirmations,$1), required_confirmations=$2, updated_at=NOW()
WHERE tenant_id=$3 AND sweep_order_id=$4
`, in.Confirmations, in.RequiredConfs, in.TenantID, in.SweepOrderID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if in.Confirmations < in.RequiredConfs {
		_, err := tx.ExecContext(ctx, `
UPDATE sweep_orders
SET status='BROADCASTED', tx_hash=$1, confirmations=GREATEST(confirmations,$2), required_confirmations=$3, updated_at=NOW()
WHERE tenant_id=$4 AND sweep_order_id=$5
`, in.TxHash, in.Confirmations, in.RequiredConfs, in.TenantID, in.SweepOrderID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	treasuryBal, err := l.getVaultBalanceTx(ctx, tx, in.TenantID, treasuryAccountID, asset)
	if err != nil {
		return err
	}
	treasuryAvail, err := addDecimalString(treasuryBal.Available, amount)
	if err != nil {
		return err
	}
	if err := l.upsertVaultBalanceTx(ctx, tx, in.TenantID, treasuryAccountID, asset, treasuryAvail); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE sweep_orders
SET status='DONE', tx_hash=$1, confirmations=GREATEST(confirmations,$2), required_confirmations=$3, updated_at=NOW()
WHERE tenant_id=$4 AND sweep_order_id=$5
`, in.TxHash, in.Confirmations, in.RequiredConfs, in.TenantID, in.SweepOrderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO vault_journals (tenant_id, biz_type, biz_id, treasury_account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'SWEEP',$2,$3,$4,'CREDIT',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, treasury_account_id, entry_side) DO NOTHING
`, in.TenantID, in.SweepOrderID, treasuryAccountID, asset, amount, treasuryAvail); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) FailSweepOnChain(ctx context.Context, tenantID, sweepOrderID, reason string, confirmations int64) error {
	_, err := l.db.ExecContext(ctx, `
UPDATE sweep_orders
SET status='FAILED', reason=$1, confirmations=GREATEST(confirmations,$2), updated_at=NOW()
WHERE tenant_id=$3 AND sweep_order_id=$4 AND status <> 'DONE'
`, reason, confirmations, tenantID, sweepOrderID)
	return err
}

func (l *PostgresLedger) ReserveTreasuryTransfer(ctx context.Context, in ports.TreasuryTransferReserveInput) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	requiredConfs := in.RequiredConfirmations
	if requiredConfs <= 0 {
		requiredConfs = 1
	}

	var existingStatus string
	err = tx.QueryRowContext(ctx, `
SELECT status
FROM treasury_transfer_orders
WHERE tenant_id=$1 AND transfer_order_id=$2
FOR UPDATE
`, in.TenantID, in.TransferOrderID).Scan(&existingStatus)
	if err == nil {
		switch strings.ToUpper(strings.TrimSpace(existingStatus)) {
		case "RESERVED", "BROADCASTED", "DONE":
			return tx.Commit()
		default:
			return fmt.Errorf("treasury transfer already exists with status=%s", existingStatus)
		}
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	sourceBal, err := l.getVaultBalanceTx(ctx, tx, in.TenantID, in.FromAccountID, in.Asset)
	if err != nil {
		return err
	}
	nextSourceAvail, err := subDecimalString(sourceBal.Available, in.Amount)
	if err != nil {
		return err
	}
	if err := l.upsertVaultBalanceTx(ctx, tx, in.TenantID, in.FromAccountID, in.Asset, nextSourceAvail); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO treasury_transfer_orders (
  tenant_id, transfer_order_id, from_account_id, to_account_id, chain, network, asset, amount,
  required_confirmations, status, source_tier, destination_tier
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,'RESERVED',$10,$11
)
ON CONFLICT (tenant_id, transfer_order_id) DO NOTHING
`, in.TenantID, in.TransferOrderID, in.FromAccountID, in.ToAccountID, in.Chain, in.Network, in.Asset, in.Amount, requiredConfs, strings.TrimSpace(in.SourceTier), strings.TrimSpace(in.DestinationTier)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO vault_journals (tenant_id, biz_type, biz_id, treasury_account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'TREASURY_TRANSFER',$2,$3,$4,'DEBIT_PENDING',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, treasury_account_id, entry_side) DO NOTHING
`, in.TenantID, in.TransferOrderID, in.FromAccountID, in.Asset, in.Amount, nextSourceAvail); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) MarkTreasuryTransferBroadcasted(ctx context.Context, tenantID, transferOrderID, txHash string, requiredConfs int64) error {
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	res, err := l.db.ExecContext(ctx, `
UPDATE treasury_transfer_orders
SET status='BROADCASTED', tx_hash=$1, required_confirmations=$2, reason='', updated_at=NOW()
WHERE tenant_id=$3 AND transfer_order_id=$4 AND status IN ('RESERVED', 'BROADCASTED')
`, txHash, requiredConfs, tenantID, transferOrderID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("treasury transfer not reserved")
	}
	return nil
}

func (l *PostgresLedger) ConfirmTreasuryTransferOnChain(ctx context.Context, in ports.TreasuryTransferConfirmInput) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		toAccountID   string
		fromAccountID string
		asset         string
		amount        string
		status        string
		oldConfirms   int64
		reqConfs      int64
	)
	err = tx.QueryRowContext(ctx, `
SELECT to_account_id, from_account_id, asset, amount, status, confirmations, required_confirmations
FROM treasury_transfer_orders
WHERE tenant_id=$1 AND transfer_order_id=$2
FOR UPDATE
`, in.TenantID, in.TransferOrderID).Scan(&toAccountID, &fromAccountID, &asset, &amount, &status, &oldConfirms, &reqConfs)
	if err != nil {
		return err
	}
	if in.RequiredConfs <= 0 {
		in.RequiredConfs = reqConfs
	}
	if in.RequiredConfs <= 0 {
		in.RequiredConfs = 1
	}
	if in.Confirmations < oldConfirms {
		in.Confirmations = oldConfirms
	}
	if status == "DONE" {
		_, err := tx.ExecContext(ctx, `
UPDATE treasury_transfer_orders
SET confirmations=GREATEST(confirmations,$1), required_confirmations=$2, updated_at=NOW()
WHERE tenant_id=$3 AND transfer_order_id=$4
`, in.Confirmations, in.RequiredConfs, in.TenantID, in.TransferOrderID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if status != "BROADCASTED" && status != "RESERVED" {
		return fmt.Errorf("treasury transfer status invalid for onchain confirm: %s", status)
	}
	if in.Confirmations < in.RequiredConfs {
		_, err := tx.ExecContext(ctx, `
UPDATE treasury_transfer_orders
SET status='BROADCASTED', tx_hash=$1, confirmations=GREATEST(confirmations,$2), required_confirmations=$3, updated_at=NOW()
WHERE tenant_id=$4 AND transfer_order_id=$5
`, in.TxHash, in.Confirmations, in.RequiredConfs, in.TenantID, in.TransferOrderID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	destinationBal, err := l.getVaultBalanceTx(ctx, tx, in.TenantID, toAccountID, asset)
	if err != nil {
		return err
	}
	sourceBal, err := l.getVaultBalanceTx(ctx, tx, in.TenantID, fromAccountID, asset)
	if err != nil {
		return err
	}
	nextDestinationAvail, err := addDecimalString(destinationBal.Available, amount)
	if err != nil {
		return err
	}
	if err := l.upsertVaultBalanceTx(ctx, tx, in.TenantID, toAccountID, asset, nextDestinationAvail); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE treasury_transfer_orders
SET status='DONE', tx_hash=$1, confirmations=GREATEST(confirmations,$2), required_confirmations=$3, updated_at=NOW()
WHERE tenant_id=$4 AND transfer_order_id=$5
`, in.TxHash, in.Confirmations, in.RequiredConfs, in.TenantID, in.TransferOrderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO vault_journals (tenant_id, biz_type, biz_id, treasury_account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'TREASURY_TRANSFER',$2,$3,$4,'DEBIT',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, treasury_account_id, entry_side) DO NOTHING
`, in.TenantID, in.TransferOrderID, fromAccountID, asset, amount, sourceBal.Available); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO vault_journals (tenant_id, biz_type, biz_id, treasury_account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'TREASURY_TRANSFER',$2,$3,$4,'CREDIT',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, treasury_account_id, entry_side) DO NOTHING
`, in.TenantID, in.TransferOrderID, toAccountID, asset, amount, nextDestinationAvail); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) FailTreasuryTransferOnChain(ctx context.Context, tenantID, transferOrderID, reason string, confirmations int64) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		fromAccountID string
		asset         string
		amount        string
		status        string
	)
	err = tx.QueryRowContext(ctx, `
SELECT from_account_id, asset, amount, status
FROM treasury_transfer_orders
WHERE tenant_id=$1 AND transfer_order_id=$2
FOR UPDATE
`, tenantID, transferOrderID).Scan(&fromAccountID, &asset, &amount, &status)
	if err != nil {
		return err
	}
	switch status {
	case "FAILED":
		return tx.Commit()
	case "DONE":
		return tx.Commit()
	case "RESERVED", "BROADCASTED":
	default:
		return fmt.Errorf("treasury transfer status invalid for fail: %s", status)
	}
	sourceBal, err := l.getVaultBalanceTx(ctx, tx, tenantID, fromAccountID, asset)
	if err != nil {
		return err
	}
	nextSourceAvail, err := addDecimalString(sourceBal.Available, amount)
	if err != nil {
		return err
	}
	if err := l.upsertVaultBalanceTx(ctx, tx, tenantID, fromAccountID, asset, nextSourceAvail); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE treasury_transfer_orders
SET status='FAILED', reason=$1, confirmations=GREATEST(confirmations,$2), updated_at=NOW()
WHERE tenant_id=$3 AND transfer_order_id=$4
`, reason, confirmations, tenantID, transferOrderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO vault_journals (tenant_id, biz_type, biz_id, treasury_account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'TREASURY_TRANSFER',$2,$3,$4,'ROLLBACK',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, treasury_account_id, entry_side) DO NOTHING
`, tenantID, transferOrderID, fromAccountID, asset, amount, nextSourceAvail); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) GetTreasuryTransferStatus(ctx context.Context, tenantID, transferOrderID string) (ports.TreasuryTransferStatus, error) {
	var out ports.TreasuryTransferStatus
	err := l.db.QueryRowContext(ctx, `
SELECT status, tx_hash, reason, amount, from_account_id, to_account_id, source_tier, destination_tier, confirmations, required_confirmations
FROM treasury_transfer_orders
WHERE tenant_id=$1 AND transfer_order_id=$2
`, tenantID, transferOrderID).Scan(
		&out.Status,
		&out.TxHash,
		&out.Reason,
		&out.Amount,
		&out.FromAccountID,
		&out.ToAccountID,
		&out.SourceTier,
		&out.DestinationTier,
		&out.Confirmations,
		&out.RequiredConfs,
	)
	if err != nil {
		return ports.TreasuryTransferStatus{}, err
	}
	return out, nil
}

func (l *PostgresLedger) GetBalance(ctx context.Context, tenantID, accountID, asset string) (ports.BalanceSnapshot, error) {
	var out ports.BalanceSnapshot
	err := l.db.QueryRowContext(ctx, `
SELECT available, frozen, withdraw_locked
FROM ledger_balances
WHERE tenant_id=$1 AND account_id=$2 AND asset=$3
`, tenantID, accountID, asset).Scan(&out.Available, &out.Frozen, &out.WithdrawLocked)
	if err == sql.ErrNoRows {
		return ports.BalanceSnapshot{Available: "0", Frozen: "0", WithdrawLocked: "0", Withdrawable: "0"}, nil
	}
	if err != nil {
		return ports.BalanceSnapshot{}, err
	}
	out.Withdrawable, err = computeWithdrawable(out.Available, out.WithdrawLocked)
	if err != nil {
		return ports.BalanceSnapshot{}, err
	}
	return out, nil
}

func (l *PostgresLedger) ListAccountAssets(ctx context.Context, tenantID, accountID string) ([]ports.AccountAsset, error) {
	rows, err := l.db.QueryContext(ctx, `
SELECT COALESCE(lb.asset, vb.asset) AS asset,
       COALESCE(lb.available, '0') AS available,
       COALESCE(lb.frozen, '0') AS frozen,
       COALESCE(lb.withdraw_locked, '0') AS withdraw_locked,
       COALESCE(vb.available, '0') AS vault_available
FROM ledger_balances lb
FULL OUTER JOIN vault_balances vb
  ON lb.tenant_id = vb.tenant_id
 AND lb.account_id = vb.account_id
 AND lb.asset = vb.asset
WHERE COALESCE(lb.tenant_id, vb.tenant_id) = $1
  AND COALESCE(lb.account_id, vb.account_id) = $2
ORDER BY 1
`, tenantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ports.AccountAsset, 0, 8)
	for rows.Next() {
		var item ports.AccountAsset
		if err := rows.Scan(&item.Asset, &item.Available, &item.Frozen, &item.WithdrawLocked, &item.VaultAvailable); err != nil {
			return nil, err
		}
		item.Withdrawable, err = computeWithdrawable(item.Available, item.WithdrawLocked)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (l *PostgresLedger) getBalanceTx(ctx context.Context, tx *sql.Tx, tenantID, accountID, asset string) (ports.BalanceSnapshot, error) {
	var out ports.BalanceSnapshot
	err := tx.QueryRowContext(ctx, `
SELECT available, frozen, withdraw_locked
FROM ledger_balances
WHERE tenant_id=$1 AND account_id=$2 AND asset=$3
FOR UPDATE
`, tenantID, accountID, asset).Scan(&out.Available, &out.Frozen, &out.WithdrawLocked)
	if err == sql.ErrNoRows {
		return ports.BalanceSnapshot{Available: "0", Frozen: "0", WithdrawLocked: "0", Withdrawable: "0"}, nil
	}
	if err != nil {
		return ports.BalanceSnapshot{}, err
	}
	out.Withdrawable, err = computeWithdrawable(out.Available, out.WithdrawLocked)
	if err != nil {
		return ports.BalanceSnapshot{}, err
	}
	return out, nil
}

func (l *PostgresLedger) upsertBalanceTx(ctx context.Context, tx *sql.Tx, tenantID, accountID, asset, available, frozen, withdrawLocked string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO ledger_balances (tenant_id, account_id, asset, available, frozen, withdraw_locked)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (tenant_id, account_id, asset)
DO UPDATE SET available=EXCLUDED.available, frozen=EXCLUDED.frozen, withdraw_locked=EXCLUDED.withdraw_locked, updated_at=NOW()
`, tenantID, accountID, asset, available, frozen, withdrawLocked)
	return err
}

func (l *PostgresLedger) getVaultBalanceTx(ctx context.Context, tx *sql.Tx, tenantID, treasuryAccountID, asset string) (ports.BalanceSnapshot, error) {
	var out ports.BalanceSnapshot
	err := tx.QueryRowContext(ctx, `
SELECT available, '0'
FROM vault_balances
WHERE tenant_id=$1 AND account_id=$2 AND asset=$3
FOR UPDATE
`, tenantID, treasuryAccountID, asset).Scan(&out.Available, &out.Frozen)
	if err == sql.ErrNoRows {
		return ports.BalanceSnapshot{Available: "0", Frozen: "0"}, nil
	}
	if err != nil {
		return ports.BalanceSnapshot{}, err
	}
	return out, nil
}

func (l *PostgresLedger) upsertVaultBalanceTx(ctx context.Context, tx *sql.Tx, tenantID, treasuryAccountID, asset, available string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO vault_balances (tenant_id, account_id, asset, available)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, account_id, asset)
DO UPDATE SET available=EXCLUDED.available, updated_at=NOW()
`, tenantID, treasuryAccountID, asset, available)
	return err
}

func (l *PostgresLedger) ensureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ledger_freezes (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
	order_id TEXT NOT NULL,
	chain TEXT NOT NULL DEFAULT '',
	network TEXT NOT NULL,
	asset TEXT NOT NULL DEFAULT '',
	amount TEXT NOT NULL,
	status TEXT NOT NULL,
	tx_hash TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	confirmations BIGINT NOT NULL DEFAULT 0,
	required_confirmations BIGINT NOT NULL DEFAULT 1,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, order_id)
);`,
		`CREATE TABLE IF NOT EXISTS ledger_entries (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
order_id TEXT NOT NULL,
asset TEXT NOT NULL DEFAULT '',
entry_type TEXT NOT NULL,
amount TEXT NOT NULL,
status TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, order_id, entry_type)
);`,
		`CREATE TABLE IF NOT EXISTS ledger_balances (
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
asset TEXT NOT NULL,
available TEXT NOT NULL DEFAULT '0',
frozen TEXT NOT NULL DEFAULT '0',
withdraw_locked TEXT NOT NULL DEFAULT '0',
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
PRIMARY KEY (tenant_id, account_id, asset)
);`,
		`CREATE TABLE IF NOT EXISTS ledger_journals (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
biz_type TEXT NOT NULL,
biz_id TEXT NOT NULL,
account_id TEXT NOT NULL,
asset TEXT NOT NULL,
entry_side TEXT NOT NULL,
amount TEXT NOT NULL,
balance_after TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, biz_type, biz_id, account_id, entry_side)
);`,
		`CREATE TABLE IF NOT EXISTS withdraw_jobs (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
order_id TEXT NOT NULL,
chain TEXT NOT NULL,
network TEXT NOT NULL,
coin TEXT NOT NULL,
from_address TEXT NOT NULL DEFAULT '',
to_address TEXT NOT NULL DEFAULT '',
amount TEXT NOT NULL,
contract_address TEXT NOT NULL DEFAULT '',
amount_unit TEXT NOT NULL DEFAULT '',
token_decimals BIGINT NOT NULL DEFAULT 0,
base64_tx TEXT NOT NULL DEFAULT '',
fee TEXT NOT NULL DEFAULT '',
vin_json JSONB NOT NULL DEFAULT '[]',
vout_json JSONB NOT NULL DEFAULT '[]',
signers_json JSONB NOT NULL DEFAULT '[]',
sign_type TEXT NOT NULL DEFAULT '',
required_confirmations BIGINT NOT NULL DEFAULT 1,
status TEXT NOT NULL DEFAULT 'QUEUED',
tx_hash TEXT NOT NULL DEFAULT '',
last_error TEXT NOT NULL DEFAULT '',
attempt_count INT NOT NULL DEFAULT 0,
replacement_count INT NOT NULL DEFAULT 0,
next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
processing_started_at TIMESTAMPTZ NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, order_id)
);`,
		`CREATE TABLE IF NOT EXISTS withdraw_tx_attempts (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
order_id TEXT NOT NULL,
tx_hash TEXT NOT NULL,
unsigned_tx TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'BROADCASTED',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, tx_hash)
);`,
		`CREATE TABLE IF NOT EXISTS deposit_events (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
order_id TEXT NOT NULL,
chain TEXT NOT NULL,
network TEXT NOT NULL,
coin TEXT NOT NULL,
amount TEXT NOT NULL,
tx_hash TEXT NOT NULL,
from_address TEXT NOT NULL DEFAULT '',
to_address TEXT NOT NULL DEFAULT '',
confirmations BIGINT NOT NULL DEFAULT 0,
required_confirmations BIGINT NOT NULL DEFAULT 1,
unlock_confirmations BIGINT NOT NULL DEFAULT 1,
status TEXT NOT NULL,
credited BOOLEAN NOT NULL DEFAULT FALSE,
unlocked BOOLEAN NOT NULL DEFAULT FALSE,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, order_id)
);`,
		`CREATE TABLE IF NOT EXISTS ledger_audit_events (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
order_id TEXT NOT NULL,
chain TEXT NOT NULL DEFAULT '',
network TEXT NOT NULL DEFAULT '',
asset TEXT NOT NULL DEFAULT '',
amount TEXT NOT NULL DEFAULT '0',
event_type TEXT NOT NULL,
detail TEXT NOT NULL DEFAULT '',
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, order_id, event_type)
);`,
		`CREATE TABLE IF NOT EXISTS sweep_orders (
id BIGSERIAL PRIMARY KEY,
		tenant_id TEXT NOT NULL,
	sweep_order_id TEXT NOT NULL,
	from_account_id TEXT NOT NULL,
	treasury_account_id TEXT NOT NULL,
	chain TEXT NOT NULL DEFAULT '',
	network TEXT NOT NULL,
	asset TEXT NOT NULL,
	amount TEXT NOT NULL,
	tx_hash TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	confirmations BIGINT NOT NULL DEFAULT 0,
	required_confirmations BIGINT NOT NULL DEFAULT 1,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (tenant_id, sweep_order_id)
);`,
		`CREATE TABLE IF NOT EXISTS treasury_transfer_orders (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
transfer_order_id TEXT NOT NULL,
from_account_id TEXT NOT NULL,
to_account_id TEXT NOT NULL,
chain TEXT NOT NULL DEFAULT '',
network TEXT NOT NULL,
asset TEXT NOT NULL,
amount TEXT NOT NULL,
tx_hash TEXT NOT NULL DEFAULT '',
reason TEXT NOT NULL DEFAULT '',
confirmations BIGINT NOT NULL DEFAULT 0,
required_confirmations BIGINT NOT NULL DEFAULT 1,
source_tier TEXT NOT NULL DEFAULT 'HOT',
destination_tier TEXT NOT NULL DEFAULT 'COLD',
status TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, transfer_order_id)
);`,
		`CREATE TABLE IF NOT EXISTS vault_balances (
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
asset TEXT NOT NULL,
available TEXT NOT NULL DEFAULT '0',
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
PRIMARY KEY (tenant_id, account_id, asset)
);`,
		`CREATE TABLE IF NOT EXISTS vault_journals (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
biz_type TEXT NOT NULL,
biz_id TEXT NOT NULL,
treasury_account_id TEXT NOT NULL,
asset TEXT NOT NULL,
entry_side TEXT NOT NULL,
amount TEXT NOT NULL,
balance_after TEXT NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, biz_type, biz_id, treasury_account_id, entry_side)
);`,
	}
	for _, stmt := range stmts {
		if _, err := l.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ledger schema init failed: %w", err)
		}
	}
	alterStmts := []string{
		`ALTER TABLE ledger_freezes ADD COLUMN IF NOT EXISTS tx_hash TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ledger_freezes ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ledger_freezes ADD COLUMN IF NOT EXISTS asset TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ledger_freezes ADD COLUMN IF NOT EXISTS chain TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ledger_freezes ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'unknown';`,
		`ALTER TABLE ledger_freezes ADD COLUMN IF NOT EXISTS confirmations BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE ledger_freezes ADD COLUMN IF NOT EXISTS required_confirmations BIGINT NOT NULL DEFAULT 1;`,
		`ALTER TABLE ledger_entries ADD COLUMN IF NOT EXISTS asset TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE ledger_balances ADD COLUMN IF NOT EXISTS withdraw_locked TEXT NOT NULL DEFAULT '0';`,
		`ALTER TABLE withdraw_jobs ADD COLUMN IF NOT EXISTS tx_hash TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE withdraw_jobs ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE withdraw_jobs ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0;`,
		`ALTER TABLE withdraw_jobs ADD COLUMN IF NOT EXISTS replacement_count INT NOT NULL DEFAULT 0;`,
		`ALTER TABLE withdraw_jobs ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`ALTER TABLE withdraw_jobs ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ NULL;`,
		`ALTER TABLE withdraw_jobs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS required_confirmations BIGINT NOT NULL DEFAULT 1;`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS unlock_confirmations BIGINT NOT NULL DEFAULT 1;`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS credited BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS unlocked BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'unknown';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS chain TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'unknown';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS tx_hash TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS confirmations BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS required_confirmations BIGINT NOT NULL DEFAULT 1;`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS chain TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'unknown';`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS tx_hash TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS confirmations BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS required_confirmations BIGINT NOT NULL DEFAULT 1;`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS source_tier TEXT NOT NULL DEFAULT 'HOT';`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS destination_tier TEXT NOT NULL DEFAULT 'COLD';`,
		`ALTER TABLE treasury_transfer_orders ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
	}
	for _, stmt := range alterStmts {
		if _, err := l.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ledger schema alter failed: %w", err)
		}
	}
	_, _ = l.db.ExecContext(ctx, `UPDATE ledger_freezes SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `UPDATE deposit_events SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `UPDATE sweep_orders SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `UPDATE treasury_transfer_orders SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `UPDATE withdraw_jobs SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `UPDATE withdraw_jobs SET next_retry_at=NOW() WHERE next_retry_at IS NULL`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE ledger_freezes ALTER COLUMN network DROP DEFAULT`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE deposit_events ALTER COLUMN network DROP DEFAULT`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE sweep_orders ALTER COLUMN network DROP DEFAULT`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE treasury_transfer_orders ALTER COLUMN network DROP DEFAULT`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE withdraw_jobs ALTER COLUMN network DROP DEFAULT`)
	_, _ = l.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_withdraw_jobs_status_retry ON withdraw_jobs (status, next_retry_at, created_at, id)`)
	_, _ = l.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_withdraw_jobs_sender_status ON withdraw_jobs (tenant_id, chain, network, from_address, status, created_at, id)`)
	_, _ = l.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_withdraw_attempts_order_status ON withdraw_tx_attempts (tenant_id, order_id, status, updated_at, id)`)
	_, _ = l.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_treasury_transfer_status ON treasury_transfer_orders (status, updated_at, id)`)
	return nil
}

func addDecimalString(a, b string) (string, error) {
	aa, err := parseIntString(a)
	if err != nil {
		return "", err
	}
	bb, err := parseIntString(b)
	if err != nil {
		return "", err
	}
	return new(big.Int).Add(aa, bb).String(), nil
}

func subDecimalString(a, b string) (string, error) {
	aa, err := parseIntString(a)
	if err != nil {
		return "", err
	}
	bb, err := parseIntString(b)
	if err != nil {
		return "", err
	}
	if aa.Cmp(bb) < 0 {
		return "", fmt.Errorf("insufficient balance")
	}
	return new(big.Int).Sub(aa, bb).String(), nil
}

func parseIntString(v string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(strings.TrimSpace(v), 10)
	if !ok {
		return nil, fmt.Errorf("invalid integer amount: %s", v)
	}
	if n.Sign() < 0 {
		return nil, fmt.Errorf("negative amount not allowed: %s", v)
	}
	return n, nil
}

func computeWithdrawable(available, withdrawLocked string) (string, error) {
	return subDecimalString(available, withdrawLocked)
}

func normalizeDepositLifecycleStatus(scanStatus, fallbackStatus string, confirmations, requiredConfs, unlockConfs int64) string {
	for _, candidate := range []string{scanStatus, fallbackStatus} {
		switch strings.ToUpper(strings.TrimSpace(candidate)) {
		case "REVERTED", "REORGED", "FAILED":
			return "REVERTED"
		case "FINALIZED":
			return "FINALIZED"
		case "CONFIRMED":
			if unlockConfs > 0 && confirmations >= unlockConfs {
				return "FINALIZED"
			}
			return "CONFIRMED"
		}
	}
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	if unlockConfs > 0 && unlockConfs < requiredConfs {
		unlockConfs = requiredConfs
	}
	if unlockConfs > 0 && confirmations >= unlockConfs {
		return "FINALIZED"
	}
	if confirmations >= requiredConfs {
		return "CONFIRMED"
	}
	return "PENDING"
}

func resolveEffectiveDepositStatus(oldStatus, targetStatus string) string {
	oldStatus = normalizeStoredDepositStatus(oldStatus)
	targetStatus = normalizeStoredDepositStatus(targetStatus)
	if targetStatus == "REVERTED" {
		return "REVERTED"
	}
	switch oldStatus {
	case "REVERTED":
		return "REVERTED"
	case "FINALIZED":
		return "FINALIZED"
	case "CONFIRMED":
		if targetStatus == "FINALIZED" {
			return "FINALIZED"
		}
		return "CONFIRMED"
	default:
		return targetStatus
	}
}

func normalizeStoredDepositStatus(v string) string {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "REVERTED", "REORGED", "FAILED":
		return "REVERTED"
	case "FINALIZED":
		return "FINALIZED"
	case "CONFIRMED":
		return "CONFIRMED"
	default:
		return "PENDING"
	}
}

func depositStatusCredits(status string) bool {
	switch normalizeStoredDepositStatus(status) {
	case "CONFIRMED", "FINALIZED":
		return true
	default:
		return false
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
