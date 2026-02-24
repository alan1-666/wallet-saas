package handler

import (
	"bytes"
	"encoding/json"
	"net/http"

	"wallet-saas-v2/services/chain-gateway/internal/ports"
	"wallet-saas-v2/services/chain-gateway/internal/service"
)

type ChainHandler struct {
	Chain *service.ChainService
}

type ConvertAddressRequest struct {
	Chain     string `json:"chain"`
	Type      string `json:"type"`
	PublicKey string `json:"public_key"`
}

type ConvertAddressResponse struct {
	Address string `json:"address"`
}

type SendTxRequest struct {
	Chain      string   `json:"chain"`
	Network    string   `json:"network"`
	Coin       string   `json:"coin"`
	RawTx      string   `json:"raw_tx"`
	UnsignedTx string   `json:"unsigned_tx"`
	Signatures []string `json:"signatures"`
	PublicKeys []string `json:"public_keys"`
}

type SendTxResponse struct {
	TxHash string `json:"tx_hash"`
}

type BuildUnsignedVin struct {
	Hash    string `json:"hash"`
	Index   uint32 `json:"index"`
	Amount  int64  `json:"amount"`
	Address string `json:"address"`
}

type BuildUnsignedVout struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
	Index   uint32 `json:"index"`
}

type BuildUnsignedRequest struct {
	Chain    string              `json:"chain"`
	Network  string              `json:"network"`
	Base64Tx string              `json:"base64_tx"`
	Fee      string              `json:"fee"`
	Vin      []BuildUnsignedVin  `json:"vin"`
	Vout     []BuildUnsignedVout `json:"vout"`
}

type BuildUnsignedResponse struct {
	UnsignedTx string   `json:"unsigned_tx"`
	SignHashes []string `json:"sign_hashes,omitempty"`
}

type SupportChainsRequest struct {
	Chain   string `json:"chain"`
	Network string `json:"network"`
}

type SupportChainsResponse struct {
	Support bool `json:"support"`
}

type ValidAddressRequest struct {
	Chain   string `json:"chain"`
	Network string `json:"network"`
	Format  string `json:"format"`
	Address string `json:"address"`
}

type ValidAddressResponse struct {
	Valid bool `json:"valid"`
}

type FeeRequest struct {
	Chain   string `json:"chain"`
	Coin    string `json:"coin"`
	Network string `json:"network"`
	RawTx   string `json:"raw_tx"`
	Address string `json:"address"`
}

type AccountRequest struct {
	Chain           string `json:"chain"`
	Coin            string `json:"coin"`
	Network         string `json:"network"`
	Address         string `json:"address"`
	ContractAddress string `json:"contract_address"`
}

type TxByHashRequest struct {
	Chain   string `json:"chain"`
	Coin    string `json:"coin"`
	Network string `json:"network"`
	Hash    string `json:"hash"`
}

type TxByAddressRequest struct {
	Chain           string `json:"chain"`
	Coin            string `json:"coin"`
	Network         string `json:"network"`
	Address         string `json:"address"`
	ContractAddress string `json:"contract_address"`
	Page            uint32 `json:"page"`
	PageSize        uint32 `json:"page_size"`
	Cursor          string `json:"cursor"`
}

type UnspentOutputsRequest struct {
	Chain   string `json:"chain"`
	Network string `json:"network"`
	Address string `json:"address"`
}

func (h *ChainHandler) ConvertAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConvertAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	var (
		address string
		err     error
	)
	address, err = h.Chain.ConvertAddress(r.Context(), req.Chain, req.Type, req.PublicKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ConvertAddressResponse{Address: address})
}

func (h *ChainHandler) SendTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Network == "" {
		req.Network = "mainnet"
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SendTxResponse{TxHash: txHash})
}

func (h *ChainHandler) BuildUnsignedTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BuildUnsignedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Network == "" {
		req.Network = "mainnet"
	}

	vin := make([]ports.TxVin, 0, len(req.Vin))
	for _, v := range req.Vin {
		vin = append(vin, ports.TxVin{Hash: v.Hash, Index: v.Index, Amount: v.Amount, Address: v.Address})
	}
	vout := make([]ports.TxVout, 0, len(req.Vout))
	for _, v := range req.Vout {
		vout = append(vout, ports.TxVout{Address: v.Address, Amount: v.Amount, Index: v.Index})
	}
	res, err := h.Chain.BuildUnsigned(r.Context(), service.BuildUnsignedInput{
		Chain:    req.Chain,
		Network:  req.Network,
		Base64Tx: req.Base64Tx,
		Fee:      req.Fee,
		Vin:      vin,
		Vout:     vout,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BuildUnsignedResponse{UnsignedTx: res.UnsignedTx, SignHashes: res.SignHashes})
}

func (h *ChainHandler) SupportChains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SupportChainsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	support, err := h.Chain.SupportChains(r.Context(), service.SupportChainsInput{Chain: req.Chain, Network: req.Network})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SupportChainsResponse{Support: support})
}

func (h *ChainHandler) ValidAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ValidAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	valid, err := h.Chain.ValidAddress(r.Context(), service.ValidAddressInput{
		Chain:   req.Chain,
		Network: req.Network,
		Format:  req.Format,
		Address: req.Address,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ValidAddressResponse{Valid: valid})
}

func (h *ChainHandler) Fee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req FeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	fee, err := h.Chain.GetFee(r.Context(), ports.FeeInput{
		Chain:   req.Chain,
		Coin:    req.Coin,
		Network: req.Network,
		RawTx:   req.RawTx,
		Address: req.Address,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fee)
}

func (h *ChainHandler) AccountQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	acc, err := h.Chain.GetAccount(r.Context(), ports.AccountInput{
		Chain:           req.Chain,
		Coin:            req.Coin,
		Network:         req.Network,
		Address:         req.Address,
		ContractAddress: req.ContractAddress,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(acc)
}

func (h *ChainHandler) TxByHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req TxByHashRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	raw, err := h.Chain.GetTxByHash(r.Context(), ports.TxQueryInput{
		Chain:   req.Chain,
		Coin:    req.Coin,
		Network: req.Network,
		Hash:    req.Hash,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeRawJSON(w, raw)
}

func (h *ChainHandler) TxByAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req TxByAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	raw, err := h.Chain.GetTxByAddress(r.Context(), ports.TxQueryInput{
		Chain:           req.Chain,
		Coin:            req.Coin,
		Network:         req.Network,
		Address:         req.Address,
		ContractAddress: req.ContractAddress,
		Page:            req.Page,
		PageSize:        req.PageSize,
		Cursor:          req.Cursor,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeRawJSON(w, raw)
}

func (h *ChainHandler) UnspentOutputs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req UnspentOutputsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	raw, err := h.Chain.GetUnspentOutputs(r.Context(), req.Chain, req.Network, req.Address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeRawJSON(w, raw)
}

func writeRawJSON(w http.ResponseWriter, raw []byte) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}
