package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.AccountID == "" || req.Chain == "" || req.Coin == "" {
		http.Error(w, "tenant_id/account_id/chain/coin are required", http.StatusBadRequest)
		return
	}
	if !h.ensureAccountActive(w, r, req.TenantID, req.AccountID, "address_create") {
		return
	}
	if req.Network == "" {
		http.Error(w, "network is required", http.StatusBadRequest)
		return
	}
	meta, err := h.Registry.GetChainMetadata(r.Context(), req.Chain, req.Network)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !meta.Enabled {
		http.Error(w, "chain/network disabled", http.StatusBadRequest)
		return
	}
	if req.SignType == "" {
		req.SignType = "ecdsa"
	}
	if req.Model == "" {
		req.Model = strings.ToLower(strings.TrimSpace(meta.Model))
	}
	if strings.ToLower(strings.TrimSpace(req.Model)) != strings.ToLower(strings.TrimSpace(meta.Model)) {
		http.Error(w, "model mismatch for chain/network", http.StatusBadRequest)
		return
	}
	req.Chain = strings.ToLower(strings.TrimSpace(meta.Chain))
	req.Network = strings.ToLower(strings.TrimSpace(req.Network))
	req.Model = strings.ToLower(strings.TrimSpace(meta.Model))
	req.Coin = strings.ToUpper(strings.TrimSpace(req.Coin))
	policy, err := h.Registry.GetChainPolicy(r.Context(), req.Chain, req.Network)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !policy.Enabled {
		http.Error(w, "chain/network policy disabled", http.StatusBadRequest)
		return
	}
	req.MinConfirmations = policy.RequiredConfirmations
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

	address, err := h.ChainAddr.ConvertAddress(r.Context(), req.Chain, req.Network, req.AddressType, pubKey)
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

func (h *WithdrawHandler) findAccountAddress(ctx context.Context, tenantID, accountID, chain, network, coin string) (ports.WalletAddress, error) {
	items, err := h.Registry.ListAccountAddresses(ctx, tenantID, accountID)
	if err != nil {
		return ports.WalletAddress{}, err
	}
	chain = strings.ToLower(strings.TrimSpace(chain))
	network = strings.ToLower(strings.TrimSpace(network))
	coin = strings.ToUpper(strings.TrimSpace(coin))
	for _, it := range items {
		if strings.ToUpper(strings.TrimSpace(it.Status)) != "ACTIVE" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(it.Chain)) != chain {
			continue
		}
		if strings.ToLower(strings.TrimSpace(it.Network)) != network {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(it.Coin)) != coin {
			continue
		}
		if strings.TrimSpace(it.Address) == "" || strings.TrimSpace(it.PublicKey) == "" {
			continue
		}
		return it, nil
	}
	return ports.WalletAddress{}, fmt.Errorf("active address not found for account=%s chain=%s network=%s coin=%s", accountID, chain, network, coin)
}
