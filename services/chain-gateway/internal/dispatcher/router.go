package dispatcher

import (
	"fmt"

	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

type Router struct {
	Account ports.ChainAdapter
	UTXO    ports.ChainAdapter
}

func (r *Router) Resolve(chain string) (ports.ChainAdapter, error) {
	if clients.IsUTXOChain(chain) {
		if r.UTXO == nil {
			return nil, fmt.Errorf("utxo adapter not configured")
		}
		return r.UTXO, nil
	}
	if r.Account == nil {
		return nil, fmt.Errorf("account adapter not configured")
	}
	return r.Account, nil
}
