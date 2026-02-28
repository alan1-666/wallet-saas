package solana

import (
	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
)

func New(reader *RPCReader) accountadapter.ChainPlugin {
	return accountadapter.NewSolanaPlugin(reader)
}
