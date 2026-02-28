package bitcoin

import (
	utxoadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo"
	sourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/config"
)

func New(conf *sourcecfg.Config) utxoadapter.ChainPlugin {
	adaptor, err := NewChainAdaptor(conf)
	return utxoadapter.NewBridgePlugin("bitcoin", ChainName, adaptor, err)
}
