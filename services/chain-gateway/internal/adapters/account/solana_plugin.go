package account

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"

	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

type SolanaReader interface {
	ListIncomingTransfers(ctx context.Context, chain, network, address, cursor string, pageSize uint32) (ports.IncomingTransferResult, error)
	GetTxFinality(ctx context.Context, chain, network, txHash string) (ports.TxFinality, error)
	GetBalance(ctx context.Context, chain, network, address string) (ports.BalanceResult, error)
}

type SolanaPlugin struct {
	BasePlugin
	Reader SolanaReader
}

func NewSolanaPlugin(reader SolanaReader) *SolanaPlugin {
	return &SolanaPlugin{Reader: reader}
}

func (p *SolanaPlugin) Chain() string { return "solana" }

func (p *SolanaPlugin) SupportChains(_ context.Context, chain, _ string) (bool, error) {
	return clients.NormalizeChain(chain) == "solana", nil
}

func (p *SolanaPlugin) ConvertAddress(_ context.Context, chain, _network, _addrType, publicKey string) (string, error) {
	if clients.NormalizeChain(chain) != "solana" {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}
	pubHex := strings.TrimSpace(strings.TrimPrefix(publicKey, "0x"))
	if pubHex == "" {
		return "", fmt.Errorf("public key is required")
	}
	b, err := hex.DecodeString(pubHex)
	if err != nil {
		return "", fmt.Errorf("invalid public key: %w", err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("invalid public key length=%d", len(b))
	}
	return solana.PublicKeyFromBytes(b).String(), nil
}

func (p *SolanaPlugin) ValidAddress(_ context.Context, chain, _network, _format, address string) (bool, error) {
	if clients.NormalizeChain(chain) != "solana" {
		return false, nil
	}
	_, err := solana.PublicKeyFromBase58(strings.TrimSpace(address))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (p *SolanaPlugin) GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	if p.Reader == nil {
		return ports.AccountResult{}, fmt.Errorf("solana reader is nil")
	}
	out, err := p.Reader.GetBalance(ctx, in.Chain, in.Network, in.Address)
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

func (p *SolanaPlugin) GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if p.Reader == nil {
		return nil, fmt.Errorf("solana reader is nil")
	}
	out, err := p.Reader.GetTxFinality(ctx, in.Chain, in.Network, in.Hash)
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

func (p *SolanaPlugin) GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if p.Reader == nil {
		return nil, fmt.Errorf("solana reader is nil")
	}
	out, err := p.Reader.ListIncomingTransfers(ctx, in.Chain, in.Network, in.Address, in.Cursor, in.PageSize)
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
