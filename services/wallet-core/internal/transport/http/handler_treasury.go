package httptransport

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func (h *WithdrawHandler) TreasuryTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Registry == nil {
		http.Error(w, "treasury transfer not enabled", http.StatusNotImplemented)
		return
	}
	var req TreasuryTransferRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.FromAccountID == "" || req.ToAccountID == "" || req.Chain == "" || req.Network == "" || req.Asset == "" || req.Amount == "" {
		http.Error(w, "tenant_id/from_account_id/to_account_id/chain/network/asset/amount are required", http.StatusBadRequest)
		return
	}
	if req.FromAccountID == req.ToAccountID {
		http.Error(w, "from_account_id and to_account_id must differ", http.StatusBadRequest)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.FromAccountID, "treasury_transfer") {
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.ToAccountID, "treasury_transfer") {
		return
	}
	if strings.TrimSpace(req.TransferOrderID) == "" {
		req.TransferOrderID = requestIDFrom(r, "treasury_transfer")
	}

	existing, err := h.Ledger.GetTreasuryTransferStatus(r.Context(), req.TenantID, req.TransferOrderID)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TreasuryTransferResponse{
			Status:          existing.Status,
			TxHash:          existing.TxHash,
			TransferOrderID: req.TransferOrderID,
			SourceTier:      existing.SourceTier,
			DestinationTier: existing.DestinationTier,
		})
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

	fromAddr, err := h.findAccountAddress(r.Context(), req.TenantID, req.FromAccountID, req.Chain, req.Network, req.Asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	toAddr, err := h.findAccountAddress(r.Context(), req.TenantID, req.ToAccountID, req.Chain, req.Network, req.Asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sourceTier := normalizeTreasuryTier(req.SourceTier, req.FromAccountID, "HOT")
	destinationTier := normalizeTreasuryTier(req.DestinationTier, req.ToAccountID, "COLD")
	if err := h.Ledger.ReserveTreasuryTransfer(r.Context(), ports.TreasuryTransferReserveInput{
		TenantID:              req.TenantID,
		TransferOrderID:       req.TransferOrderID,
		FromAccountID:         req.FromAccountID,
		ToAccountID:           req.ToAccountID,
		Chain:                 req.Chain,
		Network:               req.Network,
		Asset:                 req.Asset,
		Amount:                req.Amount,
		RequiredConfirmations: requiredConfs,
		SourceTier:            sourceTier,
		DestinationTier:       destinationTier,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	broadcast, err := h.Orchestrator.BroadcastOnly(r.Context(), orchestrator.WithdrawRequest{
		TenantID:  req.TenantID,
		AccountID: req.FromAccountID,
		OrderID:   req.TransferOrderID,
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
			To:      toAddr.Address,
			Amount:  req.Amount,
		},
	})
	if err != nil {
		_ = h.Ledger.FailTreasuryTransferOnChain(r.Context(), req.TenantID, req.TransferOrderID, err.Error(), 0)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := retryLedgerMutation(3, 200*time.Millisecond, func() error {
		return h.Ledger.MarkTreasuryTransferBroadcasted(r.Context(), req.TenantID, req.TransferOrderID, broadcast.TxHash, requiredConfs)
	}); err != nil {
		http.Error(w, "treasury transfer broadcasted but ledger mark failed, tx_hash="+broadcast.TxHash+": "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TreasuryTransferResponse{
		Status:          "BROADCASTED",
		TxHash:          broadcast.TxHash,
		TransferOrderID: req.TransferOrderID,
		SourceTier:      sourceTier,
		DestinationTier: destinationTier,
	})
}

func (h *WithdrawHandler) TreasuryTransferOnchainNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req TreasuryTransferOnchainNotifyRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.TransferOrderID == "" || req.TxHash == "" {
		http.Error(w, "tenant_id/transfer_order_id/tx_hash are required", http.StatusBadRequest)
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "CONFIRMED"
	}
	var err error
	switch status {
	case "CONFIRMED":
		err = h.Ledger.ConfirmTreasuryTransferOnChain(r.Context(), ports.TreasuryTransferConfirmInput{
			TenantID:        req.TenantID,
			TransferOrderID: req.TransferOrderID,
			TxHash:          req.TxHash,
			Confirmations:   req.Confirmations,
			RequiredConfs:   req.RequiredConfs,
		})
	case "FAILED", "REVERTED":
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			reason = "onchain failed"
		}
		err = h.Ledger.FailTreasuryTransferOnChain(r.Context(), req.TenantID, req.TransferOrderID, reason, req.Confirmations)
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

func (h *WithdrawHandler) TreasuryTransferStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	transferOrderID := r.URL.Query().Get("transfer_order_id")
	if tenantID == "" || transferOrderID == "" {
		http.Error(w, "tenant_id and transfer_order_id are required", http.StatusBadRequest)
		return
	}
	status, err := h.Ledger.GetTreasuryTransferStatus(r.Context(), tenantID, transferOrderID)
	if err == sql.ErrNoRows {
		http.Error(w, "treasury transfer order not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TreasuryTransferStatusResponse{
		TenantID:        tenantID,
		TransferOrderID: transferOrderID,
		Transfer:        status,
	})
}

func (h *WithdrawHandler) TreasuryWaterline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	hotAccountID := strings.TrimSpace(r.URL.Query().Get("hot_account_id"))
	coldAccountID := strings.TrimSpace(r.URL.Query().Get("cold_account_id"))
	asset := strings.TrimSpace(r.URL.Query().Get("asset"))
	hotBalanceCap := strings.TrimSpace(r.URL.Query().Get("hot_balance_cap"))
	hotBalanceFloor := strings.TrimSpace(r.URL.Query().Get("hot_balance_floor"))
	if tenantID == "" || hotAccountID == "" || coldAccountID == "" || asset == "" {
		http.Error(w, "tenant_id/hot_account_id/cold_account_id/asset are required", http.StatusBadRequest)
		return
	}
	if hotBalanceCap == "" && hotBalanceFloor == "" {
		http.Error(w, "hot_balance_cap or hot_balance_floor is required", http.StatusBadRequest)
		return
	}
	if hotBalanceCap == "" {
		hotBalanceCap = "0"
	}
	if hotBalanceFloor == "" {
		hotBalanceFloor = "0"
	}
	if compareIntegerStrings(hotBalanceCap, "0") > 0 && compareIntegerStrings(hotBalanceFloor, hotBalanceCap) > 0 {
		http.Error(w, "hot_balance_floor cannot exceed hot_balance_cap", http.StatusBadRequest)
		return
	}
	hotVault, err := h.getVaultAvailable(r, tenantID, hotAccountID, asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	coldVault, err := h.getVaultAvailable(r, tenantID, coldAccountID, asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state := "NORMAL"
	recommendedAction := "NONE"
	suggestedTransferAmount := "0"
	if compareIntegerStrings(hotBalanceFloor, "0") > 0 && compareIntegerStrings(hotVault, hotBalanceFloor) < 0 {
		state = "LOW"
		recommendedAction = "COLD_TO_HOT"
		suggestedTransferAmount, err = subIntegerStrings(hotBalanceFloor, hotVault)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if compareIntegerStrings(hotBalanceCap, "0") > 0 && compareIntegerStrings(hotVault, hotBalanceCap) > 0 {
		state = "HIGH"
		recommendedAction = "HOT_TO_COLD"
		suggestedTransferAmount, err = subIntegerStrings(hotVault, hotBalanceCap)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TreasuryWaterlineResponse{
		TenantID:                tenantID,
		Asset:                   asset,
		HotAccountID:            hotAccountID,
		ColdAccountID:           coldAccountID,
		HotVaultAvailable:       hotVault,
		ColdVaultAvailable:      coldVault,
		HotBalanceCap:           hotBalanceCap,
		HotBalanceFloor:         hotBalanceFloor,
		State:                   state,
		RecommendedAction:       recommendedAction,
		SuggestedTransferAmount: suggestedTransferAmount,
	})
}

func normalizeTreasuryTier(requestTier, accountID, fallback string) string {
	tier := strings.ToUpper(strings.TrimSpace(requestTier))
	if tier == "HOT" || tier == "COLD" {
		return tier
	}
	accountID = strings.ToLower(strings.TrimSpace(accountID))
	if strings.Contains(accountID, "cold") {
		return "COLD"
	}
	if strings.Contains(accountID, "hot") {
		return "HOT"
	}
	return strings.ToUpper(strings.TrimSpace(fallback))
}
