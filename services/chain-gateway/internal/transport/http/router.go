package httptransport

import (
	"net/http"
)

func NewMux(chainHandler *ChainHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", chainHandler.Healthz)
	mux.HandleFunc("/v1/chain/convert-address", chainHandler.ConvertAddress)
	mux.HandleFunc("/v1/chain/list-incoming-transfers", chainHandler.ListIncomingTransfers)
	mux.HandleFunc("/v1/chain/tx-finality", chainHandler.TxFinality)
	mux.HandleFunc("/v1/chain/balance", chainHandler.Balance)
	mux.HandleFunc("/v1/chain/send-tx", chainHandler.SendTx)
	mux.HandleFunc("/v1/chain/build-unsigned", chainHandler.BuildUnsignedTx)
	return mux
}
