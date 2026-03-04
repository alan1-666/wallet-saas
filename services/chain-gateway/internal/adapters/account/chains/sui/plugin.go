package sui

import (
	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	bridgeplugin "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/common/bridge"
	sourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/config"
)

func New(conf *sourcecfg.Config) accountadapter.ChainPlugin {
	adaptor, err := NewSuiAdaptor(conf)
	return bridgeplugin.NewPlugin("sui", ChainName, adaptor, err)
}
