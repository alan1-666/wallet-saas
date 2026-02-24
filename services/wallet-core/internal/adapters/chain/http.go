package chain

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type HTTPChain struct {
	baseURL string
	client  *http.Client
}

type buildUnsignedRequest struct {
	Chain    string `json:"chain"`
	Network  string `json:"network"`
	Coin     string `json:"coin"`
	Base64Tx string `json:"base64_tx"`
	Fee      string `json:"fee"`
	Vin      []vin  `json:"vin,omitempty"`
	Vout     []vout `json:"vout,omitempty"`
}

type vin struct {
	Hash    string `json:"hash"`
	Index   uint32 `json:"index"`
	Amount  int64  `json:"amount"`
	Address string `json:"address"`
}

type vout struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
	Index   uint32 `json:"index"`
}

type buildUnsignedResponse struct {
	UnsignedTx string   `json:"unsigned_tx"`
	SignHashes []string `json:"sign_hashes,omitempty"`
}

type sendTxRequest struct {
	Chain      string   `json:"chain"`
	Network    string   `json:"network"`
	Coin       string   `json:"coin"`
	RawTx      string   `json:"raw_tx,omitempty"`
	UnsignedTx string   `json:"unsigned_tx,omitempty"`
	Signatures []string `json:"signatures,omitempty"`
	PublicKeys []string `json:"public_keys,omitempty"`
}

type sendTxResponse struct {
	TxHash string `json:"tx_hash"`
}

func NewHTTP(baseURL string) *HTTPChain {
	return &HTTPChain{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{}}
}

func (h *HTTPChain) BuildUnsignedTx(ctx context.Context, params ports.BuildUnsignedParams) (ports.BuildUnsignedResult, error) {
	network := params.Network
	if network == "" {
		network = "mainnet"
	}

	base64Tx := params.Base64Tx
	if base64Tx == "" {
		raw := params.Chain + ":" + params.From + ":" + params.To + ":" + params.Amount
		base64Tx = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	payload := buildUnsignedRequest{
		Chain:    params.Chain,
		Network:  network,
		Coin:     params.Coin,
		Base64Tx: base64Tx,
		Fee:      params.Fee,
		Vin:      make([]vin, 0, len(params.Vin)),
		Vout:     make([]vout, 0, len(params.Vout)),
	}
	for _, x := range params.Vin {
		payload.Vin = append(payload.Vin, vin{Hash: x.Hash, Index: x.Index, Amount: x.Amount, Address: x.Address})
	}
	for _, x := range params.Vout {
		payload.Vout = append(payload.Vout, vout{Address: x.Address, Amount: x.Amount, Index: x.Index})
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/v1/chain/build-unsigned", bytes.NewReader(body))
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return ports.BuildUnsignedResult{}, fmt.Errorf("chain-gateway status: %d", resp.StatusCode)
	}

	var out buildUnsignedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	if out.UnsignedTx == "" {
		return ports.BuildUnsignedResult{}, fmt.Errorf("empty unsigned tx")
	}
	return ports.BuildUnsignedResult{
		UnsignedTx: out.UnsignedTx,
		SignHashes: out.SignHashes,
	}, nil
}

func (h *HTTPChain) Broadcast(ctx context.Context, params ports.BroadcastParams) (string, error) {
	network := params.Network
	if network == "" {
		network = "mainnet"
	}
	payload := sendTxRequest{
		Chain:      params.Chain,
		Network:    network,
		Coin:       params.Coin,
		RawTx:      params.RawTx,
		UnsignedTx: params.UnsignedTx,
		Signatures: params.Signatures,
		PublicKeys: params.PublicKeys,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/v1/chain/send-tx", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("chain-gateway status: %d", resp.StatusCode)
	}

	var out sendTxResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.TxHash == "" {
		return "", fmt.Errorf("empty tx hash")
	}
	return out.TxHash, nil
}
