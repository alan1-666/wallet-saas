package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	sourcechain "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains"
	"wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/account"
	sourcepbcommon "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/common"
	"wallet-saas-v2/services/chain-gateway/internal/ports"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Plugin bridges source chain adaptors into account adapter ChainPlugin.
// Keep this implementation in chains/common so chain logic stays under chains/.
type Plugin struct {
	accountadapter.BasePlugin
	chain           string
	sourceChainName string
	adapter         sourcechain.IChainAdaptor
	initErr         error
}

func NewPlugin(chainName string, sourceChainName string, adapter sourcechain.IChainAdaptor, initErr error) *Plugin {
	return &Plugin{
		chain:           strings.ToLower(strings.TrimSpace(chainName)),
		sourceChainName: strings.TrimSpace(sourceChainName),
		adapter:         adapter,
		initErr:         initErr,
	}
}

func (p *Plugin) Chain() string { return p.chain }

func (p *Plugin) ensureReady() error {
	if p.initErr != nil {
		return fmt.Errorf("account chain plugin init failed chain=%s: %w", p.chain, p.initErr)
	}
	if p.adapter == nil {
		return fmt.Errorf("account chain plugin not initialized chain=%s", p.chain)
	}
	return nil
}

func (p *Plugin) SupportChains(_ context.Context, chain, _ string) (bool, error) {
	return strings.EqualFold(strings.TrimSpace(chain), p.chain), nil
}

func (p *Plugin) ConvertAddress(_ context.Context, _chain, network, addrType, publicKey string) (string, error) {
	if err := p.ensureReady(); err != nil {
		return "", err
	}
	resp, err := p.adapter.ConvertAddress(&account.ConvertAddressRequest{
		Chain:     p.sourceChainName,
		Network:   strings.TrimSpace(network),
		Type:      strings.TrimSpace(addrType),
		PublicKey: strings.TrimPrefix(strings.TrimSpace(publicKey), "0x"),
	})
	if err != nil {
		return "", err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetAddress()), nil
}

func (p *Plugin) SendTx(_ context.Context, _chain, network, coin, rawTx string) (string, error) {
	if err := p.ensureReady(); err != nil {
		return "", err
	}
	resp, err := p.adapter.SendTx(&account.SendTxRequest{
		Chain:   p.sourceChainName,
		Coin:    strings.TrimSpace(coin),
		Network: strings.TrimSpace(network),
		RawTx:   strings.TrimSpace(rawTx),
	})
	if err != nil {
		return "", err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetTxHash()), nil
}

func (p *Plugin) BuildUnsignedAccount(_ context.Context, _chain, network, base64Tx string) (ports.BuildUnsignedResult, error) {
	if err := p.ensureReady(); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	payload := strings.TrimSpace(base64Tx)
	resp, err := p.adapter.BuildUnSignTransaction(&account.UnSignTransactionRequest{
		Chain:    p.sourceChainName,
		Network:  strings.TrimSpace(network),
		Base64Tx: payload,
	})
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	result := ports.BuildUnsignedResult{
		UnsignedTx: payload,
	}
	if hash := strings.TrimSpace(resp.GetUnSignTx()); hash != "" {
		result.SignHashes = []string{hash}
	}
	return result, nil
}

func (p *Plugin) BuildSignedAccount(_ context.Context, _chain, network, base64Tx, signature, publicKey string) (string, error) {
	if err := p.ensureReady(); err != nil {
		return "", err
	}
	resp, err := p.adapter.BuildSignedTransaction(&account.SignedTransactionRequest{
		Chain:     p.sourceChainName,
		Network:   strings.TrimSpace(network),
		Base64Tx:  strings.TrimSpace(base64Tx),
		Signature: strings.TrimSpace(signature),
		PublicKey: strings.TrimSpace(publicKey),
	})
	if err != nil {
		return "", err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetSignedTx()), nil
}

func (p *Plugin) ValidAddress(_ context.Context, _chain, network, _ string, address string) (bool, error) {
	if err := p.ensureReady(); err != nil {
		return false, err
	}
	resp, err := p.adapter.ValidAddress(&account.ValidAddressRequest{
		Chain:   p.sourceChainName,
		Network: strings.TrimSpace(network),
		Address: strings.TrimSpace(address),
	})
	if err != nil {
		return false, err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return false, err
	}
	return resp.GetValid(), nil
}

func (p *Plugin) GetFee(_ context.Context, in ports.FeeInput) (ports.FeeResult, error) {
	if err := p.ensureReady(); err != nil {
		return ports.FeeResult{}, err
	}
	resp, err := p.adapter.GetFee(&account.FeeRequest{
		Chain:   p.sourceChainName,
		Coin:    strings.TrimSpace(in.Coin),
		Network: strings.TrimSpace(in.Network),
		RawTx:   strings.TrimSpace(in.RawTx),
		Address: strings.TrimSpace(in.Address),
	})
	if err != nil {
		return ports.FeeResult{}, err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.FeeResult{}, err
	}
	return ports.FeeResult{
		SlowFee:   strings.TrimSpace(resp.GetSlowFee()),
		NormalFee: strings.TrimSpace(resp.GetNormalFee()),
		FastFee:   strings.TrimSpace(resp.GetFastFee()),
	}, nil
}

func (p *Plugin) GetAccount(_ context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	if err := p.ensureReady(); err != nil {
		return ports.AccountResult{}, err
	}
	resp, err := p.adapter.GetAccount(&account.AccountRequest{
		Chain:           p.sourceChainName,
		Coin:            strings.TrimSpace(in.Coin),
		Network:         strings.TrimSpace(in.Network),
		Address:         strings.TrimSpace(in.Address),
		ContractAddress: strings.TrimSpace(in.ContractAddress),
	})
	if err != nil {
		return ports.AccountResult{}, err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.AccountResult{}, err
	}
	return ports.AccountResult{
		Network:       strings.TrimSpace(resp.GetNetwork()),
		AccountNumber: strings.TrimSpace(resp.GetAccountNumber()),
		Sequence:      strings.TrimSpace(resp.GetSequence()),
		Balance:       strings.TrimSpace(resp.GetBalance()),
	}, nil
}

func (p *Plugin) GetTxByHash(_ context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if err := p.ensureReady(); err != nil {
		return nil, err
	}
	resp, err := p.adapter.GetTxByHash(&account.TxHashRequest{
		Chain:   p.sourceChainName,
		Coin:    strings.TrimSpace(in.Coin),
		Network: strings.TrimSpace(in.Network),
		Hash:    strings.TrimSpace(in.Hash),
	})
	if err != nil {
		return nil, err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, err
	}
	return marshalProtoJSON(resp), nil
}

func (p *Plugin) GetTxByAddress(_ context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if err := p.ensureReady(); err != nil {
		return nil, err
	}
	page := in.Page
	if page == 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize == 0 {
		pageSize = 50
	}
	resp, err := p.adapter.GetTxByAddress(&account.TxAddressRequest{
		Chain:           p.sourceChainName,
		Coin:            strings.TrimSpace(in.Coin),
		Network:         strings.TrimSpace(in.Network),
		Address:         strings.TrimSpace(in.Address),
		ContractAddress: strings.TrimSpace(in.ContractAddress),
		Page:            page,
		Pagesize:        pageSize,
	})
	if err != nil {
		return nil, err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, err
	}
	return marshalProtoJSON(resp), nil
}

func expectCodeOK(code sourcepbcommon.ReturnCode, msg string) error {
	if code == sourcepbcommon.ReturnCode_SUCCESS {
		return nil
	}
	if strings.TrimSpace(msg) == "" {
		msg = "chain source adapter request failed"
	}
	return fmt.Errorf("%s", strings.TrimSpace(msg))
}

func marshalProtoJSON(v any) json.RawMessage {
	pm, ok := v.(proto.Message)
	if !ok {
		raw, _ := json.Marshal(v)
		return raw
	}
	raw, err := protojson.Marshal(pm)
	if err != nil {
		fallback, _ := json.Marshal(v)
		return fallback
	}
	return raw
}
