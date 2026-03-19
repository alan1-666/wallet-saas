package ports

import "context"

type LedgerPort interface {
	FreezeWithdraw(ctx context.Context, tenantID, accountID, orderID, chain, network, asset, amount string, requiredConfs int64) error
	ConfirmWithdraw(ctx context.Context, tenantID, accountID, orderID, txHash string) error
	ConfirmWithdrawOnChain(ctx context.Context, tenantID, orderID, txHash string, confirmations, requiredConfs int64) error
	FailWithdrawOnChain(ctx context.Context, tenantID, orderID, reason string, confirmations int64) error
	ReleaseWithdraw(ctx context.Context, tenantID, accountID, orderID, reason string) error
	GetWithdrawStatus(ctx context.Context, tenantID, orderID string) (LedgerStatus, error)
	CreditDeposit(ctx context.Context, in DepositCreditInput) error
	StartSweep(ctx context.Context, in SweepCollectInput) error
	ConfirmSweepOnChain(ctx context.Context, in SweepConfirmInput) error
	FailSweepOnChain(ctx context.Context, tenantID, sweepOrderID, reason string, confirmations int64) error
	GetBalance(ctx context.Context, tenantID, accountID, asset string) (BalanceSnapshot, error)
	ListAccountAssets(ctx context.Context, tenantID, accountID string) ([]AccountAsset, error)
}

type SignPort interface {
	SignMessage(ctx context.Context, signType, keyID, messageHash string) (string, error)
}

type DerivedKey struct {
	KeyID              string
	PublicKey          string
	AlternatePublicKey string
	DerivationPath     string
}

type KeyManagePort interface {
	DeriveKey(ctx context.Context, signType, keyID string) (DerivedKey, error)
}

type SignerRef struct {
	KeyID     string
	PublicKey string
}

type TxVin struct {
	Hash    string
	Index   uint32
	Amount  int64
	Address string
}

type TxVout struct {
	Address string
	Amount  int64
	Index   uint32
}

type BuildUnsignedParams struct {
	Chain           string
	Network         string
	Coin            string
	From            string
	To              string
	Amount          string
	Base64Tx        string
	ContractAddress string
	AmountUnit      string
	TokenDecimals   uint32
	Fee             string
	Vin             []TxVin
	Vout            []TxVout
}

type BuildUnsignedResult struct {
	UnsignedTx string
	SignHashes []string
}

type ChainBalance struct {
	Balance  string
	Network  string
	Sequence string
}

type LedgerStatus struct {
	Status string
	TxHash string
	Reason string
	Amount string
}

type BalanceSnapshot struct {
	Available string
	Frozen    string
}

type AccountAsset struct {
	Asset          string `json:"asset"`
	Available      string `json:"available"`
	Frozen         string `json:"frozen"`
	VaultAvailable string `json:"vault_available"`
}

type DepositCreditInput struct {
	TenantID      string
	AccountID     string
	OrderID       string
	Chain         string
	Network       string
	Coin          string
	Amount        string
	TxHash        string
	FromAddress   string
	ToAddress     string
	Confirmations int64
	RequiredConfs int64
	Status        string
}

type SweepCollectInput struct {
	TenantID          string
	FromAccountID     string
	TreasuryAccountID string
	SweepOrderID      string
	Chain             string
	Network           string
	Asset             string
	Amount            string
	TxHash            string
	RequiredConfs     int64
}

type SweepConfirmInput struct {
	TenantID      string
	SweepOrderID  string
	TxHash        string
	Confirmations int64
	RequiredConfs int64
}

type BroadcastParams struct {
	Chain      string
	Network    string
	Coin       string
	RawTx      string
	UnsignedTx string
	Signatures []string
	PublicKeys []string
}

type AuthScope struct {
	TenantID    string
	CanWithdraw bool
	CanDeposit  bool
	CanSweep    bool
}

type IdemResult struct {
	State     string
	Response  string
	RequestID string
}

type AuthPort interface {
	ValidateToken(ctx context.Context, token string) (AuthScope, error)
	CheckSignPermission(ctx context.Context, tenantID, keyID string) (bool, error)
	BindTenantKey(ctx context.Context, tenantID, keyID string) error
	Audit(ctx context.Context, tenantID, action, requestID, detail string) error
}

type IdempotencyPort interface {
	Reserve(ctx context.Context, tenantID, requestID, operation, requestHash string) (IdemResult, error)
	Commit(ctx context.Context, tenantID, requestID, operation, response string) error
	Reject(ctx context.Context, tenantID, requestID, operation, reason string) error
}

type ChainPort interface {
	BuildUnsignedTx(ctx context.Context, params BuildUnsignedParams) (BuildUnsignedResult, error)
	Broadcast(ctx context.Context, params BroadcastParams) (string, error)
	GetBalance(ctx context.Context, chain, coin, network, address, contractAddress string) (ChainBalance, error)
}

type ChainAddressPort interface {
	ConvertAddress(ctx context.Context, chain, network, addrType, publicKey string) (string, error)
}

type WatchAddressInput struct {
	TenantID          string
	AccountID         string
	Model             string
	Chain             string
	Coin              string
	Network           string
	Address           string
	KeyID             string
	PublicKey         string
	SignType          string
	AddressType       string
	DerivationPath    string
	ChangeIndex       int64
	AddressIndex      int64
	MinConfirmations  int64
	TreasuryAccountID string
	AutoSweep         bool
	SweepThreshold    string
}

type WalletAccount struct {
	TenantID   string `json:"tenant_id"`
	AccountID  string `json:"account_id"`
	AccountTag string `json:"account_tag"`
	Status     string `json:"status"`
	Remark     string `json:"remark"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type WalletAddress struct {
	TenantID       string `json:"tenant_id"`
	AccountID      string `json:"account_id"`
	Model          string `json:"model"`
	Chain          string `json:"chain"`
	Coin           string `json:"coin"`
	Network        string `json:"network"`
	Address        string `json:"address"`
	KeyID          string `json:"key_id"`
	PublicKey      string `json:"public_key"`
	SignType       string `json:"sign_type"`
	AddressType    string `json:"address_type"`
	DerivationPath string `json:"derivation_path"`
	ChangeIndex    int64  `json:"change_index"`
	AddressIndex   int64  `json:"address_index"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type ChainMetadata struct {
	Chain            string `json:"chain"`
	Network          string `json:"network"`
	Model            string `json:"model"`
	NativeAsset      string `json:"native_asset"`
	MinConfirmations int64  `json:"min_confirmations"`
	Enabled          bool   `json:"enabled"`
}

type ChainPolicy struct {
	Chain                 string `json:"chain"`
	Network               string `json:"network"`
	RequiredConfirmations int64  `json:"required_confirmations"`
	SafeDepth             int64  `json:"safe_depth"`
	ReorgWindow           int64  `json:"reorg_window"`
	FeePolicy             string `json:"fee_policy"`
	Enabled               bool   `json:"enabled"`
}

type PrepareAddressInput struct {
	TenantID    string
	AccountID   string
	Model       string
	Chain       string
	Network     string
	SignType    string
	AddressType string
}

type PreparedAddress struct {
	WalletAddress
	Existing bool
}

type AddressRegistryPort interface {
	UpsertAccount(ctx context.Context, in WalletAccount) (WalletAccount, error)
	GetAccount(ctx context.Context, tenantID, accountID string) (WalletAccount, error)
	ListAccounts(ctx context.Context, tenantID string, limit, offset int) ([]WalletAccount, error)
	ListAccountAddresses(ctx context.Context, tenantID, accountID string) ([]WalletAddress, error)
	PrepareAddress(ctx context.Context, in PrepareAddressInput) (PreparedAddress, error)
	GetAccountAddressByKeyID(ctx context.Context, tenantID, accountID, keyID string) (WalletAddress, error)
	GetChainMetadata(ctx context.Context, chain, network string) (ChainMetadata, error)
	GetChainPolicy(ctx context.Context, chain, network string) (ChainPolicy, error)
	UpsertWatchAddress(ctx context.Context, in WatchAddressInput) error
}
