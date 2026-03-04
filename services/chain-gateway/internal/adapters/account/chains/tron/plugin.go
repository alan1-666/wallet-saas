package tron

import (
	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	bridgeplugin "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/common/bridge"
	sourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/config"
)

func New(conf *sourcecfg.Config) accountadapter.ChainPlugin {
	adaptor, err := NewChainAdaptor(conf)
	return bridgeplugin.NewPlugin("tron", ChainName, adaptor, err)
}
