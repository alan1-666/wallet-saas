package httptransport

import (
	"net/http"

	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

func (h *ChainHandler) ListIncomingTransfers(w http.ResponseWriter, r *http.Request) {
	if !requireMethodPost(w, r) {
		return
	}
	var req ListIncomingTransfersRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	out, err := h.Chain.ListIncomingTransfers(r.Context(), ports.IncomingTransferInput{
		Model:           ports.ChainModel(req.Model),
		Chain:           req.Chain,
		Coin:            req.Coin,
		Network:         req.Network,
		Address:         req.Address,
		ContractAddress: req.ContractAddress,
		Cursor:          req.Cursor,
		Page:            req.Page,
		PageSize:        req.PageSize,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	writeJSON(w, out)
}

func (h *ChainHandler) TxFinality(w http.ResponseWriter, r *http.Request) {
	if !requireMethodPost(w, r) {
		return
	}
	var req TxFinalityRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	out, err := h.Chain.GetTxFinality(r.Context(), ports.TxFinalityInput{
		Chain:   req.Chain,
		Coin:    req.Coin,
		Network: req.Network,
		TxHash:  req.TxHash,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	writeJSON(w, out)
}

func (h *ChainHandler) Balance(w http.ResponseWriter, r *http.Request) {
	if !requireMethodPost(w, r) {
		return
	}
	var req BalanceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	out, err := h.Chain.GetBalance(r.Context(), ports.BalanceInput{
		Chain:           req.Chain,
		Coin:            req.Coin,
		Network:         req.Network,
		Address:         req.Address,
		ContractAddress: req.ContractAddress,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	writeJSON(w, out)
}
