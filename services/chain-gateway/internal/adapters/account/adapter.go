package account

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

// Adapter multiplexes account-model plugins by chain.
type Adapter struct {
	mu      sync.RWMutex
	plugins map[string]ChainPlugin
}

func NewAdapter(plugins ...ChainPlugin) (*Adapter, error) {
	a := &Adapter{plugins: make(map[string]ChainPlugin)}
	for _, plugin := range plugins {
		if err := a.Register(plugin); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (a *Adapter) Register(plugin ChainPlugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin is nil")
	}
	chain := clients.NormalizeChain(plugin.Chain())
	if chain == "" {
		return fmt.Errorf("plugin chain is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.plugins[chain] = plugin
	return nil
}

func (a *Adapter) resolve(chain string) (ChainPlugin, error) {
	normalized := clients.NormalizeChain(chain)
	if normalized == "" {
		return nil, fmt.Errorf("chain is required")
	}
	a.mu.RLock()
	plugin, ok := a.plugins[normalized]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("account plugin not configured chain=%s", normalized)
	}
	return plugin, nil
}

func (a *Adapter) SupportedChains() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, 0, len(a.plugins))
	for chain := range a.plugins {
		out = append(out, chain)
	}
	sort.Strings(out)
	return out
}

func (a *Adapter) ConvertAddress(ctx context.Context, chain, network, addrType, publicKey string) (string, error) {
	plugin, err := a.resolve(chain)
	if err != nil {
		return "", err
	}
	return plugin.ConvertAddress(ctx, clients.NormalizeChain(chain), strings.ToLower(strings.TrimSpace(network)), addrType, publicKey)
}

func (a *Adapter) SendTx(ctx context.Context, chain, network, coin, rawTx string) (string, error) {
	plugin, err := a.resolve(chain)
	if err != nil {
		return "", err
	}
	return plugin.SendTx(ctx, clients.NormalizeChain(chain), strings.ToLower(strings.TrimSpace(network)), coin, rawTx)
}

func (a *Adapter) BuildUnsignedAccount(ctx context.Context, chain, network, base64Tx string) (ports.BuildUnsignedResult, error) {
	plugin, err := a.resolve(chain)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	return plugin.BuildUnsignedAccount(ctx, clients.NormalizeChain(chain), strings.ToLower(strings.TrimSpace(network)), base64Tx)
}

func (a *Adapter) BuildSignedAccount(ctx context.Context, chain, network, base64Tx, signature, publicKey string) (string, error) {
	plugin, err := a.resolve(chain)
	if err != nil {
		return "", err
	}
	return plugin.BuildSignedAccount(
		ctx,
		clients.NormalizeChain(chain),
		strings.ToLower(strings.TrimSpace(network)),
		base64Tx,
		signature,
		publicKey,
	)
}

func (a *Adapter) BuildUnsignedUTXO(context.Context, string, string, string, []ports.TxVin, []ports.TxVout) (ports.BuildUnsignedResult, error) {
	return ports.BuildUnsignedResult{}, ports.ErrUnsupported
}

func (a *Adapter) BuildSignedUTXO(context.Context, string, string, []byte, [][]byte, [][]byte) ([]byte, string, error) {
	return nil, "", ports.ErrUnsupported
}

func (a *Adapter) SupportChains(ctx context.Context, chain, network string) (bool, error) {
	plugin, err := a.resolve(chain)
	if err != nil {
		return false, nil
	}
	return plugin.SupportChains(ctx, clients.NormalizeChain(chain), strings.ToLower(strings.TrimSpace(network)))
}

func (a *Adapter) ValidAddress(ctx context.Context, chain, network, format, address string) (bool, error) {
	plugin, err := a.resolve(chain)
	if err != nil {
		return false, err
	}
	return plugin.ValidAddress(ctx, clients.NormalizeChain(chain), strings.ToLower(strings.TrimSpace(network)), format, address)
}

func (a *Adapter) GetFee(ctx context.Context, in ports.FeeInput) (ports.FeeResult, error) {
	plugin, err := a.resolve(in.Chain)
	if err != nil {
		return ports.FeeResult{}, err
	}
	in.Chain = clients.NormalizeChain(in.Chain)
	in.Network = strings.ToLower(strings.TrimSpace(in.Network))
	return plugin.GetFee(ctx, in)
}

func (a *Adapter) GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	plugin, err := a.resolve(in.Chain)
	if err != nil {
		return ports.AccountResult{}, err
	}
	in.Chain = clients.NormalizeChain(in.Chain)
	in.Network = strings.ToLower(strings.TrimSpace(in.Network))
	return plugin.GetAccount(ctx, in)
}

func (a *Adapter) GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	plugin, err := a.resolve(in.Chain)
	if err != nil {
		return nil, err
	}
	in.Chain = clients.NormalizeChain(in.Chain)
	in.Network = strings.ToLower(strings.TrimSpace(in.Network))
	return plugin.GetTxByHash(ctx, in)
}

func (a *Adapter) GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	plugin, err := a.resolve(in.Chain)
	if err != nil {
		return nil, err
	}
	in.Chain = clients.NormalizeChain(in.Chain)
	in.Network = strings.ToLower(strings.TrimSpace(in.Network))
	return plugin.GetTxByAddress(ctx, in)
}

func (a *Adapter) GetUnspentOutputs(context.Context, string, string, string) (json.RawMessage, error) {
	return nil, ports.ErrUnsupported
}
