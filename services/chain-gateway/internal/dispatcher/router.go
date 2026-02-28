package dispatcher

import (
	"fmt"
	"strings"
	"sync"

	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

const wildcardNetwork = "*"

type Router struct {
	mu       sync.RWMutex
	bindings map[string]ports.PluginBinding
}

func NewRouter() *Router {
	return &Router{bindings: make(map[string]ports.PluginBinding)}
}

func (r *Router) Register(binding ports.PluginBinding) error {
	chain := clients.NormalizeChain(binding.Chain)
	if chain == "" {
		return fmt.Errorf("chain is required")
	}
	if binding.Adapter == nil {
		return fmt.Errorf("adapter is required")
	}
	network := normalizeNetwork(binding.Network)
	if network == "" {
		return fmt.Errorf("network is required")
	}
	key := bindingKey(chain, network)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings[key] = ports.PluginBinding{
		Chain:   chain,
		Network: network,
		Model:   binding.Model,
		Adapter: binding.Adapter,
	}
	return nil
}

func (r *Router) Resolve(chain, network string) (ports.PluginBinding, error) {
	c := clients.NormalizeChain(chain)
	if c == "" {
		return ports.PluginBinding{}, fmt.Errorf("chain is required")
	}
	n := normalizeNetwork(network)
	if n == "" {
		return ports.PluginBinding{}, fmt.Errorf("network is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if hit, ok := r.bindings[bindingKey(c, n)]; ok {
		return hit, nil
	}
	if hit, ok := r.bindings[bindingKey(c, wildcardNetwork)]; ok {
		return hit, nil
	}
	return ports.PluginBinding{}, fmt.Errorf("chain plugin not configured chain=%s network=%s", c, n)
}

func bindingKey(chain, network string) string {
	return chain + "|" + network
}

func normalizeNetwork(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}
