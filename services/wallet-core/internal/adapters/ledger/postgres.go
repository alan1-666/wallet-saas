package ledger

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strings"

	_ "github.com/lib/pq"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type PostgresLedger struct {
	db *sql.DB
}

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

	if asset == "" || chain == "" || network == "" {
		return fmt.Errorf("chain/network/asset are required")
	}
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	var existingStatus string
	err = tx.QueryRowContext(ctx, `
SELECT status FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, tenantID, orderID).Scan(&existingStatus)
	if err == nil {
		if existingStatus == "FROZEN" || existingStatus == "BROADCASTED" || existingStatus == "CONFIRMED" {
			return tx.Commit()
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
	avail, err := subDecimalString(current.Available, amount)
	if err != nil {
		return err
	}
	frozen, err := addDecimalString(current.Frozen, amount)
	if err != nil {
		return err
	}

	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, avail, frozen); err != nil {
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

	return tx.Commit()
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
	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, current.Available, frozen); err != nil {
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
	return tx.Commit()
}

func (l *PostgresLedger) FailWithdrawOnChain(ctx context.Context, tenantID, orderID, reason string, confirmations int64) error {
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
	)
	err = tx.QueryRowContext(ctx, `
SELECT account_id, asset, amount, status
FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, tenantID, orderID).Scan(&accountID, &asset, &amount, &status)
	if err != nil {
		return err
	}
	if status == "RELEASED" {
		return tx.Commit()
	}
	if status == "CONFIRMED" {
		return fmt.Errorf("withdraw already confirmed")
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
	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, avail, frozen); err != nil {
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
	if err := l.upsertBalanceTx(ctx, tx, tenantID, accountID, asset, avail, frozen); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_journals (tenant_id, biz_type, biz_id, account_id, asset, entry_side, amount, balance_after)
VALUES ($1,'WITHDRAW',$2,$3,$4,'UNFREEZE',$5,$6)
ON CONFLICT (tenant_id, biz_type, biz_id, account_id, entry_side) DO NOTHING
`, tenantID, orderID, accountID, asset, amount, avail); err != nil {
		return err
	}

	return tx.Commit()
}

func (l *PostgresLedger) GetWithdrawStatus(ctx context.Context, tenantID, orderID string) (ports.LedgerStatus, error) {
	var out ports.LedgerStatus
	err := l.db.QueryRowContext(ctx, `
SELECT status, tx_hash, reason, amount
FROM ledger_freezes
WHERE tenant_id=$1 AND order_id=$2
`, tenantID, orderID).Scan(&out.Status, &out.TxHash, &out.Reason, &out.Amount)
	if err != nil {
		return ports.LedgerStatus{}, err
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
	targetStatus := normalizeDepositStatus(strings.TrimSpace(in.Status))
	switch targetStatus {
	case "REVERTED":
	case "PENDING":
	case "CONFIRMED":
	default:
		if in.Confirmations >= requiredConfs {
			targetStatus = "CONFIRMED"
		} else {
			targetStatus = "PENDING"
		}
	}

	var (
		oldStatus string
		oldCredit bool
	)
	err = tx.QueryRowContext(ctx, `
SELECT status, credited
FROM deposit_events
WHERE tenant_id=$1 AND order_id=$2
FOR UPDATE
`, in.TenantID, in.OrderID).Scan(&oldStatus, &oldCredit)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
INSERT INTO deposit_events (tenant_id, account_id, order_id, chain, network, coin, amount, tx_hash, from_address, to_address, confirmations, required_confirmations, status, credited)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,FALSE)
`, in.TenantID, in.AccountID, in.OrderID, in.Chain, in.Network, in.Coin, in.Amount, in.TxHash, in.FromAddress, in.ToAddress, in.Confirmations, requiredConfs, targetStatus)
		if err != nil {
			return err
		}
		oldStatus = ""
		oldCredit = false
	} else if err != nil {
		return err
	}

	effectiveStatus := targetStatus
	switch normalizeDepositStatus(oldStatus) {
	case "REVERTED":
		effectiveStatus = "REVERTED"
	case "CONFIRMED":
		if targetStatus == "PENDING" {
			effectiveStatus = "CONFIRMED"
		}
	}
	if targetStatus == "REVERTED" {
		effectiveStatus = "REVERTED"
	}

	newCredited := oldCredit
	if !oldCredit && effectiveStatus == "CONFIRMED" {
		bal, err := l.getBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin)
		if err != nil {
			return err
		}
		avail, err := addDecimalString(bal.Available, in.Amount)
		if err != nil {
			return err
		}
		if err := l.upsertBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin, avail, bal.Frozen); err != nil {
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

	if oldCredit && effectiveStatus == "REVERTED" {
		bal, err := l.getBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin)
		if err != nil {
			return err
		}
		avail, err := subDecimalString(bal.Available, in.Amount)
		if err != nil {
			return err
		}
		if err := l.upsertBalanceTx(ctx, tx, in.TenantID, in.AccountID, in.Coin, avail, bal.Frozen); err != nil {
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
		_ = l.markReorgRiskTx(ctx, tx, in)
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
    status=$11,
    credited=$12,
    updated_at=NOW()
WHERE tenant_id=$13 AND order_id=$14
`, in.AccountID, in.Chain, in.Network, in.Coin, in.Amount, in.TxHash, in.FromAddress, in.ToAddress, in.Confirmations, requiredConfs, effectiveStatus, newCredited, in.TenantID, in.OrderID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (l *PostgresLedger) markReorgRiskTx(ctx context.Context, tx *sql.Tx, in ports.DepositCreditInput) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO risk_events (tenant_id, account_id, order_id, chain, coin, amount, rule_limit, decision)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (tenant_id, order_id)
DO UPDATE SET
  account_id=EXCLUDED.account_id,
  chain=EXCLUDED.chain,
  coin=EXCLUDED.coin,
  amount=EXCLUDED.amount,
  rule_limit=EXCLUDED.rule_limit,
  decision=EXCLUDED.decision
`, in.TenantID, in.AccountID, in.OrderID, in.Chain, in.Coin, in.Amount, "REORG", "REORG_REVERTED")
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "risk_events") {
		return nil
	}
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

func (l *PostgresLedger) GetBalance(ctx context.Context, tenantID, accountID, asset string) (ports.BalanceSnapshot, error) {
	var out ports.BalanceSnapshot
	err := l.db.QueryRowContext(ctx, `
SELECT available, frozen
FROM ledger_balances
WHERE tenant_id=$1 AND account_id=$2 AND asset=$3
`, tenantID, accountID, asset).Scan(&out.Available, &out.Frozen)
	if err == sql.ErrNoRows {
		return ports.BalanceSnapshot{Available: "0", Frozen: "0"}, nil
	}
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
		if err := rows.Scan(&item.Asset, &item.Available, &item.Frozen, &item.VaultAvailable); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (l *PostgresLedger) getBalanceTx(ctx context.Context, tx *sql.Tx, tenantID, accountID, asset string) (ports.BalanceSnapshot, error) {
	var out ports.BalanceSnapshot
	err := tx.QueryRowContext(ctx, `
SELECT available, frozen
FROM ledger_balances
WHERE tenant_id=$1 AND account_id=$2 AND asset=$3
FOR UPDATE
`, tenantID, accountID, asset).Scan(&out.Available, &out.Frozen)
	if err == sql.ErrNoRows {
		return ports.BalanceSnapshot{Available: "0", Frozen: "0"}, nil
	}
	if err != nil {
		return ports.BalanceSnapshot{}, err
	}
	return out, nil
}

func (l *PostgresLedger) upsertBalanceTx(ctx context.Context, tx *sql.Tx, tenantID, accountID, asset, available, frozen string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO ledger_balances (tenant_id, account_id, asset, available, frozen)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (tenant_id, account_id, asset)
DO UPDATE SET available=EXCLUDED.available, frozen=EXCLUDED.frozen, updated_at=NOW()
`, tenantID, accountID, asset, available, frozen)
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
status TEXT NOT NULL,
credited BOOLEAN NOT NULL DEFAULT FALSE,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tenant_id, order_id)
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
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS required_confirmations BIGINT NOT NULL DEFAULT 1;`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS credited BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
		`ALTER TABLE deposit_events ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'unknown';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS chain TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'unknown';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS tx_hash TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS confirmations BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS required_confirmations BIGINT NOT NULL DEFAULT 1;`,
		`ALTER TABLE sweep_orders ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();`,
	}
	for _, stmt := range alterStmts {
		if _, err := l.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ledger schema alter failed: %w", err)
		}
	}
	_, _ = l.db.ExecContext(ctx, `UPDATE ledger_freezes SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `UPDATE deposit_events SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `UPDATE sweep_orders SET network='unknown' WHERE network='' OR network IS NULL`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE ledger_freezes ALTER COLUMN network DROP DEFAULT`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE deposit_events ALTER COLUMN network DROP DEFAULT`)
	_, _ = l.db.ExecContext(ctx, `ALTER TABLE sweep_orders ALTER COLUMN network DROP DEFAULT`)
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

func normalizeDepositStatus(v string) string {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "REVERTED":
		return "REVERTED"
	case "CONFIRMED":
		return "CONFIRMED"
	default:
		return "PENDING"
	}
}
