package handler

import (
	"io"
	"net/http"
	"strings"
)

type ProxyHandler struct {
	WalletCoreAddr string
	Client         *http.Client
}

func NewProxyHandler(walletCoreAddr string) *ProxyHandler {
	return &ProxyHandler{WalletCoreAddr: strings.TrimRight(walletCoreAddr, "/"), Client: &http.Client{}}
}

func (h *ProxyHandler) ProxyWithdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.WalletCoreAddr+"/v1/withdraw", strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-Request-ID", r.Header.Get("X-Request-ID"))

	resp, err := h.Client.Do(req)
	if err != nil {
		http.Error(w, "wallet-core unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *ProxyHandler) ProxyDepositNotify(w http.ResponseWriter, r *http.Request) {
	h.proxyPost(w, r, "/v1/deposit/notify")
}

func (h *ProxyHandler) ProxySweepRun(w http.ResponseWriter, r *http.Request) {
	h.proxyPost(w, r, "/v1/sweep/run")
}

func (h *ProxyHandler) ProxyWithdrawStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	upstream := h.WalletCoreAddr + "/v1/withdraw/status"
	if r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-Request-ID", r.Header.Get("X-Request-ID"))

	resp, err := h.Client.Do(req)
	if err != nil {
		http.Error(w, "wallet-core unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *ProxyHandler) ProxyBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	upstream := h.WalletCoreAddr + "/v1/balance"
	if r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-Request-ID", r.Header.Get("X-Request-ID"))
	resp, err := h.Client.Do(req)
	if err != nil {
		http.Error(w, "wallet-core unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *ProxyHandler) ProxyCreateAddress(w http.ResponseWriter, r *http.Request) {
	h.proxyPost(w, r, "/v1/address/create")
}

func (h *ProxyHandler) ProxyAccountUpsert(w http.ResponseWriter, r *http.Request) {
	h.proxyPost(w, r, "/v1/account/upsert")
}

func (h *ProxyHandler) ProxyAccountGet(w http.ResponseWriter, r *http.Request) {
	h.proxyGet(w, r, "/v1/account/get")
}

func (h *ProxyHandler) ProxyAccountList(w http.ResponseWriter, r *http.Request) {
	h.proxyGet(w, r, "/v1/account/list")
}

func (h *ProxyHandler) ProxyAccountAddresses(w http.ResponseWriter, r *http.Request) {
	h.proxyGet(w, r, "/v1/account/addresses")
}

func (h *ProxyHandler) ProxyAccountAssets(w http.ResponseWriter, r *http.Request) {
	h.proxyGet(w, r, "/v1/account/assets")
}

func (h *ProxyHandler) proxyPost(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.WalletCoreAddr+path, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-Request-ID", r.Header.Get("X-Request-ID"))
	resp, err := h.Client.Do(req)
	if err != nil {
		http.Error(w, "wallet-core unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *ProxyHandler) proxyGet(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	upstream := h.WalletCoreAddr + path
	if r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-Request-ID", r.Header.Get("X-Request-ID"))
	resp, err := h.Client.Do(req)
	if err != nil {
		http.Error(w, "wallet-core unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
