package server

import (
	"net/http"

	"wallet-saas-v2/services/wallet-core/internal/handler"
)

func NewMux(withdrawHandler *handler.WithdrawHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/withdraw", withdrawHandler.Create)
	mux.HandleFunc("/v1/withdraw/status", withdrawHandler.Status)
	mux.HandleFunc("/v1/deposit/notify", withdrawHandler.DepositNotify)
	mux.HandleFunc("/v1/sweep/run", withdrawHandler.SweepRun)
	mux.HandleFunc("/v1/balance", withdrawHandler.Balance)
	mux.HandleFunc("/v1/address/create", withdrawHandler.CreateAddress)
	mux.HandleFunc("/v1/account/upsert", withdrawHandler.AccountUpsert)
	mux.HandleFunc("/v1/account/get", withdrawHandler.AccountGet)
	mux.HandleFunc("/v1/account/list", withdrawHandler.AccountList)
	mux.HandleFunc("/v1/account/addresses", withdrawHandler.AccountAddresses)
	mux.HandleFunc("/v1/account/assets", withdrawHandler.AccountAssets)
	return mux
}
