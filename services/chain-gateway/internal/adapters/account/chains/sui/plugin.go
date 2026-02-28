package sui

import (
	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	sourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/config"
)

func New(conf *sourcecfg.Config) accountadapter.ChainPlugin {
	adaptor, err := NewSuiAdaptor(conf)
	return accountadapter.NewBridgePlugin("sui", ChainName, adaptor, err)
}
