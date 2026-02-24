package server

import (
	"net/http"

	"wallet-saas-v2/services/chain-gateway/internal/handler"
)

func NewMux(chainHandler *handler.ChainHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/chain/convert-address", chainHandler.ConvertAddress)
	mux.HandleFunc("/v1/chain/support-chains", chainHandler.SupportChains)
	mux.HandleFunc("/v1/chain/valid-address", chainHandler.ValidAddress)
	mux.HandleFunc("/v1/chain/fee", chainHandler.Fee)
	mux.HandleFunc("/v1/chain/account", chainHandler.AccountQuery)
	mux.HandleFunc("/v1/chain/tx-by-hash", chainHandler.TxByHash)
	mux.HandleFunc("/v1/chain/tx-by-address", chainHandler.TxByAddress)
	mux.HandleFunc("/v1/chain/unspent-outputs", chainHandler.UnspentOutputs)
	mux.HandleFunc("/v1/chain/send-tx", chainHandler.SendTx)
	mux.HandleFunc("/v1/chain/build-unsigned", chainHandler.BuildUnsignedTx)
	return mux
}
