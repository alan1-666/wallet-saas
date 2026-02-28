package httptransport

import (
	"net/http"

	"wallet-saas-v2/services/chain-gateway/internal/service"
)

func (h *ChainHandler) SendTx(w http.ResponseWriter, r *http.Request) {
	if !requireMethodPost(w, r) {
		return
	}
	var req SendTxRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	txHash, err := h.Chain.SendTx(r.Context(), service.SendTxInput{
		Chain:      req.Chain,
		Network:    req.Network,
		Coin:       req.Coin,
		RawTx:      req.RawTx,
		UnsignedTx: req.UnsignedTx,
		Signatures: req.Signatures,
		PublicKeys: req.PublicKeys,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, SendTxResponse{TxHash: txHash})
}

func (h *ChainHandler) BuildUnsignedTx(w http.ResponseWriter, r *http.Request) {
	if !requireMethodPost(w, r) {
		return
	}
	var req BuildUnsignedRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.Chain.BuildUnsigned(r.Context(), service.BuildUnsignedInput{
		Chain:    req.Chain,
		Network:  req.Network,
		Base64Tx: req.Base64Tx,
		Fee:      req.Fee,
		Vin:      toPortsVin(req.Vin),
		Vout:     toPortsVout(req.Vout),
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	writeJSON(w, BuildUnsignedResponse{UnsignedTx: res.UnsignedTx, SignHashes: res.SignHashes})
}
