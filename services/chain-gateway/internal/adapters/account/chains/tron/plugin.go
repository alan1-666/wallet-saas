package tron

import (
	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	sourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/config"
)

func New(conf *sourcecfg.Config) accountadapter.ChainPlugin {
	adaptor, err := NewChainAdaptor(conf)
	return accountadapter.NewBridgePlugin("tron", ChainName, adaptor, err)
}
