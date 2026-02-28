package account

import (
	"context"
	"encoding/json"

	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

// ChainPlugin defines account-model chain capabilities.
type ChainPlugin interface {
	Chain() string
	ConvertAddress(ctx context.Context, chain, network, addrType, publicKey string) (string, error)
	SendTx(ctx context.Context, chain, network, coin, rawTx string) (string, error)
	BuildUnsignedAccount(ctx context.Context, chain, network, base64Tx string) (string, error)
	SupportChains(ctx context.Context, chain, network string) (bool, error)
	ValidAddress(ctx context.Context, chain, network, format, address string) (bool, error)
	GetFee(ctx context.Context, in ports.FeeInput) (ports.FeeResult, error)
	GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error)
	GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error)
	GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error)
}
