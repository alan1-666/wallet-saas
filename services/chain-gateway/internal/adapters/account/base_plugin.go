package account

import (
	"context"
	"encoding/json"
	"fmt"

	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

// BasePlugin provides default unsupported implementations for optional methods.
type BasePlugin struct{}

func (BasePlugin) Chain() string { return "" }

func (BasePlugin) ConvertAddress(context.Context, string, string, string, string) (string, error) {
	return "", fmt.Errorf("%w: convert address", ports.ErrUnsupported)
}

func (BasePlugin) SendTx(context.Context, string, string, string, string) (string, error) {
	return "", fmt.Errorf("%w: send tx", ports.ErrUnsupported)
}

func (BasePlugin) BuildUnsignedAccount(context.Context, string, string, string) (ports.BuildUnsignedResult, error) {
	return ports.BuildUnsignedResult{}, fmt.Errorf("%w: build unsigned account", ports.ErrUnsupported)
}

func (BasePlugin) BuildSignedAccount(context.Context, string, string, string, string, string) (string, error) {
	return "", fmt.Errorf("%w: build signed account", ports.ErrUnsupported)
}

func (BasePlugin) SupportChains(context.Context, string, string) (bool, error) {
	return false, nil
}

func (BasePlugin) ValidAddress(context.Context, string, string, string, string) (bool, error) {
	return false, fmt.Errorf("%w: valid address", ports.ErrUnsupported)
}

func (BasePlugin) GetFee(context.Context, ports.FeeInput) (ports.FeeResult, error) {
	return ports.FeeResult{}, fmt.Errorf("%w: get fee", ports.ErrUnsupported)
}

func (BasePlugin) GetAccount(context.Context, ports.AccountInput) (ports.AccountResult, error) {
	return ports.AccountResult{}, fmt.Errorf("%w: get account", ports.ErrUnsupported)
}

func (BasePlugin) GetTxByHash(context.Context, ports.TxQueryInput) (json.RawMessage, error) {
	return nil, fmt.Errorf("%w: get tx by hash", ports.ErrUnsupported)
}

func (BasePlugin) GetTxByAddress(context.Context, ports.TxQueryInput) (json.RawMessage, error) {
	return nil, fmt.Errorf("%w: get tx by address", ports.ErrUnsupported)
}
