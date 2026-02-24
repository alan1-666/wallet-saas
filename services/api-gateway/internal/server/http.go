package server

import (
	"net/http"

	"wallet-saas-v2/services/api-gateway/internal/handler"
)

func NewMux(proxy *handler.ProxyHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Health)
	mux.HandleFunc("/v1/withdraw", proxy.ProxyWithdraw)
	mux.HandleFunc("/v1/withdraw/status", proxy.ProxyWithdrawStatus)
	mux.HandleFunc("/v1/deposit/notify", proxy.ProxyDepositNotify)
	mux.HandleFunc("/v1/sweep/run", proxy.ProxySweepRun)
	mux.HandleFunc("/v1/balance", proxy.ProxyBalance)
	mux.HandleFunc("/v1/address/create", proxy.ProxyCreateAddress)
	mux.HandleFunc("/v1/account/upsert", proxy.ProxyAccountUpsert)
	mux.HandleFunc("/v1/account/get", proxy.ProxyAccountGet)
	mux.HandleFunc("/v1/account/list", proxy.ProxyAccountList)
	mux.HandleFunc("/v1/account/addresses", proxy.ProxyAccountAddresses)
	mux.HandleFunc("/v1/account/assets", proxy.ProxyAccountAssets)
	return mux
}
