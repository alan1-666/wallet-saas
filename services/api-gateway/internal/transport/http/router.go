package httptransport

import (
	"net/http"
)

func NewMux(proxy *ProxyHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", Health)
	mux.HandleFunc("/v1/withdraw", proxy.ProxyWithdraw)
	mux.HandleFunc("/v1/withdraw/status", proxy.ProxyWithdrawStatus)
	mux.HandleFunc("/v1/withdraw/onchain/notify", proxy.ProxyWithdrawOnchainNotify)
	mux.HandleFunc("/v1/deposit/notify", proxy.ProxyDepositNotify)
	mux.HandleFunc("/v1/sweep/run", proxy.ProxySweepRun)
	mux.HandleFunc("/v1/sweep/onchain/notify", proxy.ProxySweepOnchainNotify)
	mux.HandleFunc("/v1/treasury/transfer", proxy.ProxyTreasuryTransfer)
	mux.HandleFunc("/v1/treasury/transfer/status", proxy.ProxyTreasuryTransferStatus)
	mux.HandleFunc("/v1/treasury/onchain/notify", proxy.ProxyTreasuryTransferOnchainNotify)
	mux.HandleFunc("/v1/treasury/waterline", proxy.ProxyTreasuryWaterline)
	mux.HandleFunc("/v1/balance", proxy.ProxyBalance)
	mux.HandleFunc("/v1/address/create", proxy.ProxyCreateAddress)
	mux.HandleFunc("/v1/account/upsert", proxy.ProxyAccountUpsert)
	mux.HandleFunc("/v1/account/get", proxy.ProxyAccountGet)
	mux.HandleFunc("/v1/account/list", proxy.ProxyAccountList)
	mux.HandleFunc("/v1/account/addresses", proxy.ProxyAccountAddresses)
	mux.HandleFunc("/v1/account/assets", proxy.ProxyAccountAssets)
	return mux
}
