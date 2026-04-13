package httptransport

import (
	"wallet-saas-v2/services/chain-gateway/internal/endpoint"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
	"wallet-saas-v2/services/chain-gateway/internal/service"
)

type ChainHandler struct {
	Chain     *service.ChainService
	Endpoints *endpoint.Manager
}

type ConvertAddressRequest struct {
	Chain     string `json:"chain"`
	Network   string `json:"network"`
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

type ListIncomingTransfersRequest struct {
	Model           string `json:"model"`
	Chain           string `json:"chain"`
	Coin            string `json:"coin"`
	Network         string `json:"network"`
	Address         string `json:"address"`
	ContractAddress string `json:"contract_address"`
	Page            uint32 `json:"page"`
	PageSize        uint32 `json:"page_size"`
	Cursor          string `json:"cursor"`
}

type TxFinalityRequest struct {
	Chain   string `json:"chain"`
	Coin    string `json:"coin"`
	Network string `json:"network"`
	TxHash  string `json:"tx_hash"`
}

type BalanceRequest struct {
	Chain           string `json:"chain"`
	Coin            string `json:"coin"`
	Network         string `json:"network"`
	Address         string `json:"address"`
	ContractAddress string `json:"contract_address"`
}

func toPortsVin(items []BuildUnsignedVin) []ports.TxVin {
	out := make([]ports.TxVin, 0, len(items))
	for _, v := range items {
		out = append(out, ports.TxVin{Hash: v.Hash, Index: v.Index, Amount: v.Amount, Address: v.Address})
	}
	return out
}

func toPortsVout(items []BuildUnsignedVout) []ports.TxVout {
	out := make([]ports.TxVout, 0, len(items))
	for _, v := range items {
		out = append(out, ports.TxVout{Address: v.Address, Amount: v.Amount, Index: v.Index})
	}
	return out
}
