package httptransport

import "net/http"

func (h *ChainHandler) ConvertAddress(w http.ResponseWriter, r *http.Request) {
	if !requireMethodPost(w, r) {
		return
	}
	var req ConvertAddressRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	address, err := h.Chain.ConvertAddress(r.Context(), req.Chain, req.Network, req.Type, req.PublicKey)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	writeJSON(w, ConvertAddressResponse{Address: address})
}
