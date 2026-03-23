package httptransport

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

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
	bal, err := h.Ledger.GetBalance(r.Context(), tenantID, accountID, asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BalanceResponse{
		TenantID:       tenantID,
		AccountID:      accountID,
		Asset:          asset,
		Available:      bal.Available,
		Frozen:         bal.Frozen,
		WithdrawLocked: bal.WithdrawLocked,
		Withdrawable:   bal.Withdrawable,
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TenantID == "" || req.AccountID == "" {
		http.Error(w, "tenant_id/account_id are required", http.StatusBadRequest)
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
	items, err := h.Ledger.ListAccountAssets(r.Context(), tenantID, accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AccountAssetsResponse{Items: items})
}
