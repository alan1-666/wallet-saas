package tron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	sourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/config"
	sourcepb "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/account"
	sourcepbcommon "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/common"
	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Plugin struct {
	accountadapter.BasePlugin
	adaptor *ChainAdaptor
	initErr error
}

func New(conf *sourcecfg.Config) accountadapter.ChainPlugin {
	adaptor, err := NewChainAdaptor(conf)
	return &Plugin{
		adaptor: adaptor,
		initErr: err,
	}
}

func (p *Plugin) Chain() string {
	return "tron"
}

func (p *Plugin) ensureWriter() error {
	if p.initErr != nil {
		return fmt.Errorf("tron adaptor init failed: %w", p.initErr)
	}
	if p.adaptor == nil {
		return fmt.Errorf("tron adaptor not initialized")
	}
	return nil
}

func (p *Plugin) SupportChains(_ context.Context, chain, _ string) (bool, error) {
	return clients.NormalizeChain(chain) == "tron", nil
}

func (p *Plugin) ConvertAddress(_ context.Context, _chain, network, addrType, publicKey string) (string, error) {
	if err := p.ensureWriter(); err != nil {
		return "", err
	}
	resp, err := p.adaptor.ConvertAddress(&sourcepb.ConvertAddressRequest{
		Chain:     ChainName,
		Network:   strings.TrimSpace(network),
		Type:      strings.TrimSpace(addrType),
		PublicKey: strings.TrimPrefix(strings.TrimSpace(publicKey), "0x"),
	})
	if err != nil {
		return "", err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetAddress()), nil
}

func (p *Plugin) SendTx(_ context.Context, _chain, network, coin, rawTx string) (string, error) {
	if err := p.ensureWriter(); err != nil {
		return "", err
	}
	resp, err := p.adaptor.SendTx(&sourcepb.SendTxRequest{
		Chain:   ChainName,
		Coin:    strings.TrimSpace(coin),
		Network: strings.TrimSpace(network),
		RawTx:   strings.TrimSpace(rawTx),
	})
	if err != nil {
		return "", err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetTxHash()), nil
}

func (p *Plugin) BuildUnsignedAccount(_ context.Context, _chain, network, base64Tx string) (ports.BuildUnsignedResult, error) {
	if err := p.ensureWriter(); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	resp, err := p.adaptor.BuildUnSignTransaction(&sourcepb.UnSignTransactionRequest{
		Chain:    ChainName,
		Network:  strings.TrimSpace(network),
		Base64Tx: strings.TrimSpace(base64Tx),
	})
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	unsignedTx := strings.TrimSpace(resp.GetUnSignTx())
	if unsignedTx == "" {
		return ports.BuildUnsignedResult{}, fmt.Errorf("empty unsigned tx")
	}
	signHash, err := signHashFromTransactionPayload(unsignedTx)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	return ports.BuildUnsignedResult{
		UnsignedTx: unsignedTx,
		SignHashes: []string{signHash},
	}, nil
}

func (p *Plugin) BuildSignedAccount(_ context.Context, _chain, network, base64Tx, signature, publicKey string) (string, error) {
	if err := p.ensureWriter(); err != nil {
		return "", err
	}
	resp, err := p.adaptor.BuildSignedTransaction(&sourcepb.SignedTransactionRequest{
		Chain:     ChainName,
		Network:   strings.TrimSpace(network),
		Base64Tx:  strings.TrimSpace(base64Tx),
		Signature: strings.TrimSpace(signature),
		PublicKey: strings.TrimSpace(publicKey),
	})
	if err != nil {
		return "", err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetSignedTx()), nil
}

func (p *Plugin) ValidAddress(_ context.Context, _chain, network, _format, address string) (bool, error) {
	if err := p.ensureWriter(); err != nil {
		return false, err
	}
	resp, err := p.adaptor.ValidAddress(&sourcepb.ValidAddressRequest{
		Chain:   ChainName,
		Network: strings.TrimSpace(network),
		Address: strings.TrimSpace(address),
	})
	if err != nil {
		return false, err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return false, err
	}
	return resp.GetValid(), nil
}

func (p *Plugin) GetFee(_ context.Context, in ports.FeeInput) (ports.FeeResult, error) {
	if err := p.ensureWriter(); err != nil {
		return ports.FeeResult{}, err
	}
	resp, err := p.adaptor.GetFee(&sourcepb.FeeRequest{
		Chain:   ChainName,
		Coin:    strings.TrimSpace(in.Coin),
		Network: strings.TrimSpace(in.Network),
		RawTx:   strings.TrimSpace(in.RawTx),
		Address: strings.TrimSpace(in.Address),
	})
	if err != nil {
		return ports.FeeResult{}, err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.FeeResult{}, err
	}
	return ports.FeeResult{
		SlowFee:   strings.TrimSpace(resp.GetSlowFee()),
		NormalFee: strings.TrimSpace(resp.GetNormalFee()),
		FastFee:   strings.TrimSpace(resp.GetFastFee()),
	}, nil
}

func (p *Plugin) GetAccount(_ context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	if err := p.ensureWriter(); err != nil {
		return ports.AccountResult{}, err
	}
	resp, err := p.adaptor.GetAccount(&sourcepb.AccountRequest{
		Chain:           ChainName,
		Coin:            strings.TrimSpace(in.Coin),
		Network:         strings.TrimSpace(in.Network),
		Address:         strings.TrimSpace(in.Address),
		ContractAddress: strings.TrimSpace(in.ContractAddress),
	})
	if err != nil {
		return ports.AccountResult{}, err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
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
	if err := p.ensureWriter(); err != nil {
		return nil, err
	}
	resp, err := p.adaptor.GetTxByHash(&sourcepb.TxHashRequest{
		Chain:   ChainName,
		Coin:    strings.TrimSpace(in.Coin),
		Network: strings.TrimSpace(in.Network),
		Hash:    strings.TrimSpace(in.Hash),
	})
	if err != nil {
		return nil, err
	}
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, err
	}
	return tronMarshalProtoJSON(resp), nil
}

func (p *Plugin) GetTxByAddress(_ context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if err := p.ensureWriter(); err != nil {
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
	resp, err := p.adaptor.GetTxByAddress(&sourcepb.TxAddressRequest{
		Chain:           ChainName,
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
	if err := tronExpectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, err
	}
	return tronMarshalProtoJSON(resp), nil
}

func tronExpectCodeOK(code sourcepbcommon.ReturnCode, msg string) error {
	if code == sourcepbcommon.ReturnCode_SUCCESS {
		return nil
	}
	if strings.TrimSpace(msg) == "" {
		msg = "tron adaptor request failed"
	}
	return fmt.Errorf("%s", strings.TrimSpace(msg))
}

func tronMarshalProtoJSON(v any) json.RawMessage {
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
