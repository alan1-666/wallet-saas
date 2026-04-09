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
	in.Model = normalizeModel(in.Model)
	in.Chain = normalizeChain(in.Chain)
	in.Network = normalizeNetwork(in.Network)
	in.Coin = normalizeCoin(in.Coin)
	in.SignType = normalizeSignType(in.SignType)
	if in.SignType == "" {
		in.SignType = "ecdsa"
	}
	if in.Model == "" || in.Chain == "" || in.Network == "" || in.Coin == "" || strings.TrimSpace(in.KeyID) == "" {
		return fmt.Errorf("model/chain/network/coin/key_id are required")
	}
	if strings.TrimSpace(in.Address) == "" || strings.TrimSpace(in.PublicKey) == "" {
		return fmt.Errorf("address/public_key are required")
	}
	meta, err := r.GetChainMetadata(ctx, in.Chain, in.Network)
	if err != nil {
		return err
	}
	if normalizeModel(meta.Model) != in.Model {
		return fmt.Errorf("chain model mismatch chain=%s network=%s require=%s actual=%s", in.Chain, in.Network, in.Model, meta.Model)
	}
	if !meta.Enabled {
		return fmt.Errorf("chain disabled chain=%s network=%s", in.Chain, in.Network)
	}
	policy, err := r.GetChainPolicy(ctx, in.Chain, in.Network)
	if err != nil {
		return err
	}
	if !policy.Enabled {
		return fmt.Errorf("chain policy disabled chain=%s network=%s", in.Chain, in.Network)
	}
	in.MinConfirmations = policy.RequiredConfirmations
	if in.MinConfirmations <= 0 {
		in.MinConfirmations = 1
	}
	in.UnlockConfirmations = maxInt64(policy.UnlockConfirmations, in.MinConfirmations)
	if strings.TrimSpace(in.TreasuryAccountID) == "" {
		in.TreasuryAccountID = "treasury-main"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.upsertAccountTx(ctx, tx, in.TenantID, in.AccountID, "ACTIVE", ""); err != nil {
		return err
	}

	if err := r.upsertWalletAddressTx(ctx, tx, in); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO scan_watch_addresses (
  tenant_id, account_id, model, chain, coin, network, address,
  min_confirmations, unlock_confirmations, treasury_account_id, cold_account_id, auto_sweep, sweep_threshold, hot_balance_cap, active
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,TRUE)
ON CONFLICT (model, chain, coin, network, address, tenant_id, account_id)
DO UPDATE SET
  min_confirmations=EXCLUDED.min_confirmations,
  unlock_confirmations=EXCLUDED.unlock_confirmations,
  treasury_account_id=EXCLUDED.treasury_account_id,
  cold_account_id=EXCLUDED.cold_account_id,
  auto_sweep=EXCLUDED.auto_sweep,
  sweep_threshold=EXCLUDED.sweep_threshold,
  hot_balance_cap=EXCLUDED.hot_balance_cap,
  active=TRUE,
  updated_at=NOW()
`, in.TenantID, in.AccountID, in.Model, in.Chain, in.Coin, in.Network, in.Address, in.MinConfirmations, in.UnlockConfirmations, in.TreasuryAccountID, in.ColdAccountID, in.AutoSweep, in.SweepThreshold, in.HotBalanceCap)
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
	if _, err := r.db.ExecContext(ctx, `
UPDATE wallet_accounts
SET hd_account_index = id
WHERE tenant_id=$1 AND account_id=$2 AND (hd_account_index IS NULL OR hd_account_index < 0)
`, in.TenantID, in.AccountID); err != nil {
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
SELECT tenant_id, account_id, model, chain, coin, network, address, key_id, public_key, sign_type, address_type, derivation_path, change_index, address_index, status,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       TO_CHAR(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM wallet_addresses
WHERE tenant_id=$1 AND account_id=$2
ORDER BY address_index ASC, id ASC
`, tenantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ports.WalletAddress, 0, 8)
	for rows.Next() {
		var item ports.WalletAddress
		if err := rows.Scan(&item.TenantID, &item.AccountID, &item.Model, &item.Chain, &item.Coin, &item.Network, &item.Address, &item.KeyID, &item.PublicKey, &item.SignType, &item.AddressType, &item.DerivationPath, &item.ChangeIndex, &item.AddressIndex, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRegistry) PrepareAddress(ctx context.Context, in ports.PrepareAddressInput) (ports.PreparedAddress, error) {
	in.Model = normalizeModel(in.Model)
	in.Chain = normalizeChain(in.Chain)
	in.Network = normalizeNetwork(in.Network)
	in.SignType = normalizeSignType(in.SignType)
	if in.Model == "" || in.Chain == "" || in.Network == "" || in.SignType == "" {
		return ports.PreparedAddress{}, fmt.Errorf("model/chain/network/sign_type are required")
	}

	var existing ports.WalletAddress
	err := r.db.QueryRowContext(ctx, `
SELECT tenant_id, account_id, model, chain, coin, network, address, key_id, public_key, sign_type, address_type, derivation_path, change_index, address_index, status,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       TO_CHAR(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM wallet_addresses
WHERE tenant_id=$1
  AND account_id=$2
  AND LOWER(chain)=LOWER($3)
  AND LOWER(network)=LOWER($4)
  AND UPPER(COALESCE(status, 'ACTIVE'))='ACTIVE'
  AND COALESCE(key_id, '') <> ''
ORDER BY address_index ASC, id ASC
LIMIT 1
`, in.TenantID, in.AccountID, in.Chain, in.Network).Scan(
		&existing.TenantID, &existing.AccountID, &existing.Model, &existing.Chain, &existing.Coin, &existing.Network, &existing.Address, &existing.KeyID, &existing.PublicKey, &existing.SignType, &existing.AddressType, &existing.DerivationPath, &existing.ChangeIndex, &existing.AddressIndex, &existing.Status, &existing.CreatedAt, &existing.UpdatedAt,
	)
	if err == nil {
		return ports.PreparedAddress{WalletAddress: existing, Existing: true}, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return ports.PreparedAddress{}, err
	}

	var accountIndex int64
	if err := r.db.QueryRowContext(ctx, `
SELECT hd_account_index
FROM wallet_accounts
WHERE tenant_id=$1 AND account_id=$2
`, in.TenantID, in.AccountID).Scan(&accountIndex); err != nil {
		return ports.PreparedAddress{}, err
	}
	if accountIndex < 0 {
		return ports.PreparedAddress{}, fmt.Errorf("invalid hd account index")
	}

	var addressIndex int64
	if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(address_index), -1) + 1
FROM wallet_addresses
WHERE tenant_id=$1
  AND account_id=$2
  AND LOWER(chain)=LOWER($3)
  AND LOWER(network)=LOWER($4)
  AND COALESCE(key_id, '') <> ''
`, in.TenantID, in.AccountID, in.Chain, in.Network).Scan(&addressIndex); err != nil {
		return ports.PreparedAddress{}, err
	}
	keyID := buildHDKeyID(in.SignType, in.Chain, accountIndex, 0, addressIndex)
	derivationPath, err := buildHDDerivationPath(in.SignType, in.Chain, accountIndex, 0, addressIndex)
	if err != nil {
		return ports.PreparedAddress{}, err
	}
	return ports.PreparedAddress{
		WalletAddress: ports.WalletAddress{
			TenantID:       in.TenantID,
			AccountID:      in.AccountID,
			Model:          in.Model,
			Chain:          in.Chain,
			Network:        in.Network,
			KeyID:          keyID,
			SignType:       in.SignType,
			AddressType:    in.AddressType,
			DerivationPath: derivationPath,
			ChangeIndex:    0,
			AddressIndex:   addressIndex,
			Status:         "ACTIVE",
		},
		Existing: false,
	}, nil
}

func (r *PostgresRegistry) GetAccountAddressByKeyID(ctx context.Context, tenantID, accountID, keyID string) (ports.WalletAddress, error) {
	var out ports.WalletAddress
	err := r.db.QueryRowContext(ctx, `
SELECT tenant_id, account_id, model, chain, coin, network, address, key_id, public_key, sign_type, address_type, derivation_path, change_index, address_index, status,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       TO_CHAR(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM wallet_addresses
WHERE tenant_id=$1
  AND account_id=$2
  AND key_id=$3
ORDER BY id DESC
LIMIT 1
`, tenantID, accountID, keyID).Scan(
		&out.TenantID, &out.AccountID, &out.Model, &out.Chain, &out.Coin, &out.Network, &out.Address, &out.KeyID, &out.PublicKey, &out.SignType, &out.AddressType, &out.DerivationPath, &out.ChangeIndex, &out.AddressIndex, &out.Status, &out.CreatedAt, &out.UpdatedAt,
	)
	return out, err
}

func (r *PostgresRegistry) GetChainMetadata(ctx context.Context, chain, network string) (ports.ChainMetadata, error) {
	chain = normalizeChain(chain)
	network = normalizeNetwork(network)
	if chain == "" || network == "" {
		return ports.ChainMetadata{}, fmt.Errorf("chain and network are required")
	}
	var out ports.ChainMetadata
	err := r.db.QueryRowContext(ctx, `
SELECT chain, network, model, native_asset, min_confirmations, enabled
FROM chain_metadata
WHERE chain=$1
  AND network IN ($2, '*')
ORDER BY CASE WHEN network=$2 THEN 0 ELSE 1 END
LIMIT 1
`, chain, network).Scan(&out.Chain, &out.Network, &out.Model, &out.NativeAsset, &out.MinConfirmations, &out.Enabled)
	if err == sql.ErrNoRows {
		return ports.ChainMetadata{}, fmt.Errorf("unsupported chain/network chain=%s network=%s", chain, network)
	}
	if err != nil {
		return ports.ChainMetadata{}, err
	}
	return out, nil
}

func (r *PostgresRegistry) GetChainPolicy(ctx context.Context, chain, network string) (ports.ChainPolicy, error) {
	chain = normalizeChain(chain)
	network = normalizeNetwork(network)
	if chain == "" || network == "" {
		return ports.ChainPolicy{}, fmt.Errorf("chain and network are required")
	}
	var out ports.ChainPolicy
	err := r.db.QueryRowContext(ctx, `
SELECT chain, network, required_confirmations, safe_depth, reorg_window, fee_policy, enabled
FROM chain_policies
WHERE chain=$1
  AND network IN ($2, '*')
ORDER BY CASE WHEN network=$2 THEN 0 ELSE 1 END
LIMIT 1
`, chain, network).Scan(&out.Chain, &out.Network, &out.RequiredConfirmations, &out.SafeDepth, &out.ReorgWindow, &out.FeePolicy, &out.Enabled)
	if err == sql.ErrNoRows {
		meta, metaErr := r.GetChainMetadata(ctx, chain, network)
		if metaErr != nil {
			return ports.ChainPolicy{}, metaErr
		}
		return ports.ChainPolicy{
			Chain:                 meta.Chain,
			Network:               meta.Network,
			RequiredConfirmations: maxInt64(meta.MinConfirmations, 1),
			UnlockConfirmations:   maxInt64(meta.MinConfirmations, 1),
			SafeDepth:             maxInt64(meta.MinConfirmations, 1),
			ReorgWindow:           maxInt64(meta.MinConfirmations*3, 6),
			FeePolicy:             "{}",
			Enabled:               meta.Enabled,
		}, nil
	}
	if err != nil {
		return ports.ChainPolicy{}, err
	}
	if out.RequiredConfirmations <= 0 {
		out.RequiredConfirmations = 1
	}
	if out.SafeDepth <= 0 {
		out.SafeDepth = out.RequiredConfirmations
	}
	out.UnlockConfirmations = maxInt64(out.SafeDepth, out.RequiredConfirmations)
	if out.ReorgWindow <= 0 {
		out.ReorgWindow = maxInt64(out.RequiredConfirmations*3, 6)
	}
	return out, nil
}

func (r *PostgresRegistry) upsertWalletAddressTx(ctx context.Context, tx *sql.Tx, in ports.WatchAddressInput) error {
	updateByKeyID, err := tx.ExecContext(ctx, `
UPDATE wallet_addresses
SET account_id=$2,
    model=$3,
    chain=$4,
    network=$5,
    address=$6,
    public_key=$7,
    sign_type=$8,
    address_type=$9,
    derivation_path=$10,
    change_index=$11,
    address_index=$12,
    status='ACTIVE',
    updated_at=NOW()
WHERE tenant_id=$1
  AND key_id=$13
`, in.TenantID, in.AccountID, in.Model, in.Chain, in.Network, in.Address, in.PublicKey, in.SignType, in.AddressType, in.DerivationPath, in.ChangeIndex, in.AddressIndex, in.KeyID)
	if err != nil {
		return err
	}
	if rows, _ := updateByKeyID.RowsAffected(); rows > 0 {
		return nil
	}

	updateByAddress, err := tx.ExecContext(ctx, `
UPDATE wallet_addresses
SET model=$3,
    chain=$4,
    network=$5,
    address=$6,
    key_id=CASE WHEN COALESCE(key_id, '') = '' THEN $13 ELSE key_id END,
    public_key=$7,
    sign_type=$8,
    address_type=$9,
    derivation_path=$10,
    change_index=$11,
    address_index=$12,
    status='ACTIVE',
    updated_at=NOW()
WHERE tenant_id=$1
  AND account_id=$2
  AND LOWER(chain)=LOWER($4)
  AND LOWER(network)=LOWER($5)
  AND LOWER(address)=LOWER($6)
`, in.TenantID, in.AccountID, in.Model, in.Chain, in.Network, in.Address, in.PublicKey, in.SignType, in.AddressType, in.DerivationPath, in.ChangeIndex, in.AddressIndex, in.KeyID)
	if err != nil {
		return err
	}
	if rows, _ := updateByAddress.RowsAffected(); rows > 0 {
		return nil
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO wallet_addresses (
  tenant_id, account_id, model, chain, coin, network, address, key_id, public_key, sign_type, address_type, derivation_path, change_index, address_index, status
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'ACTIVE')
`, in.TenantID, in.AccountID, in.Model, in.Chain, in.Coin, in.Network, in.Address, in.KeyID, in.PublicKey, in.SignType, in.AddressType, in.DerivationPath, in.ChangeIndex, in.AddressIndex)
	return err
}

func (r *PostgresRegistry) upsertAccountTx(ctx context.Context, tx *sql.Tx, tenantID, accountID, status, remark string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO wallet_accounts (tenant_id, account_id, status, remark)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, account_id) DO NOTHING
`, tenantID, accountID, status, remark)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE wallet_accounts
SET hd_account_index = id
WHERE tenant_id=$1 AND account_id=$2 AND (hd_account_index IS NULL OR hd_account_index < 0)
`, tenantID, accountID)
	return err
}

func (r *PostgresRegistry) ensureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS wallet_accounts (
id BIGSERIAL PRIMARY KEY,
tenant_id TEXT NOT NULL,
account_id TEXT NOT NULL,
hd_account_index BIGINT NULL,
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
network TEXT NOT NULL,
address TEXT NOT NULL,
key_id TEXT NOT NULL DEFAULT '',
public_key TEXT NOT NULL,
sign_type TEXT NOT NULL DEFAULT 'ecdsa',
address_type TEXT NOT NULL DEFAULT '',
derivation_path TEXT NOT NULL DEFAULT '',
change_index BIGINT NOT NULL DEFAULT 0,
address_index BIGINT NOT NULL DEFAULT 0,
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
network TEXT NOT NULL,
address TEXT NOT NULL,
min_confirmations BIGINT NOT NULL DEFAULT 1,
unlock_confirmations BIGINT NOT NULL DEFAULT 1,
treasury_account_id TEXT NOT NULL DEFAULT 'treasury-main',
cold_account_id TEXT NOT NULL DEFAULT '',
auto_sweep BOOLEAN NOT NULL DEFAULT FALSE,
sweep_threshold TEXT NOT NULL DEFAULT '0',
hot_balance_cap TEXT NOT NULL DEFAULT '0',
active BOOLEAN NOT NULL DEFAULT TRUE,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`,
		`CREATE TABLE IF NOT EXISTS chain_metadata (
id BIGSERIAL PRIMARY KEY,
chain TEXT NOT NULL,
network TEXT NOT NULL,
model TEXT NOT NULL,
native_asset TEXT NOT NULL DEFAULT '',
min_confirmations BIGINT NOT NULL DEFAULT 1,
enabled BOOLEAN NOT NULL DEFAULT TRUE,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (chain, network)
);`,
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
);`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("registry schema init failed: %w", err)
		}
	}
	if _, err := r.db.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS uq_scan_watch_addr_tenant
ON scan_watch_addresses (model, chain, coin, network, address, tenant_id, account_id)
`); err != nil {
		return fmt.Errorf("registry schema index failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_wallet_addresses_key_id
ON wallet_addresses (tenant_id, account_id, key_id)
`); err != nil {
		return fmt.Errorf("registry wallet_addresses key index failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS uq_chain_metadata_chain_network
ON chain_metadata (chain, network)
`); err != nil {
		return fmt.Errorf("registry chain_metadata index failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS uq_chain_policies_chain_network
ON chain_policies (chain, network)
`); err != nil {
		return fmt.Errorf("registry chain_policies index failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE scan_watch_addresses ALTER COLUMN auto_sweep SET DEFAULT FALSE`); err != nil {
		return fmt.Errorf("registry schema alter default failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE scan_watch_addresses ADD COLUMN IF NOT EXISTS cold_account_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("registry schema alter scan_watch_addresses cold_account_id failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE scan_watch_addresses ADD COLUMN IF NOT EXISTS hot_balance_cap TEXT NOT NULL DEFAULT '0'`); err != nil {
		return fmt.Errorf("registry schema alter scan_watch_addresses hot_balance_cap failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE scan_watch_addresses ADD COLUMN IF NOT EXISTS unlock_confirmations BIGINT NOT NULL DEFAULT 1`); err != nil {
		return fmt.Errorf("registry schema alter scan_watch_addresses unlock_confirmations failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE wallet_addresses ALTER COLUMN network DROP DEFAULT`); err != nil {
		return fmt.Errorf("registry schema drop wallet_addresses network default failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE scan_watch_addresses ALTER COLUMN network DROP DEFAULT`); err != nil {
		return fmt.Errorf("registry schema drop scan_watch_addresses network default failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE scan_watch_addresses SET unlock_confirmations = GREATEST(COALESCE(unlock_confirmations, 0), COALESCE(min_confirmations, 1), 1) WHERE unlock_confirmations IS NULL OR unlock_confirmations <= 0 OR unlock_confirmations < min_confirmations`); err != nil {
		return fmt.Errorf("registry schema backfill scan_watch_addresses unlock_confirmations failed: %w", err)
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
ALTER TABLE wallet_accounts ADD COLUMN IF NOT EXISTS hd_account_index BIGINT
`); err != nil {
		return fmt.Errorf("registry schema alter hd_account_index failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
ALTER TABLE wallet_addresses ADD COLUMN IF NOT EXISTS key_id TEXT NOT NULL DEFAULT ''
`); err != nil {
		return fmt.Errorf("registry schema alter key_id failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
ALTER TABLE wallet_addresses ADD COLUMN IF NOT EXISTS address_type TEXT NOT NULL DEFAULT ''
`); err != nil {
		return fmt.Errorf("registry schema alter address_type failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
ALTER TABLE wallet_addresses ADD COLUMN IF NOT EXISTS derivation_path TEXT NOT NULL DEFAULT ''
`); err != nil {
		return fmt.Errorf("registry schema alter derivation_path failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
ALTER TABLE wallet_addresses ADD COLUMN IF NOT EXISTS change_index BIGINT NOT NULL DEFAULT 0
`); err != nil {
		return fmt.Errorf("registry schema alter change_index failed: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
ALTER TABLE wallet_addresses ADD COLUMN IF NOT EXISTS address_index BIGINT NOT NULL DEFAULT 0
`); err != nil {
		return fmt.Errorf("registry schema alter address_index failed: %w", err)
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
	if _, err := r.db.ExecContext(ctx, `
UPDATE wallet_accounts
SET hd_account_index = id
WHERE hd_account_index IS NULL OR hd_account_index < 0
`); err != nil {
		return fmt.Errorf("registry backfill hd_account_index failed: %w", err)
	}
	if err := r.seedChainMetadata(ctx); err != nil {
		return err
	}
	if err := r.seedChainPolicies(ctx); err != nil {
		return err
	}
	return nil
}

func (r *PostgresRegistry) seedChainMetadata(ctx context.Context) error {
	stmts := []string{
		`INSERT INTO chain_metadata (chain, network, model, native_asset, min_confirmations, enabled) VALUES
('ethereum','mainnet','account','ETH',12,TRUE),
('ethereum','sepolia','account','ETH',1,TRUE),
('binance','mainnet','account','BNB',12,TRUE),
('binance','testnet','account','BNB',1,TRUE),
('polygon','mainnet','account','MATIC',64,TRUE),
('polygon','amoy','account','MATIC',1,TRUE),
('arbitrum','mainnet','account','ETH',12,TRUE),
('arbitrum','sepolia','account','ETH',1,TRUE),
('optimism','mainnet','account','ETH',12,TRUE),
('linea','mainnet','account','ETH',12,TRUE),
('scroll','mainnet','account','ETH',12,TRUE),
('mantle','mainnet','account','MNT',12,TRUE),
('zksync','mainnet','account','ETH',12,TRUE),
('tron','mainnet','account','TRX',20,TRUE),
('tron','nile','account','TRX',1,TRUE),
('solana','mainnet','account','SOL',32,TRUE),
('solana','devnet','account','SOL',1,TRUE),
('bitcoin','mainnet','utxo','BTC',1,TRUE),
('bitcoin','testnet','utxo','BTC',1,TRUE),
('bitcoincash','mainnet','utxo','BCH',1,TRUE),
('dash','mainnet','utxo','DASH',1,TRUE),
('dogecoin','mainnet','utxo','DOGE',6,TRUE),
('litecoin','mainnet','utxo','LTC',6,TRUE),
('zen','mainnet','utxo','ZEN',6,TRUE)
ON CONFLICT (chain, network) DO NOTHING;`,
		`INSERT INTO chain_metadata (chain, network, model, native_asset, min_confirmations, enabled) VALUES
('ethereum','*','account','ETH',12,TRUE),
('binance','*','account','BNB',12,TRUE),
('polygon','*','account','MATIC',64,TRUE),
('arbitrum','*','account','ETH',12,TRUE),
('optimism','*','account','ETH',12,TRUE),
('linea','*','account','ETH',12,TRUE),
('scroll','*','account','ETH',12,TRUE),
('mantle','*','account','MNT',12,TRUE),
('zksync','*','account','ETH',12,TRUE),
('tron','*','account','TRX',20,TRUE),
('solana','*','account','SOL',32,TRUE),
('bitcoin','*','utxo','BTC',1,TRUE),
('bitcoincash','*','utxo','BCH',1,TRUE),
('dash','*','utxo','DASH',1,TRUE),
('dogecoin','*','utxo','DOGE',6,TRUE),
('litecoin','*','utxo','LTC',6,TRUE),
('zen','*','utxo','ZEN',6,TRUE)
ON CONFLICT (chain, network) DO NOTHING;`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("seed chain_metadata failed: %w", err)
		}
	}
	return nil
}

func (r *PostgresRegistry) seedChainPolicies(ctx context.Context) error {
	stmts := []string{
		`INSERT INTO chain_policies (chain, network, required_confirmations, safe_depth, reorg_window, fee_policy, enabled) VALUES
('ethereum','mainnet',12,12,64,'{}',TRUE),
('ethereum','sepolia',1,1,12,'{}',TRUE),
('binance','mainnet',12,12,64,'{}',TRUE),
('binance','testnet',1,1,12,'{}',TRUE),
('polygon','mainnet',64,64,256,'{}',TRUE),
('polygon','amoy',1,1,12,'{}',TRUE),
('arbitrum','mainnet',12,12,64,'{}',TRUE),
('arbitrum','sepolia',1,1,12,'{}',TRUE),
('optimism','mainnet',12,12,64,'{}',TRUE),
('linea','mainnet',12,12,64,'{}',TRUE),
('scroll','mainnet',12,12,64,'{}',TRUE),
('mantle','mainnet',12,12,64,'{}',TRUE),
('zksync','mainnet',12,12,64,'{}',TRUE),
('tron','mainnet',20,20,120,'{}',TRUE),
('tron','nile',1,1,12,'{}',TRUE),
('solana','mainnet',32,32,160,'{}',TRUE),
('solana','devnet',1,1,12,'{}',TRUE),
('bitcoin','mainnet',1,1,12,'{}',TRUE),
('bitcoin','testnet',1,1,12,'{}',TRUE),
('bitcoincash','mainnet',1,1,12,'{}',TRUE),
('dash','mainnet',1,1,12,'{}',TRUE),
('dogecoin','mainnet',6,6,24,'{}',TRUE),
('litecoin','mainnet',6,6,24,'{}',TRUE),
('zen','mainnet',6,6,24,'{}',TRUE)
ON CONFLICT (chain, network) DO NOTHING;`,
		`INSERT INTO chain_policies (chain, network, required_confirmations, safe_depth, reorg_window, fee_policy, enabled) VALUES
('ethereum','*',12,12,64,'{}',TRUE),
('binance','*',12,12,64,'{}',TRUE),
('polygon','*',64,64,256,'{}',TRUE),
('arbitrum','*',12,12,64,'{}',TRUE),
('optimism','*',12,12,64,'{}',TRUE),
('linea','*',12,12,64,'{}',TRUE),
('scroll','*',12,12,64,'{}',TRUE),
('mantle','*',12,12,64,'{}',TRUE),
('zksync','*',12,12,64,'{}',TRUE),
('tron','*',20,20,120,'{}',TRUE),
('solana','*',32,32,160,'{}',TRUE),
('bitcoin','*',1,1,12,'{}',TRUE),
('bitcoincash','*',1,1,12,'{}',TRUE),
('dash','*',1,1,12,'{}',TRUE),
('dogecoin','*',6,6,24,'{}',TRUE),
('litecoin','*',6,6,24,'{}',TRUE),
('zen','*',6,6,24,'{}',TRUE)
ON CONFLICT (chain, network) DO NOTHING;`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("seed chain_policies failed: %w", err)
		}
	}
	return nil
}

func normalizeChain(v string) string {
	x := strings.ToLower(strings.TrimSpace(v))
	switch x {
	case "eth":
		return "ethereum"
	case "bsc", "bnb":
		return "binance"
	case "matic":
		return "polygon"
	case "arb":
		return "arbitrum"
	case "trx":
		return "tron"
	case "sol":
		return "solana"
	case "btc":
		return "bitcoin"
	case "bch":
		return "bitcoincash"
	case "ltc":
		return "litecoin"
	case "doge":
		return "dogecoin"
	default:
		return x
	}
}

func normalizeNetwork(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeModel(v string) string {
	x := strings.ToLower(strings.TrimSpace(v))
	switch x {
	case "account", "utxo":
		return x
	default:
		return ""
	}
}

func normalizeCoin(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
