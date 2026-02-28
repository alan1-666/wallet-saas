package evm

import (
	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
)

func New(chain string, reader *RPCReader) accountadapter.ChainPlugin {
	return accountadapter.NewEVMPlugin(chain, reader)
}
