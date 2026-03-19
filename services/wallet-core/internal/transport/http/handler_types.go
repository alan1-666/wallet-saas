package httptransport

import (
	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type WithdrawHandler struct {
	Orchestrator *orchestrator.WithdrawOrchestrator
	Ledger       ports.LedgerPort
	Auth         ports.AuthPort
	KeyManager   ports.KeyManagePort
	ChainAddr    ports.ChainAddressPort
	Registry     ports.AddressRegistryPort
}

type CreateWithdrawRequest struct {
	TenantID  string   `json:"tenant_id"`
	AccountID string   `json:"account_id"`
	OrderID   string   `json:"order_id"`
	KeyID     string   `json:"key_id"`
	KeyIDs    []string `json:"key_ids"`
	SignType  string   `json:"sign_type"`

	Chain           string `json:"chain"`
	Network         string `json:"network"`
	Coin            string `json:"coin"`
	From            string `json:"from"`
	To              string `json:"to"`
	Amount          string `json:"amount"`
	ContractAddress string `json:"contract_address"`
	AmountUnit      string `json:"amount_unit"`
	TokenDecimals   uint32 `json:"token_decimals"`
	Base64Tx        string `json:"base64_tx"`
	Fee             string `json:"fee"`
	Vin             []vin  `json:"vin"`
	Vout            []vout `json:"vout"`
}

type DepositNotifyRequest struct {
	TenantID      string `json:"tenant_id"`
	AccountID     string `json:"account_id"`
	OrderID       string `json:"order_id"`
	Chain         string `json:"chain"`
	Network       string `json:"network"`
	Coin          string `json:"coin"`
	Amount        string `json:"amount"`
	TxHash        string `json:"tx_hash"`
	FromAddress   string `json:"from_address"`
	ToAddress     string `json:"to_address"`
	Confirmations int64  `json:"confirmations"`
	RequiredConfs int64  `json:"required_confirmations"`
	Status        string `json:"status"`
}

type SweepRunRequest struct {
	TenantID          string `json:"tenant_id"`
	SweepOrderID      string `json:"sweep_order_id"`
	FromAccountID     string `json:"from_account_id"`
	TreasuryAccountID string `json:"treasury_account_id"`
	Chain             string `json:"chain"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
	Amount            string `json:"amount"`
}

type WithdrawOnchainNotifyRequest struct {
	TenantID      string `json:"tenant_id"`
	OrderID       string `json:"order_id"`
	TxHash        string `json:"tx_hash"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
	Confirmations int64  `json:"confirmations"`
	RequiredConfs int64  `json:"required_confirmations"`
}

type SweepOnchainNotifyRequest struct {
	TenantID      string `json:"tenant_id"`
	SweepOrderID  string `json:"sweep_order_id"`
	TxHash        string `json:"tx_hash"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
	Confirmations int64  `json:"confirmations"`
	RequiredConfs int64  `json:"required_confirmations"`
}

type vin struct {
	Hash    string `json:"hash"`
	Index   uint32 `json:"index"`
	Amount  int64  `json:"amount"`
	Address string `json:"address"`
}

type vout struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
	Index   uint32 `json:"index"`
}

type CreateWithdrawResponse struct {
	TxHash string `json:"tx_hash"`
}

type WithdrawStatusResponse struct {
	TenantID string             `json:"tenant_id"`
	OrderID  string             `json:"order_id"`
	Ledger   ports.LedgerStatus `json:"ledger"`
}

type SweepRunResponse struct {
	Status string `json:"status"`
	TxHash string `json:"tx_hash,omitempty"`
}

type BalanceResponse struct {
	TenantID  string `json:"tenant_id"`
	AccountID string `json:"account_id"`
	Asset     string `json:"asset"`
	Available string `json:"available"`
	Frozen    string `json:"frozen"`
}

type CreateAddressRequest struct {
	TenantID          string `json:"tenant_id"`
	AccountID         string `json:"account_id"`
	Chain             string `json:"chain"`
	Coin              string `json:"coin"`
	Network           string `json:"network"`
	AddressType       string `json:"address_type"`
	SignType          string `json:"sign_type"`
	MinConfirmations  int64  `json:"min_confirmations"`
	TreasuryAccountID string `json:"treasury_account_id"`
	AutoSweep         *bool  `json:"auto_sweep"`
	SweepThreshold    string `json:"sweep_threshold"`
	Model             string `json:"model"`
}

type CreateAddressResponse struct {
	TenantID       string `json:"tenant_id"`
	AccountID      string `json:"account_id"`
	Chain          string `json:"chain"`
	Coin           string `json:"coin"`
	Network        string `json:"network"`
	Model          string `json:"model"`
	KeyID          string `json:"key_id"`
	PublicKey      string `json:"public_key"`
	Address        string `json:"address"`
	SignType       string `json:"sign_type"`
	AddressType    string `json:"address_type"`
	DerivationPath string `json:"derivation_path"`
	ChangeIndex    int64  `json:"change_index"`
	AddressIndex   int64  `json:"address_index"`
}

type AccountUpsertRequest struct {
	TenantID   string `json:"tenant_id"`
	AccountID  string `json:"account_id"`
	AccountTag string `json:"account_tag"`
	Status     string `json:"status"`
	Remark     string `json:"remark"`
}

type AccountResponse struct {
	TenantID   string `json:"tenant_id"`
	AccountID  string `json:"account_id"`
	AccountTag string `json:"account_tag"`
	Status     string `json:"status"`
	Remark     string `json:"remark"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type AccountListResponse struct {
	Items []AccountResponse `json:"items"`
}

type AccountAddressesResponse struct {
	Items []ports.WalletAddress `json:"items"`
}

type AccountAssetsResponse struct {
	Items []ports.AccountAsset `json:"items"`
}
