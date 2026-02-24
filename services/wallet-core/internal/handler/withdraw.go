package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type WithdrawHandler struct {
	Orchestrator *orchestrator.WithdrawOrchestrator
	Risk         ports.RiskPort
	Ledger       ports.LedgerPort
	Auth         ports.AuthPort
	Idem         ports.IdempotencyPort
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

	Chain    string `json:"chain"`
	Network  string `json:"network"`
	Coin     string `json:"coin"`
	From     string `json:"from"`
	To       string `json:"to"`
	Amount   string `json:"amount"`
	Base64Tx string `json:"base64_tx"`
	Fee      string `json:"fee"`
	Vin      []vin  `json:"vin"`
	Vout     []vout `json:"vout"`
}

type DepositNotifyRequest struct {
	TenantID      string `json:"tenant_id"`
	AccountID     string `json:"account_id"`
	OrderID       string `json:"order_id"`
	Chain         string `json:"chain"`
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
	Asset             string `json:"asset"`
	Amount            string `json:"amount"`
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
	Risk     ports.RiskDecision `json:"risk"`
	Ledger   ports.LedgerStatus `json:"ledger"`
}

type SweepRunResponse struct {
	Status string `json:"status"`
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
	TenantID    string `json:"tenant_id"`
	AccountID   string `json:"account_id"`
	Chain       string `json:"chain"`
	Coin        string `json:"coin"`
	Network     string `json:"network"`
	Model       string `json:"model"`
	PublicKey   string `json:"public_key"`
	Address     string `json:"address"`
	SignType    string `json:"sign_type"`
	AddressType string `json:"address_type"`
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

func (h *WithdrawHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateWithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.TenantID == "" || req.AccountID == "" || req.OrderID == "" || req.Chain == "" || req.Coin == "" || req.Amount == "" {
		http.Error(w, "tenant_id/account_id/order_id/chain/coin/amount are required", http.StatusBadRequest)
		return
	}
	authScope, requestID, ok := h.authorizeAndTenant(w, r, req.TenantID, "withdraw", true)
	if !ok {
		return
	}
	if !authScope.CanWithdraw {
		http.Error(w, "withdraw not allowed", http.StatusForbidden)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.AccountID, "withdraw") {
		return
	}
	if req.KeyID != "" {
		allowed, err := h.Auth.CheckSignPermission(r.Context(), req.TenantID, req.KeyID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !allowed {
			http.Error(w, "key permission denied", http.StatusForbidden)
			return
		}
	}
	for _, keyID := range req.KeyIDs {
		allowed, err := h.Auth.CheckSignPermission(r.Context(), req.TenantID, keyID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !allowed {
			http.Error(w, "key permission denied", http.StatusForbidden)
			return
		}
	}
	existing, err := h.Ledger.GetWithdrawStatus(r.Context(), req.TenantID, req.OrderID)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err == nil {
		switch existing.Status {
		case "CONFIRMED":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CreateWithdrawResponse{TxHash: existing.TxHash})
			return
		case "FROZEN":
			http.Error(w, "withdraw order is processing", http.StatusConflict)
			return
		default:
			http.Error(w, "withdraw order already exists with status="+existing.Status, http.StatusConflict)
			return
		}
	}

	reqHash := hashRequest(req)
	idem, err := h.Idem.Reserve(r.Context(), req.TenantID, requestID, "withdraw", reqHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch idem.State {
	case "REPLAY":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CreateWithdrawResponse{TxHash: idem.Response})
		return
	case "CONFLICT":
		http.Error(w, "idempotency conflict", http.StatusConflict)
		return
	case "REJECTED":
		http.Error(w, "previous request failed: "+idem.Response, http.StatusConflict)
		return
	}

	tx := ports.BuildUnsignedParams{
		Chain:    req.Chain,
		Network:  req.Network,
		Coin:     req.Coin,
		From:     req.From,
		To:       req.To,
		Amount:   req.Amount,
		Base64Tx: req.Base64Tx,
		Fee:      req.Fee,
		Vin:      make([]ports.TxVin, 0, len(req.Vin)),
		Vout:     make([]ports.TxVout, 0, len(req.Vout)),
	}
	for _, x := range req.Vin {
		tx.Vin = append(tx.Vin, ports.TxVin{Hash: x.Hash, Index: x.Index, Amount: x.Amount, Address: x.Address})
	}
	for _, x := range req.Vout {
		tx.Vout = append(tx.Vout, ports.TxVout{Address: x.Address, Amount: x.Amount, Index: x.Index})
	}

	txHash, err := h.Orchestrator.CreateAndBroadcast(r.Context(), orchestrator.WithdrawRequest{
		TenantID:  req.TenantID,
		AccountID: req.AccountID,
		OrderID:   req.OrderID,
		KeyID:     req.KeyID,
		KeyIDs:    req.KeyIDs,
		SignType:  req.SignType,
		Tx:        tx,
	})
	if err != nil {
		_ = h.Idem.Reject(r.Context(), req.TenantID, requestID, "withdraw", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = h.Idem.Commit(r.Context(), req.TenantID, requestID, "withdraw", txHash)
	_ = h.Auth.Audit(r.Context(), req.TenantID, "withdraw", requestID, "txHash="+txHash)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreateWithdrawResponse{TxHash: txHash})
}

func (h *WithdrawHandler) DepositNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req DepositNotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	authScope, requestID, ok := h.authorizeAndTenant(w, r, req.TenantID, "deposit_notify", true)
	if !ok {
		return
	}
	if !authScope.CanDeposit {
		http.Error(w, "deposit not allowed", http.StatusForbidden)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.AccountID, "deposit_notify") {
		return
	}

	reqHash := hashRequest(req)
	idem, err := h.Idem.Reserve(r.Context(), req.TenantID, requestID, "deposit_notify", reqHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if idem.State == "REPLAY" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	if idem.State == "CONFLICT" {
		http.Error(w, "idempotency conflict", http.StatusConflict)
		return
	}

	err = h.Ledger.CreditDeposit(r.Context(), ports.DepositCreditInput{
		TenantID:      req.TenantID,
		AccountID:     req.AccountID,
		OrderID:       req.OrderID,
		Chain:         req.Chain,
		Coin:          req.Coin,
		Amount:        req.Amount,
		TxHash:        req.TxHash,
		FromAddress:   req.FromAddress,
		ToAddress:     req.ToAddress,
		Confirmations: req.Confirmations,
		RequiredConfs: req.RequiredConfs,
		Status:        req.Status,
	})
	if err != nil {
		_ = h.Idem.Reject(r.Context(), req.TenantID, requestID, "deposit_notify", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = h.Idem.Commit(r.Context(), req.TenantID, requestID, "deposit_notify", "ok")
	_ = h.Auth.Audit(r.Context(), req.TenantID, "deposit_notify", requestID, "txHash="+req.TxHash)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *WithdrawHandler) SweepRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SweepRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	authScope, requestID, ok := h.authorizeAndTenant(w, r, req.TenantID, "sweep_run", true)
	if !ok {
		return
	}
	if !authScope.CanSweep {
		http.Error(w, "sweep not allowed", http.StatusForbidden)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.FromAccountID, "sweep_run") {
		return
	}
	if req.TreasuryAccountID == "" {
		req.TreasuryAccountID = "treasury-main"
	}
	if req.TreasuryAccountID != req.FromAccountID && !h.ensureAccountActive(w, r, req.TenantID, req.TreasuryAccountID, "sweep_run") {
		return
	}
	if req.SweepOrderID == "" {
		req.SweepOrderID = requestID
	}

	reqHash := hashRequest(req)
	idem, err := h.Idem.Reserve(r.Context(), req.TenantID, requestID, "sweep_run", reqHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if idem.State == "REPLAY" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SweepRunResponse{Status: "DONE"})
		return
	}
	if idem.State == "CONFLICT" {
		http.Error(w, "idempotency conflict", http.StatusConflict)
		return
	}

	err = h.Ledger.CollectSweep(r.Context(), ports.SweepCollectInput{
		TenantID:          req.TenantID,
		FromAccountID:     req.FromAccountID,
		TreasuryAccountID: req.TreasuryAccountID,
		SweepOrderID:      req.SweepOrderID,
		Asset:             req.Asset,
		Amount:            req.Amount,
	})
	if err != nil {
		_ = h.Idem.Reject(r.Context(), req.TenantID, requestID, "sweep_run", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = h.Idem.Commit(r.Context(), req.TenantID, requestID, "sweep_run", "DONE")
	_ = h.Auth.Audit(r.Context(), req.TenantID, "sweep_run", requestID, "sweepOrder="+req.SweepOrderID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SweepRunResponse{Status: "DONE"})
}

func (h *WithdrawHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	orderID := r.URL.Query().Get("order_id")
	if tenantID == "" || orderID == "" {
		http.Error(w, "tenant_id and order_id are required", http.StatusBadRequest)
		return
	}
	_, _, ok := h.authorizeAndTenant(w, r, tenantID, "withdraw_status", false)
	if !ok {
		return
	}

	riskDecision, riskErr := h.Risk.GetWithdrawDecision(r.Context(), tenantID, orderID)
	if riskErr != nil && riskErr != sql.ErrNoRows {
		http.Error(w, riskErr.Error(), http.StatusInternalServerError)
		return
	}
	ledgerStatus, ledgerErr := h.Ledger.GetWithdrawStatus(r.Context(), tenantID, orderID)
	if ledgerErr != nil && ledgerErr != sql.ErrNoRows {
		http.Error(w, ledgerErr.Error(), http.StatusInternalServerError)
		return
	}
	if ledgerErr == sql.ErrNoRows {
		http.Error(w, "withdraw order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(WithdrawStatusResponse{
		TenantID: tenantID,
		OrderID:  orderID,
		Risk:     riskDecision,
		Ledger:   ledgerStatus,
	})
}

func (h *WithdrawHandler) Balance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	accountID := r.URL.Query().Get("account_id")
	asset := r.URL.Query().Get("asset")
	if tenantID == "" || accountID == "" || asset == "" {
		http.Error(w, "tenant_id, account_id and asset are required", http.StatusBadRequest)
		return
	}
	_, _, ok := h.authorizeAndTenant(w, r, tenantID, "balance", false)
	if !ok {
		return
	}
	bal, err := h.Ledger.GetBalance(r.Context(), tenantID, accountID, asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BalanceResponse{
		TenantID:  tenantID,
		AccountID: accountID,
		Asset:     asset,
		Available: bal.Available,
		Frozen:    bal.Frozen,
	})
}

func (h *WithdrawHandler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.KeyManager == nil || h.ChainAddr == nil || h.Registry == nil {
		http.Error(w, "address create not enabled", http.StatusNotImplemented)
		return
	}
	var req CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.TenantID == "" || req.AccountID == "" || req.Chain == "" || req.Coin == "" {
		http.Error(w, "tenant_id/account_id/chain/coin are required", http.StatusBadRequest)
		return
	}
	scope, requestID, ok := h.authorizeAndTenant(w, r, req.TenantID, "address_create", true)
	if !ok {
		return
	}
	if !scope.CanDeposit {
		http.Error(w, "address create not allowed", http.StatusForbidden)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.AccountID, "address_create") {
		return
	}
	if req.Network == "" {
		req.Network = "mainnet"
	}
	if req.SignType == "" {
		req.SignType = "ecdsa"
	}
	if req.Model == "" {
		req.Model = inferModel(req.Chain)
	}
	if req.MinConfirmations <= 0 {
		req.MinConfirmations = 1
	}
	autoSweep := false
	if req.AutoSweep != nil {
		autoSweep = *req.AutoSweep
	}
	if req.TreasuryAccountID == "" {
		req.TreasuryAccountID = "treasury-main"
	}

	keys, err := h.KeyManager.ExportPublicKeys(r.Context(), req.SignType, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(keys) == 0 || keys[0].CompressPubkey == "" {
		http.Error(w, "empty public key", http.StatusBadGateway)
		return
	}
	pubKey := keys[0].CompressPubkey

	address, err := h.ChainAddr.ConvertAddress(r.Context(), req.Chain, req.AddressType, pubKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := h.Auth.BindTenantKey(r.Context(), req.TenantID, pubKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.Registry.UpsertWatchAddress(r.Context(), ports.WatchAddressInput{
		TenantID:          req.TenantID,
		AccountID:         req.AccountID,
		Model:             req.Model,
		Chain:             req.Chain,
		Coin:              req.Coin,
		Network:           req.Network,
		Address:           address,
		PublicKey:         pubKey,
		SignType:          req.SignType,
		MinConfirmations:  req.MinConfirmations,
		TreasuryAccountID: req.TreasuryAccountID,
		AutoSweep:         autoSweep,
		SweepThreshold:    req.SweepThreshold,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = h.Auth.Audit(r.Context(), req.TenantID, "address_create", requestID, "account="+req.AccountID+" address="+address)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreateAddressResponse{
		TenantID:    req.TenantID,
		AccountID:   req.AccountID,
		Chain:       req.Chain,
		Coin:        req.Coin,
		Network:     req.Network,
		Model:       req.Model,
		PublicKey:   pubKey,
		Address:     address,
		SignType:    req.SignType,
		AddressType: req.AddressType,
	})
}

func (h *WithdrawHandler) AccountUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Registry == nil {
		http.Error(w, "account registry not enabled", http.StatusNotImplemented)
		return
	}
	var req AccountUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.TenantID == "" || req.AccountID == "" {
		http.Error(w, "tenant_id/account_id are required", http.StatusBadRequest)
		return
	}
	scope, requestID, ok := h.authorizeAndTenant(w, r, req.TenantID, "account_upsert", true)
	if !ok {
		return
	}
	if !scope.CanDeposit {
		http.Error(w, "account upsert not allowed", http.StatusForbidden)
		return
	}
	out, err := h.Registry.UpsertAccount(r.Context(), ports.WalletAccount{
		TenantID:   req.TenantID,
		AccountID:  req.AccountID,
		AccountTag: req.AccountTag,
		Status:     req.Status,
		Remark:     req.Remark,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = h.Auth.Audit(r.Context(), req.TenantID, "account_upsert", requestID, "account="+req.AccountID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AccountResponse{
		TenantID:   out.TenantID,
		AccountID:  out.AccountID,
		AccountTag: out.AccountTag,
		Status:     out.Status,
		Remark:     out.Remark,
		CreatedAt:  out.CreatedAt,
		UpdatedAt:  out.UpdatedAt,
	})
}

func (h *WithdrawHandler) AccountGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	accountID := r.URL.Query().Get("account_id")
	if tenantID == "" || accountID == "" {
		http.Error(w, "tenant_id/account_id are required", http.StatusBadRequest)
		return
	}
	if _, _, ok := h.authorizeAndTenant(w, r, tenantID, "account_get", false); !ok {
		return
	}
	out, err := h.Registry.GetAccount(r.Context(), tenantID, accountID)
	if err == sql.ErrNoRows {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AccountResponse{
		TenantID:   out.TenantID,
		AccountID:  out.AccountID,
		AccountTag: out.AccountTag,
		Status:     out.Status,
		Remark:     out.Remark,
		CreatedAt:  out.CreatedAt,
		UpdatedAt:  out.UpdatedAt,
	})
}

func (h *WithdrawHandler) AccountList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		http.Error(w, "tenant_id is required", http.StatusBadRequest)
		return
	}
	if _, _, ok := h.authorizeAndTenant(w, r, tenantID, "account_list", false); !ok {
		return
	}
	items, err := h.Registry.ListAccounts(r.Context(), tenantID, 100, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := AccountListResponse{Items: make([]AccountResponse, 0, len(items))}
	for _, x := range items {
		resp.Items = append(resp.Items, AccountResponse{
			TenantID:   x.TenantID,
			AccountID:  x.AccountID,
			AccountTag: x.AccountTag,
			Status:     x.Status,
			Remark:     x.Remark,
			CreatedAt:  x.CreatedAt,
			UpdatedAt:  x.UpdatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *WithdrawHandler) AccountAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	accountID := r.URL.Query().Get("account_id")
	if tenantID == "" || accountID == "" {
		http.Error(w, "tenant_id/account_id are required", http.StatusBadRequest)
		return
	}
	if _, _, ok := h.authorizeAndTenant(w, r, tenantID, "account_addresses", false); !ok {
		return
	}
	items, err := h.Registry.ListAccountAddresses(r.Context(), tenantID, accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AccountAddressesResponse{Items: items})
}

func (h *WithdrawHandler) AccountAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	accountID := r.URL.Query().Get("account_id")
	if tenantID == "" || accountID == "" {
		http.Error(w, "tenant_id/account_id are required", http.StatusBadRequest)
		return
	}
	if _, _, ok := h.authorizeAndTenant(w, r, tenantID, "account_assets", false); !ok {
		return
	}
	items, err := h.Ledger.ListAccountAssets(r.Context(), tenantID, accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AccountAssetsResponse{Items: items})
}

func (h *WithdrawHandler) ensureAccountActive(w http.ResponseWriter, r *http.Request, tenantID, accountID, action string) bool {
	if h.Registry == nil {
		return true
	}
	acc, err := h.Registry.GetAccount(r.Context(), tenantID, accountID)
	if err == sql.ErrNoRows {
		http.Error(w, "account not found", http.StatusNotFound)
		return false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	if strings.ToUpper(strings.TrimSpace(acc.Status)) != "ACTIVE" {
		http.Error(w, "account is not active for "+action, http.StatusForbidden)
		return false
	}
	return true
}

func (h *WithdrawHandler) authorizeAndTenant(w http.ResponseWriter, r *http.Request, tenantID, action string, requireRequestID bool) (ports.AuthScope, string, bool) {
	requestID := requestIDFrom(r, "")
	if requireRequestID && requestID == "" {
		http.Error(w, "X-Request-ID is required", http.StatusBadRequest)
		return ports.AuthScope{}, "", false
	}
	if requestID == "" {
		requestID = action
	}
	token := bearerToken(r.Header.Get("Authorization"))
	scope, err := h.Auth.ValidateToken(r.Context(), token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return ports.AuthScope{}, requestID, false
	}
	if scope.TenantID != "*" && scope.TenantID != tenantID {
		http.Error(w, "tenant mismatch", http.StatusForbidden)
		return ports.AuthScope{}, requestID, false
	}
	return scope, requestID, true
}

func requestIDFrom(r *http.Request, fallback string) string {
	v := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if v != "" {
		return v
	}
	return fallback
}

func bearerToken(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	parts := strings.SplitN(v, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return v
}

func hashRequest(v interface{}) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func inferModel(chain string) string {
	c := strings.ToLower(strings.TrimSpace(chain))
	switch c {
	case "bitcoin", "btc", "litecoin", "ltc", "dogecoin", "doge", "dash", "bitcoincash", "bch", "zen":
		return "utxo"
	default:
		return "account"
	}
}
