package httptransport

import (
	"net/http"
)

func NewMux(withdrawHandler *WithdrawHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/withdraw", withdrawHandler.Create)
	mux.HandleFunc("/v1/withdraw/status", withdrawHandler.Status)
	mux.HandleFunc("/v1/withdraw/onchain/notify", withdrawHandler.WithdrawOnchainNotify)
	mux.HandleFunc("/v1/deposit/notify", withdrawHandler.DepositNotify)
	mux.HandleFunc("/v1/sweep/run", withdrawHandler.SweepRun)
	mux.HandleFunc("/v1/sweep/onchain/notify", withdrawHandler.SweepOnchainNotify)
	mux.HandleFunc("/v1/treasury/transfer", withdrawHandler.TreasuryTransfer)
	mux.HandleFunc("/v1/treasury/transfer/status", withdrawHandler.TreasuryTransferStatus)
	mux.HandleFunc("/v1/treasury/onchain/notify", withdrawHandler.TreasuryTransferOnchainNotify)
	mux.HandleFunc("/v1/treasury/waterline", withdrawHandler.TreasuryWaterline)
	mux.HandleFunc("/v1/balance", withdrawHandler.Balance)
	mux.HandleFunc("/v1/address/create", withdrawHandler.CreateAddress)
	mux.HandleFunc("/v1/account/upsert", withdrawHandler.AccountUpsert)
	mux.HandleFunc("/v1/account/get", withdrawHandler.AccountGet)
	mux.HandleFunc("/v1/account/list", withdrawHandler.AccountList)
	mux.HandleFunc("/v1/account/addresses", withdrawHandler.AccountAddresses)
	mux.HandleFunc("/v1/account/assets", withdrawHandler.AccountAssets)
	return mux
}
