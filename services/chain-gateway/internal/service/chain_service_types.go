package service

import (
	"wallet-saas-v2/services/chain-gateway/internal/dispatcher"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

type ChainService struct {
	Router *dispatcher.Router
}

type BuildUnsignedInput struct {
	Chain    string
	Network  string
	Base64Tx string
	Fee      string
	Vin      []ports.TxVin
	Vout     []ports.TxVout
}

type SendTxInput struct {
	Chain      string
	Network    string
	Coin       string
	RawTx      string
	UnsignedTx string
	Signatures []string
	PublicKeys []string
}
