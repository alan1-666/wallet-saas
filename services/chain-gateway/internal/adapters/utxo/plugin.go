package utxo

import (
	"context"
	"encoding/json"

	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

// ChainPlugin defines utxo-model chain capabilities.
type ChainPlugin interface {
	Chain() string
	ConvertAddress(ctx context.Context, chain, network, addrType, publicKey string) (string, error)
	SendTx(ctx context.Context, chain, network, coin, rawTx string) (string, error)
	BuildUnsignedUTXO(ctx context.Context, chain, network, fee string, vin []ports.TxVin, vout []ports.TxVout) (ports.BuildUnsignedResult, error)
	BuildSignedUTXO(ctx context.Context, chain, network string, txData []byte, signatures [][]byte, publicKeys [][]byte) ([]byte, string, error)
	SupportChains(ctx context.Context, chain, network string) (bool, error)
	ValidAddress(ctx context.Context, chain, network, format, address string) (bool, error)
	GetFee(ctx context.Context, in ports.FeeInput) (ports.FeeResult, error)
	GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error)
	GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error)
	GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error)
	GetUnspentOutputs(ctx context.Context, chain, network, address string) (json.RawMessage, error)
}
