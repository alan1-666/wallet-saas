package httptransport

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func (h *WithdrawHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateWithdrawRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.AccountID == "" || req.OrderID == "" || req.Chain == "" || req.Network == "" || req.Coin == "" || req.Amount == "" {
		http.Error(w, "tenant_id/account_id/order_id/chain/network/coin/amount are required", http.StatusBadRequest)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.AccountID, "withdraw") {
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
		case "BROADCASTED":
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
		TenantID:      req.TenantID,
		AccountID:     req.AccountID,
		OrderID:       req.OrderID,
		RequiredConfs: requiredConfs,
		KeyID:         req.KeyID,
		KeyIDs:        req.KeyIDs,
		SignType:      req.SignType,
		Tx:            tx,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreateWithdrawResponse{TxHash: txHash})
}

func (h *WithdrawHandler) WithdrawOnchainNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req WithdrawOnchainNotifyRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.OrderID == "" || req.TxHash == "" {
		http.Error(w, "tenant_id/order_id/tx_hash are required", http.StatusBadRequest)
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "CONFIRMED"
	}
	var err error
	switch status {
	case "CONFIRMED":
		err = h.Ledger.ConfirmWithdrawOnChain(r.Context(), req.TenantID, req.OrderID, req.TxHash, req.Confirmations, req.RequiredConfs)
	case "FAILED", "REVERTED":
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			reason = "onchain failed"
		}
		err = h.Ledger.FailWithdrawOnChain(r.Context(), req.TenantID, req.OrderID, reason, req.Confirmations)
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
