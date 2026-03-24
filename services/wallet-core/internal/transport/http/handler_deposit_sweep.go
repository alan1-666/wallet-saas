package httptransport

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func (h *WithdrawHandler) DepositNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req DepositNotifyRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.AccountID == "" || req.OrderID == "" || req.Chain == "" || req.Network == "" || req.Coin == "" || req.Amount == "" {
		http.Error(w, "tenant_id/account_id/order_id/chain/network/coin/amount are required", http.StatusBadRequest)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.AccountID, "deposit_notify") {
		return
	}

	err := h.Ledger.CreditDeposit(r.Context(), ports.DepositCreditInput{
		TenantID:      req.TenantID,
		AccountID:     req.AccountID,
		OrderID:       req.OrderID,
		Chain:         req.Chain,
		Network:       req.Network,
		Coin:          req.Coin,
		Amount:        req.Amount,
		TxHash:        req.TxHash,
		FromAddress:   req.FromAddress,
		ToAddress:     req.ToAddress,
		Confirmations: req.Confirmations,
		RequiredConfs: req.RequiredConfs,
		UnlockConfs:   req.UnlockConfs,
		ScanStatus:    req.ScanStatus,
		Status:        req.Status,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *WithdrawHandler) SweepRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Registry == nil {
		http.Error(w, "sweep run not enabled", http.StatusNotImplemented)
		return
	}
	var req SweepRunRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.FromAccountID, "sweep_run") {
		return
	}
	if req.TreasuryAccountID == "" {
		req.TreasuryAccountID = "treasury-main"
	}
	if req.Chain == "" || req.Network == "" || req.Asset == "" || req.Amount == "" {
		http.Error(w, "chain/network/asset/amount are required", http.StatusBadRequest)
		return
	}
	if req.TreasuryAccountID != req.FromAccountID && !h.ensureAccountActive(w, r, req.TenantID, req.TreasuryAccountID, "sweep_run") {
		return
	}
	if strings.TrimSpace(req.ColdAccountID) != "" && req.ColdAccountID != req.FromAccountID && req.ColdAccountID != req.TreasuryAccountID {
		if !h.ensureAccountActive(w, r, req.TenantID, req.ColdAccountID, "sweep_run") {
			return
		}
	}
	requiredConfs := int64(1)
	if h.Registry != nil {
		policy, err := h.Registry.GetChainPolicy(r.Context(), req.Chain, req.Network)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !policy.Enabled {
			http.Error(w, "chain/network policy disabled", http.StatusBadRequest)
			return
		}
		if policy.RequiredConfirmations > 0 {
			requiredConfs = policy.RequiredConfirmations
		}
	}
	if req.SweepOrderID == "" {
		requestID := requestIDFrom(r, "sweep_run")
		req.SweepOrderID = requestID
	}
	var err error
	fromAddr, err := h.findAccountAddress(r.Context(), req.TenantID, req.FromAccountID, req.Chain, req.Network, req.Asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	destinationAccountID, destinationTier, err := h.resolveSweepDestinationAccount(r, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	treasuryAddr, err := h.findAccountAddress(r.Context(), req.TenantID, destinationAccountID, req.Chain, req.Network, req.Asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.Ledger.ReserveSweep(r.Context(), ports.SweepReserveInput{
		TenantID:          req.TenantID,
		FromAccountID:     req.FromAccountID,
		TreasuryAccountID: destinationAccountID,
		SweepOrderID:      req.SweepOrderID,
		Chain:             req.Chain,
		Network:           req.Network,
		Asset:             req.Asset,
		Amount:            req.Amount,
		RequiredConfs:     requiredConfs,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	broadcast, err := h.Orchestrator.BroadcastOnly(r.Context(), orchestrator.WithdrawRequest{
		TenantID:      req.TenantID,
		AccountID:     req.FromAccountID,
		OrderID:       req.SweepOrderID,
		RequiredConfs: requiredConfs,
		Signers: []ports.SignerRef{{
			KeyID:     fromAddr.KeyID,
			PublicKey: fromAddr.PublicKey,
		}},
		SignType: fromAddr.SignType,
		Tx: ports.BuildUnsignedParams{
			Chain:   req.Chain,
			Network: req.Network,
			Coin:    req.Asset,
			From:    fromAddr.Address,
			To:      treasuryAddr.Address,
			Amount:  req.Amount,
		},
	})
	if err != nil {
		_ = h.Ledger.FailSweepOnChain(r.Context(), req.TenantID, req.SweepOrderID, err.Error(), 0)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = retryLedgerMutation(3, 200*time.Millisecond, func() error {
		return h.Ledger.StartSweep(r.Context(), ports.SweepCollectInput{
			TenantID:          req.TenantID,
			FromAccountID:     req.FromAccountID,
			TreasuryAccountID: destinationAccountID,
			SweepOrderID:      req.SweepOrderID,
			Chain:             req.Chain,
			Network:           req.Network,
			Asset:             req.Asset,
			Amount:            req.Amount,
			TxHash:            broadcast.TxHash,
			RequiredConfs:     requiredConfs,
		})
	})
	if err != nil {
		http.Error(w, "sweep broadcasted but ledger mark failed, tx_hash="+broadcast.TxHash+": "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SweepRunResponse{Status: "BROADCASTED", TxHash: broadcast.TxHash, DestinationAccountID: destinationAccountID, DestinationTier: destinationTier})
}

func (h *WithdrawHandler) SweepOnchainNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SweepOnchainNotifyRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.SweepOrderID == "" || req.TxHash == "" {
		http.Error(w, "tenant_id/sweep_order_id/tx_hash are required", http.StatusBadRequest)
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "CONFIRMED"
	}
	var err error
	switch status {
	case "CONFIRMED":
		err = h.Ledger.ConfirmSweepOnChain(r.Context(), ports.SweepConfirmInput{
			TenantID:      req.TenantID,
			SweepOrderID:  req.SweepOrderID,
			TxHash:        req.TxHash,
			Confirmations: req.Confirmations,
			RequiredConfs: req.RequiredConfs,
		})
	case "FAILED", "REVERTED":
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			reason = "onchain failed"
		}
		err = h.Ledger.FailSweepOnChain(r.Context(), req.TenantID, req.SweepOrderID, reason, req.Confirmations)
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *WithdrawHandler) resolveSweepDestinationAccount(r *http.Request, req SweepRunRequest) (string, string, error) {
	hotAccountID := strings.TrimSpace(req.TreasuryAccountID)
	if hotAccountID == "" {
		hotAccountID = "treasury-main"
	}
	coldAccountID := strings.TrimSpace(req.ColdAccountID)
	hotBalanceCap := strings.TrimSpace(req.HotBalanceCap)
	if coldAccountID == "" || hotBalanceCap == "" || hotBalanceCap == "0" || h.Ledger == nil {
		return hotAccountID, "HOT", nil
	}

	currentHotVault, err := h.getVaultAvailable(r, req.TenantID, hotAccountID, req.Asset)
	if err != nil {
		return "", "", err
	}
	projectedHotVault, err := addIntegerStrings(currentHotVault, req.Amount)
	if err != nil {
		return "", "", err
	}
	if compareIntegerStrings(projectedHotVault, hotBalanceCap) > 0 {
		return coldAccountID, "COLD", nil
	}
	return hotAccountID, "HOT", nil
}

func (h *WithdrawHandler) getVaultAvailable(r *http.Request, tenantID, accountID, asset string) (string, error) {
	items, err := h.Ledger.ListAccountAssets(r.Context(), tenantID, accountID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Asset), strings.TrimSpace(asset)) {
			if strings.TrimSpace(item.VaultAvailable) == "" {
				return "0", nil
			}
			return strings.TrimSpace(item.VaultAvailable), nil
		}
	}
	return "0", nil
}

func addIntegerStrings(a, b string) (string, error) {
	aa, ok := new(big.Int).SetString(strings.TrimSpace(a), 10)
	if !ok {
		return "", fmt.Errorf("invalid integer amount: %s", a)
	}
	bb, ok := new(big.Int).SetString(strings.TrimSpace(b), 10)
	if !ok {
		return "", fmt.Errorf("invalid integer amount: %s", b)
	}
	if aa.Sign() < 0 || bb.Sign() < 0 {
		return "", fmt.Errorf("negative amount not allowed")
	}
	return new(big.Int).Add(aa, bb).String(), nil
}

func subIntegerStrings(a, b string) (string, error) {
	aa, ok := new(big.Int).SetString(strings.TrimSpace(a), 10)
	if !ok {
		return "", fmt.Errorf("invalid integer amount: %s", a)
	}
	bb, ok := new(big.Int).SetString(strings.TrimSpace(b), 10)
	if !ok {
		return "", fmt.Errorf("invalid integer amount: %s", b)
	}
	if aa.Sign() < 0 || bb.Sign() < 0 {
		return "", fmt.Errorf("negative amount not allowed")
	}
	if aa.Cmp(bb) < 0 {
		return "", fmt.Errorf("insufficient balance")
	}
	return new(big.Int).Sub(aa, bb).String(), nil
}

func compareIntegerStrings(a, b string) int {
	aa, ok := new(big.Int).SetString(strings.TrimSpace(a), 10)
	if !ok {
		return 0
	}
	bb, ok := new(big.Int).SetString(strings.TrimSpace(b), 10)
	if !ok {
		return 0
	}
	return aa.Cmp(bb)
}
