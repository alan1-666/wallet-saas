package utxo

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	sourcechain "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains"
	"wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/rpc/common"
	"wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/rpc/utxo"
	"wallet-saas-v2/services/chain-gateway/internal/ports"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type BridgePlugin struct {
	BasePlugin
	chain           string
	sourceChainName string
	adapter         sourcechain.IChainAdaptor
	initErr         error
}

func NewBridgePlugin(chainName string, sourceChainName string, adapter sourcechain.IChainAdaptor, initErr error) *BridgePlugin {
	return &BridgePlugin{
		chain:           strings.ToLower(strings.TrimSpace(chainName)),
		sourceChainName: strings.TrimSpace(sourceChainName),
		adapter:         adapter,
		initErr:         initErr,
	}
}

func (p *BridgePlugin) Chain() string { return p.chain }

func (p *BridgePlugin) ensureReady() error {
	if p.initErr != nil {
		return fmt.Errorf("utxo chain plugin init failed chain=%s: %w", p.chain, p.initErr)
	}
	if p.adapter == nil {
		return fmt.Errorf("utxo chain plugin not initialized chain=%s", p.chain)
	}
	return nil
}

func (p *BridgePlugin) SupportChains(_ context.Context, chain, _ string) (bool, error) {
	return strings.EqualFold(strings.TrimSpace(chain), p.chain), nil
}

func (p *BridgePlugin) ConvertAddress(_ context.Context, _chain, network, addrType, publicKey string) (string, error) {
	if err := p.ensureReady(); err != nil {
		return "", err
	}
	resp, err := p.adapter.ConvertAddress(&utxo.ConvertAddressRequest{
		Chain:     p.sourceChainName,
		Network:   strings.TrimSpace(network),
		Format:    strings.TrimSpace(addrType),
		PublicKey: strings.TrimPrefix(strings.TrimSpace(publicKey), "0x"),
	})
	if err != nil {
		return "", err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetAddress()), nil
}

func (p *BridgePlugin) SendTx(_ context.Context, _chain, network, coin, rawTx string) (string, error) {
	if err := p.ensureReady(); err != nil {
		return "", err
	}
	resp, err := p.adapter.SendTx(&utxo.SendTxRequest{
		Chain:   p.sourceChainName,
		Coin:    strings.TrimSpace(coin),
		Network: strings.TrimSpace(network),
		RawTx:   strings.TrimSpace(rawTx),
	})
	if err != nil {
		return "", err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetTxHash()), nil
}

func (p *BridgePlugin) BuildUnsignedUTXO(_ context.Context, _chain, network, fee string, vin []ports.TxVin, vout []ports.TxVout) (ports.BuildUnsignedResult, error) {
	if err := p.ensureReady(); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	reqVin := make([]*utxo.Vin, 0, len(vin))
	for _, item := range vin {
		reqVin = append(reqVin, &utxo.Vin{
			Hash:    strings.TrimSpace(item.Hash),
			Index:   item.Index,
			Amount:  item.Amount,
			Address: strings.TrimSpace(item.Address),
		})
	}
	reqVout := make([]*utxo.Vout, 0, len(vout))
	for _, item := range vout {
		reqVout = append(reqVout, &utxo.Vout{
			Address: strings.TrimSpace(item.Address),
			Amount:  item.Amount,
			Index:   item.Index,
		})
	}
	resp, err := p.adapter.CreateUnSignTransaction(&utxo.UnSignTransactionRequest{
		Chain:   p.sourceChainName,
		Network: strings.TrimSpace(network),
		Fee:     strings.TrimSpace(fee),
		Vin:     reqVin,
		Vout:    reqVout,
	})
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	hashes := make([]string, 0, len(resp.GetSignHashes()))
	for _, hash := range resp.GetSignHashes() {
		hashes = append(hashes, hex.EncodeToString(hash))
	}
	return ports.BuildUnsignedResult{
		UnsignedTx: string(resp.GetTxData()),
		SignHashes: hashes,
	}, nil
}

func (p *BridgePlugin) BuildSignedUTXO(_ context.Context, _chain, network string, txData []byte, signatures [][]byte, publicKeys [][]byte) ([]byte, string, error) {
	if err := p.ensureReady(); err != nil {
		return nil, "", err
	}
	resp, err := p.adapter.BuildSignedTransaction(&utxo.SignedTransactionRequest{
		Chain:      p.sourceChainName,
		Network:    strings.TrimSpace(network),
		TxData:     txData,
		Signatures: signatures,
		PublicKeys: publicKeys,
	})
	if err != nil {
		return nil, "", err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, "", err
	}
	return resp.GetSignedTxData(), hex.EncodeToString(resp.GetHash()), nil
}

func (p *BridgePlugin) ValidAddress(_ context.Context, _chain, network, format, address string) (bool, error) {
	if err := p.ensureReady(); err != nil {
		return false, err
	}
	resp, err := p.adapter.ValidAddress(&utxo.ValidAddressRequest{
		Chain:   p.sourceChainName,
		Network: strings.TrimSpace(network),
		Format:  strings.TrimSpace(format),
		Address: strings.TrimSpace(address),
	})
	if err != nil {
		return false, err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return false, err
	}
	return resp.GetValid(), nil
}

func (p *BridgePlugin) GetFee(_ context.Context, in ports.FeeInput) (ports.FeeResult, error) {
	if err := p.ensureReady(); err != nil {
		return ports.FeeResult{}, err
	}
	resp, err := p.adapter.GetFee(&utxo.FeeRequest{
		Chain:   p.sourceChainName,
		Coin:    strings.TrimSpace(in.Coin),
		Network: strings.TrimSpace(in.Network),
		RawTx:   strings.TrimSpace(in.RawTx),
	})
	if err != nil {
		return ports.FeeResult{}, err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.FeeResult{}, err
	}
	return ports.FeeResult{
		BestFee:    strings.TrimSpace(resp.GetBestFee()),
		BestFeeSat: strings.TrimSpace(resp.GetBestFeeSat()),
		SlowFee:    strings.TrimSpace(resp.GetSlowFee()),
		NormalFee:  strings.TrimSpace(resp.GetNormalFee()),
		FastFee:    strings.TrimSpace(resp.GetFastFee()),
	}, nil
}

func (p *BridgePlugin) GetAccount(_ context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	if err := p.ensureReady(); err != nil {
		return ports.AccountResult{}, err
	}
	resp, err := p.adapter.GetAccount(&utxo.AccountRequest{
		Chain:        p.sourceChainName,
		Network:      strings.TrimSpace(in.Network),
		Address:      strings.TrimSpace(in.Address),
		Brc20Address: strings.TrimSpace(in.ContractAddress),
	})
	if err != nil {
		return ports.AccountResult{}, err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.AccountResult{}, err
	}
	return ports.AccountResult{
		Network: strings.TrimSpace(resp.GetNetwork()),
		Balance: strings.TrimSpace(resp.GetBalance()),
	}, nil
}

func (p *BridgePlugin) GetTxByHash(_ context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if err := p.ensureReady(); err != nil {
		return nil, err
	}
	resp, err := p.adapter.GetTxByHash(&utxo.TxHashRequest{
		Chain:   p.sourceChainName,
		Coin:    strings.TrimSpace(in.Coin),
		Network: strings.TrimSpace(in.Network),
		Hash:    strings.TrimSpace(in.Hash),
	})
	if err != nil {
		return nil, err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, err
	}
	return marshalProtoJSON(resp), nil
}

func (p *BridgePlugin) GetTxByAddress(_ context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
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
	resp, err := p.adapter.GetTxByAddress(&utxo.TxAddressRequest{
		Chain:    p.sourceChainName,
		Coin:     strings.TrimSpace(in.Coin),
		Network:  strings.TrimSpace(in.Network),
		Address:  strings.TrimSpace(in.Address),
		Page:     page,
		Pagesize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, err
	}
	return marshalProtoJSON(resp), nil
}

func (p *BridgePlugin) GetUnspentOutputs(_ context.Context, _chain, network, address string) (json.RawMessage, error) {
	if err := p.ensureReady(); err != nil {
		return nil, err
	}
	resp, err := p.adapter.GetUnspentOutputs(&utxo.UnspentOutputsRequest{
		Chain:   p.sourceChainName,
		Network: strings.TrimSpace(network),
		Address: strings.TrimSpace(address),
	})
	if err != nil {
		return nil, err
	}
	if err := expectBridgeCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return nil, err
	}
	return marshalProtoJSON(resp), nil
}

func expectBridgeCodeOK(code common.ReturnCode, msg string) error {
	if code == common.ReturnCode_SUCCESS {
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
