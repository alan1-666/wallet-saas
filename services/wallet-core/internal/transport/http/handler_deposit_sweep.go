package httptransport

import (
	"encoding/json"
	"net/http"
	"strings"

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
	treasuryAddr, err := h.findAccountAddress(r.Context(), req.TenantID, req.TreasuryAccountID, req.Chain, req.Network, req.Asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	txHash, err := h.Orchestrator.CreateAndBroadcast(r.Context(), orchestrator.WithdrawRequest{
		TenantID:      req.TenantID,
		AccountID:     req.FromAccountID,
		OrderID:       req.SweepOrderID,
		RequiredConfs: requiredConfs,
		KeyID:         fromAddr.PublicKey,
		SignType:      fromAddr.SignType,
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.Ledger.StartSweep(r.Context(), ports.SweepCollectInput{
		TenantID:          req.TenantID,
		FromAccountID:     req.FromAccountID,
		TreasuryAccountID: req.TreasuryAccountID,
		SweepOrderID:      req.SweepOrderID,
		Chain:             req.Chain,
		Network:           req.Network,
		Asset:             req.Asset,
		Amount:            req.Amount,
		TxHash:            txHash,
		RequiredConfs:     requiredConfs,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SweepRunResponse{Status: "BROADCASTED", TxHash: txHash})
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
