package solana

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	sourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/config"
	sourcepb "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/account"
	sourcepbcommon "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/common"
	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

// Plugin is the single Solana adapter implementation in v2.
// Read path uses endpoint-manager aware RPCReader, write/build path reuses
// the stable chain adaptor implementation from this package.
type Plugin struct {
	reader  *RPCReader
	adaptor writerAdaptor
	initErr error
}

type writerAdaptor interface {
	ConvertAddress(req *sourcepb.ConvertAddressRequest) (*sourcepb.ConvertAddressResponse, error)
	GetBlockHeaderByNumber(req *sourcepb.BlockHeaderNumberRequest) (*sourcepb.BlockHeaderResponse, error)
	SendTx(req *sourcepb.SendTxRequest) (*sourcepb.SendTxResponse, error)
	BuildUnSignTransaction(req *sourcepb.UnSignTransactionRequest) (*sourcepb.UnSignTransactionResponse, error)
	BuildSignedTransaction(req *sourcepb.SignedTransactionRequest) (*sourcepb.SignedTransactionResponse, error)
	ValidAddress(req *sourcepb.ValidAddressRequest) (*sourcepb.ValidAddressResponse, error)
	GetFee(req *sourcepb.FeeRequest) (*sourcepb.FeeResponse, error)
}

func New(reader *RPCReader, conf *sourcecfg.Config) accountadapter.ChainPlugin {
	adaptor, err := NewChainAdaptor(conf)
	return &Plugin{
		reader:  reader,
		adaptor: adaptor,
		initErr: err,
	}
}

func (p *Plugin) Chain() string {
	return "solana"
}

func (p *Plugin) ensureWriter() error {
	if p.initErr != nil {
		return fmt.Errorf("solana adaptor init failed: %w", p.initErr)
	}
	if p.adaptor == nil {
		return fmt.Errorf("solana adaptor not initialized")
	}
	return nil
}

func normalizeNetwork(network string) string {
	n := strings.ToLower(strings.TrimSpace(network))
	if n == "" {
		return "mainnet"
	}
	return n
}

func normalizeCoin(coin string) string {
	if c := strings.ToUpper(strings.TrimSpace(coin)); c != "" {
		return c
	}
	return "SOL"
}

func toSourceChain(chain string) string {
	if clients.NormalizeChain(chain) == "solana" {
		return ChainName
	}
	return strings.TrimSpace(chain)
}

func expectCodeOK(code sourcepbcommon.ReturnCode, msg string) error {
	if code == sourcepbcommon.ReturnCode_SUCCESS {
		return nil
	}
	if strings.TrimSpace(msg) == "" {
		msg = "solana adaptor request failed"
	}
	return fmt.Errorf("%s", strings.TrimSpace(msg))
}

func (p *Plugin) SupportChains(_ context.Context, chain, _ string) (bool, error) {
	return clients.NormalizeChain(chain) == "solana", nil
}

func (p *Plugin) ConvertAddress(_ context.Context, chain, network, addrType, publicKey string) (string, error) {
	if err := p.ensureWriter(); err != nil {
		return "", err
	}
	resp, err := p.adaptor.ConvertAddress(&sourcepb.ConvertAddressRequest{
		Chain:     toSourceChain(chain),
		Network:   normalizeNetwork(network),
		Type:      strings.TrimSpace(addrType),
		PublicKey: strings.TrimSpace(publicKey),
	})
	if err != nil {
		return "", err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.GetAddress()), nil
}

func (p *Plugin) SendTx(_ context.Context, chain, network, coin, rawTx string) (string, error) {
	if err := p.ensureWriter(); err != nil {
		return "", err
	}
	resp, err := p.adaptor.SendTx(&sourcepb.SendTxRequest{
		Chain:   toSourceChain(chain),
		Coin:    normalizeCoin(coin),
		Network: normalizeNetwork(network),
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

func (p *Plugin) BuildUnsignedAccount(_ context.Context, chain, network, base64Tx string) (ports.BuildUnsignedResult, error) {
	if err := p.ensureWriter(); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	normalizedPayload, err := p.normalizeUnsignedPayload(chain, network, base64Tx)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	resp, err := p.adaptor.BuildUnSignTransaction(&sourcepb.UnSignTransactionRequest{
		Chain:    toSourceChain(chain),
		Network:  normalizeNetwork(network),
		Base64Tx: normalizedPayload,
	})
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	signHash := strings.TrimSpace(resp.GetUnSignTx())
	if signHash == "" {
		return ports.BuildUnsignedResult{}, fmt.Errorf("empty sign hash")
	}
	return ports.BuildUnsignedResult{
		UnsignedTx: normalizedPayload,
		SignHashes: []string{signHash},
	}, nil
}

func (p *Plugin) BuildSignedAccount(_ context.Context, chain, network, base64Tx, signature, publicKey string) (string, error) {
	if err := p.ensureWriter(); err != nil {
		return "", err
	}
	resp, err := p.adaptor.BuildSignedTransaction(&sourcepb.SignedTransactionRequest{
		Chain:     toSourceChain(chain),
		Network:   normalizeNetwork(network),
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

func (p *Plugin) ValidAddress(_ context.Context, chain, network, format, address string) (bool, error) {
	_ = format
	if err := p.ensureWriter(); err != nil {
		return false, err
	}
	resp, err := p.adaptor.ValidAddress(&sourcepb.ValidAddressRequest{
		Chain:   toSourceChain(chain),
		Network: normalizeNetwork(network),
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
	if err := p.ensureWriter(); err != nil {
		return ports.FeeResult{}, err
	}
	resp, err := p.adaptor.GetFee(&sourcepb.FeeRequest{
		Chain:   toSourceChain(in.Chain),
		Coin:    normalizeCoin(in.Coin),
		Network: normalizeNetwork(in.Network),
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

func (p *Plugin) normalizeUnsignedPayload(chain, network, base64Tx string) (string, error) {
	payload := strings.TrimSpace(base64Tx)
	if payload == "" {
		return "", fmt.Errorf("base64_tx is required")
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("invalid base64_tx: %w", err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", fmt.Errorf("empty tx payload")
	}

	var tx TxStructure
	if err := json.Unmarshal(raw, &tx); err != nil {
		parts := strings.Split(text, ":")
		// Legacy wallet-core payload: chain:from:to:amount
		if len(parts) != 4 {
			return "", fmt.Errorf("unsupported tx payload format")
		}
		tx = TxStructure{
			FromAddress: strings.TrimSpace(parts[1]),
			ToAddress:   strings.TrimSpace(parts[2]),
			Value:       strings.TrimSpace(parts[3]),
		}
	}

	if strings.TrimSpace(tx.FromAddress) == "" || strings.TrimSpace(tx.ToAddress) == "" || strings.TrimSpace(tx.Value) == "" {
		return "", fmt.Errorf("invalid tx payload")
	}
	if strings.TrimSpace(tx.Nonce) == "" {
		nonce, err := p.fetchLatestBlockhash(chain, network)
		if err != nil {
			return "", err
		}
		tx.Nonce = nonce
	}
	out, err := json.Marshal(tx)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func (p *Plugin) fetchLatestBlockhash(chain, network string) (string, error) {
	resp, err := p.adaptor.GetBlockHeaderByNumber(&sourcepb.BlockHeaderNumberRequest{
		Chain:   toSourceChain(chain),
		Network: normalizeNetwork(network),
		Height:  0,
	})
	if err != nil {
		return "", err
	}
	if err := expectCodeOK(resp.GetCode(), resp.GetMsg()); err != nil {
		return "", err
	}
	header := resp.GetBlockHeader()
	if header == nil {
		return "", fmt.Errorf("empty block header")
	}
	nonce := strings.TrimSpace(header.GetHash())
	if nonce == "" {
		return "", fmt.Errorf("empty block hash")
	}
	return nonce, nil
}

func (p *Plugin) ensureReader() error {
	if p.reader == nil {
		return fmt.Errorf("solana reader not initialized")
	}
	return nil
}

func (p *Plugin) GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	if err := p.ensureReader(); err != nil {
		return ports.AccountResult{}, err
	}
	var (
		out ports.BalanceResult
		err error
	)
	if strings.TrimSpace(in.ContractAddress) != "" && !isSOLTransfer(strings.TrimSpace(in.ContractAddress)) {
		out, err = p.reader.GetTokenBalance(ctx, in.Chain, in.Network, in.Address, in.ContractAddress)
	} else {
		out, err = p.reader.GetBalance(ctx, in.Chain, in.Network, in.Address)
	}
	if err != nil {
		return ports.AccountResult{}, err
	}
	return ports.AccountResult{
		Network:       out.Network,
		AccountNumber: "0",
		Sequence:      out.Sequence,
		Balance:       out.Balance,
	}, nil
}

func (p *Plugin) GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if err := p.ensureReader(); err != nil {
		return nil, err
	}
	out, err := p.reader.GetTxFinality(ctx, in.Chain, in.Network, in.Hash)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"tx_hash":       out.TxHash,
		"confirmations": out.Confirmations,
		"status":        out.Status,
		"found":         out.Found,
	}
	return json.Marshal(payload)
}

func (p *Plugin) GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if err := p.ensureReader(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Address) == "" {
		return nil, fmt.Errorf("address is required")
	}
	out, err := p.reader.ListIncomingTransfers(ctx, in.Chain, in.Network, in.Address, in.Cursor, in.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(out.Items))
	for idx, item := range out.Items {
		items = append(items, map[string]any{
			"hash":          item.TxHash,
			"from":          item.FromAddress,
			"to":            item.ToAddress,
			"value":         item.Amount,
			"index":         fallbackIndex(item.Index, idx),
			"confirmations": item.Confirmations,
			"status":        item.Status,
		})
	}
	payload := map[string]any{
		"tx":          items,
		"next_cursor": out.NextCursor,
	}
	return json.Marshal(payload)
}

func fallbackIndex(v int64, idx int) int64 {
	if v > 0 {
		return v
	}
	return int64(idx)
}
