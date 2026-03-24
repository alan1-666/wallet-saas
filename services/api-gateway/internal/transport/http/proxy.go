package httptransport

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wallet-saas-v2/services/api-gateway/internal/security"
)

const maxRequestBodyBytes int64 = 1024 * 1024

type ProxyHandler struct {
	WalletCoreAddr string
	Client         *http.Client
	Security       security.Provider
}

type routeSpec struct {
	Method              string
	Action              string
	Permission          string
	IdempotencyOp       string
	RequireTenant       bool
	RequireChainNetwork bool
	CheckSignPermission bool
}

func NewProxyHandler(walletCoreAddr string, timeout time.Duration, sec security.Provider) *ProxyHandler {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if sec == nil {
		sec = security.NewNoop()
	}
	return &ProxyHandler{
		WalletCoreAddr: strings.TrimRight(walletCoreAddr, "/"),
		Client:         &http.Client{Timeout: timeout},
		Security:       sec,
	}
}

func (h *ProxyHandler) ProxyWithdraw(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/withdraw", routeSpec{
		Method:              http.MethodPost,
		Action:              "withdraw",
		Permission:          "withdraw",
		IdempotencyOp:       "withdraw",
		RequireTenant:       true,
		RequireChainNetwork: true,
		CheckSignPermission: true,
	})
}

func (h *ProxyHandler) ProxyDepositNotify(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/deposit/notify", routeSpec{
		Method:              http.MethodPost,
		Action:              "deposit_notify",
		Permission:          "deposit",
		IdempotencyOp:       "deposit_notify",
		RequireTenant:       true,
		RequireChainNetwork: true,
	})
}

func (h *ProxyHandler) ProxyWithdrawOnchainNotify(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/withdraw/onchain/notify", routeSpec{
		Method:        http.MethodPost,
		Action:        "withdraw_onchain_notify",
		Permission:    "withdraw",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxySweepRun(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/sweep/run", routeSpec{
		Method:              http.MethodPost,
		Action:              "sweep_run",
		Permission:          "sweep",
		IdempotencyOp:       "sweep_run",
		RequireTenant:       true,
		RequireChainNetwork: true,
	})
}

func (h *ProxyHandler) ProxySweepOnchainNotify(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/sweep/onchain/notify", routeSpec{
		Method:        http.MethodPost,
		Action:        "sweep_onchain_notify",
		Permission:    "sweep",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyTreasuryTransfer(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/treasury/transfer", routeSpec{
		Method:              http.MethodPost,
		Action:              "treasury_transfer",
		Permission:          "sweep",
		IdempotencyOp:       "treasury_transfer",
		RequireTenant:       true,
		RequireChainNetwork: true,
	})
}

func (h *ProxyHandler) ProxyTreasuryTransferOnchainNotify(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/treasury/onchain/notify", routeSpec{
		Method:        http.MethodPost,
		Action:        "treasury_transfer_onchain_notify",
		Permission:    "sweep",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyTreasuryTransferStatus(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/treasury/transfer/status", routeSpec{
		Method:        http.MethodGet,
		Action:        "treasury_transfer_status",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyTreasuryWaterline(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/treasury/waterline", routeSpec{
		Method:        http.MethodGet,
		Action:        "treasury_waterline",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyWithdrawStatus(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/withdraw/status", routeSpec{
		Method:        http.MethodGet,
		Action:        "withdraw_status",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyBalance(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/balance", routeSpec{
		Method:        http.MethodGet,
		Action:        "balance",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyCreateAddress(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/address/create", routeSpec{
		Method:              http.MethodPost,
		Action:              "address_create",
		Permission:          "deposit",
		RequireTenant:       true,
		RequireChainNetwork: true,
	})
}

func (h *ProxyHandler) ProxyAccountUpsert(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/account/upsert", routeSpec{
		Method:        http.MethodPost,
		Action:        "account_upsert",
		Permission:    "deposit",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyAccountGet(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/account/get", routeSpec{
		Method:        http.MethodGet,
		Action:        "account_get",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyAccountList(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/account/list", routeSpec{
		Method:        http.MethodGet,
		Action:        "account_list",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyAccountAddresses(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/account/addresses", routeSpec{
		Method:        http.MethodGet,
		Action:        "account_addresses",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) ProxyAccountAssets(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/v1/account/assets", routeSpec{
		Method:        http.MethodGet,
		Action:        "account_assets",
		RequireTenant: true,
	})
}

func (h *ProxyHandler) proxy(w http.ResponseWriter, r *http.Request, upstreamPath string, spec routeSpec) {
	if r.Method != spec.Method {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestID := requestIDFrom(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = generateRequestID(spec.Action)
	}
	w.Header().Set("X-Request-ID", requestID)

	var body []byte
	var err error
	if spec.Method == http.MethodPost {
		body, err = readBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	tenantID, err := tenantFromRequest(r, body)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if spec.RequireTenant && strings.TrimSpace(tenantID) == "" {
		http.Error(w, "tenant_id is required", http.StatusBadRequest)
		return
	}

	_, statusCode, authMsg := h.authorize(r, tenantID, spec.Permission)
	if statusCode != 0 {
		http.Error(w, authMsg, statusCode)
		return
	}
	if spec.CheckSignPermission {
		if err := h.checkSignPermission(r, tenantID, body); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	}
	chain, network, amount, err := requestChainContext(r, body)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if spec.RequireChainNetwork && (strings.TrimSpace(chain) == "" || strings.TrimSpace(network) == "") {
		http.Error(w, "chain and network are required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(chain) != "" && strings.TrimSpace(network) != "" {
		if err := h.Security.CheckTenantChainPolicy(r.Context(), tenantID, chain, network, spec.Permission, amount); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	}

	idemState := "NONE"
	if spec.IdempotencyOp != "" {
		reqHash := hashRequest(body)
		idem, err := h.Security.Reserve(r.Context(), tenantID, requestID, spec.IdempotencyOp, reqHash)
		if err != nil {
			http.Error(w, "idempotency unavailable", http.StatusInternalServerError)
			return
		}
		idemState = idem.State
		switch idem.State {
		case "REPLAY":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(idem.Response))
			return
		case "CONFLICT":
			http.Error(w, "idempotency conflict", http.StatusConflict)
			return
		case "REJECTED":
			http.Error(w, "previous request failed: "+idem.Response, http.StatusConflict)
			return
		}
	}

	respStatus, respBody, contentType, err := h.forward(r, upstreamPath, requestID, body)
	if err != nil {
		http.Error(w, "wallet-core unavailable", http.StatusBadGateway)
		return
	}

	if spec.IdempotencyOp != "" && idemState == "NEW" {
		if respStatus >= 200 && respStatus < 300 {
			_ = h.Security.Commit(r.Context(), tenantID, requestID, spec.IdempotencyOp, string(respBody))
		} else if respStatus >= 400 && respStatus < 500 {
			_ = h.Security.Reject(r.Context(), tenantID, requestID, spec.IdempotencyOp, fmt.Sprintf("upstream status=%d body=%s", respStatus, string(respBody)))
		}
	}
	if strings.TrimSpace(tenantID) != "" {
		_ = h.Security.Audit(r.Context(), tenantID, spec.Action, requestID, fmt.Sprintf("path=%s status=%d", upstreamPath, respStatus))
	}

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(respStatus)
	_, _ = w.Write(respBody)
}

func (h *ProxyHandler) forward(r *http.Request, upstreamPath, requestID string, body []byte) (int, []byte, string, error) {
	upstream := h.WalletCoreAddr + upstreamPath
	if r.Method == http.MethodGet && r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstream, reader)
	if err != nil {
		return 0, nil, "", err
	}
	if r.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-Request-ID", requestID)

	resp, err := h.Client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, "", err
	}
	return resp.StatusCode, respBody, resp.Header.Get("Content-Type"), nil
}

func (h *ProxyHandler) authorize(r *http.Request, tenantID, permission string) (security.Scope, int, string) {
	token := bearerToken(r.Header.Get("Authorization"))
	if strings.TrimSpace(token) == "" {
		return security.Scope{}, http.StatusUnauthorized, "missing token"
	}
	scope, err := h.Security.ValidateToken(r.Context(), token)
	if err != nil {
		return security.Scope{}, http.StatusUnauthorized, "invalid token"
	}
	if strings.TrimSpace(tenantID) != "" && scope.TenantID != "*" && scope.TenantID != tenantID {
		return security.Scope{}, http.StatusForbidden, "tenant mismatch"
	}
	switch permission {
	case "withdraw":
		if !scope.CanWithdraw {
			return security.Scope{}, http.StatusForbidden, "withdraw not allowed"
		}
	case "deposit":
		if !scope.CanDeposit {
			return security.Scope{}, http.StatusForbidden, "deposit not allowed"
		}
	case "sweep":
		if !scope.CanSweep {
			return security.Scope{}, http.StatusForbidden, "sweep not allowed"
		}
	}
	return scope, 0, ""
}

func (h *ProxyHandler) checkSignPermission(r *http.Request, tenantID string, body []byte) error {
	type keyCheckRequest struct {
		KeyID  string   `json:"key_id"`
		KeyIDs []string `json:"key_ids"`
	}
	var req keyCheckRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return fmt.Errorf("invalid json")
	}
	keys := make([]string, 0, 1+len(req.KeyIDs))
	if strings.TrimSpace(req.KeyID) != "" {
		keys = append(keys, strings.TrimSpace(req.KeyID))
	}
	for _, k := range req.KeyIDs {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		allowed, err := h.Security.CheckSignPermission(r.Context(), tenantID, key)
		if err != nil {
			return fmt.Errorf("check sign permission failed")
		}
		if !allowed {
			return fmt.Errorf("key permission denied")
		}
	}
	return nil
}

func readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read request body")
	}
	if int64(len(body)) > maxRequestBodyBytes {
		return nil, fmt.Errorf("request body too large")
	}
	return body, nil
}

func tenantFromRequest(r *http.Request, body []byte) (string, error) {
	if r.Method == http.MethodGet {
		return strings.TrimSpace(r.URL.Query().Get("tenant_id")), nil
	}
	if len(body) == 0 {
		return "", nil
	}
	var payload struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.TenantID), nil
}

func requestChainContext(r *http.Request, body []byte) (string, string, string, error) {
	if r.Method == http.MethodGet {
		return strings.ToLower(strings.TrimSpace(r.URL.Query().Get("chain"))),
			strings.ToLower(strings.TrimSpace(r.URL.Query().Get("network"))),
			strings.TrimSpace(r.URL.Query().Get("amount")),
			nil
	}
	if len(body) == 0 {
		return "", "", "", nil
	}
	var payload struct {
		Chain   string `json:"chain"`
		Network string `json:"network"`
		Amount  string `json:"amount"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", "", err
	}
	return strings.ToLower(strings.TrimSpace(payload.Chain)),
		strings.ToLower(strings.TrimSpace(payload.Network)),
		strings.TrimSpace(payload.Amount),
		nil
}

func hashRequest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
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

func requestIDFrom(headerValue string) string {
	v := strings.TrimSpace(headerValue)
	return v
}

func generateRequestID(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "req"
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%d-%s", p, time.Now().UnixNano(), hex.EncodeToString(b))
}
