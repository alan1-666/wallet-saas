package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wallet-saas-v2/services/chain-gateway/internal/normalize"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

func (s *ChainService) ListIncomingTransfers(ctx context.Context, in ports.IncomingTransferInput) (ports.IncomingTransferResult, error) {
	if err := validateChainNetwork(in.Chain, in.Network); err != nil {
		return ports.IncomingTransferResult{}, err
	}
	binding, err := s.Router.Resolve(in.Chain, in.Network)
	if err != nil {
		return ports.IncomingTransferResult{}, err
	}
	if in.Model != "" && in.Model != binding.Model {
		return ports.IncomingTransferResult{}, fmt.Errorf("model mismatch chain=%s network=%s require=%s actual=%s", in.Chain, in.Network, in.Model, binding.Model)
	}
	if strings.TrimSpace(in.Address) == "" && !allowEmptyAddressScan(binding.Model, binding.Chain) {
		return ports.IncomingTransferResult{}, fmt.Errorf("address is required")
	}
	raw, err := s.fetchIncomingRaw(ctx, binding, in)
	if err != nil {
		return ports.IncomingTransferResult{}, err
	}
	return normalize.IncomingTransfers(binding.Model, binding.Chain, raw, in.Address), nil
}

func (s *ChainService) GetTxFinality(ctx context.Context, in ports.TxFinalityInput) (ports.TxFinality, error) {
	if err := validateChainNetwork(in.Chain, in.Network); err != nil {
		return ports.TxFinality{}, err
	}
	if strings.TrimSpace(in.TxHash) == "" {
		return ports.TxFinality{}, fmt.Errorf("tx_hash is required")
	}
	binding, err := s.Router.Resolve(in.Chain, in.Network)
	if err != nil {
		return ports.TxFinality{}, err
	}
	rawMsg, err := withRetry(ctx, "get-tx-by-hash", func() (json.RawMessage, error) {
		return binding.Adapter.GetTxByHash(ctx, ports.TxQueryInput{
			Chain:   binding.Chain,
			Coin:    in.Coin,
			Network: in.Network,
			Hash:    in.TxHash,
		})
	})
	if err != nil {
		return ports.TxFinality{}, err
	}
	raw, err := decodeRawObject(rawMsg)
	if err != nil {
		return ports.TxFinality{}, err
	}
	out := normalize.Finality(in.TxHash, raw)
	if strings.TrimSpace(out.TxHash) == "" {
		out.TxHash = in.TxHash
	}
	return out, nil
}

func (s *ChainService) GetBalance(ctx context.Context, in ports.BalanceInput) (ports.BalanceResult, error) {
	if err := validateChainNetwork(in.Chain, in.Network); err != nil {
		return ports.BalanceResult{}, err
	}
	if strings.TrimSpace(in.Address) == "" {
		return ports.BalanceResult{}, fmt.Errorf("address is required")
	}
	binding, err := s.Router.Resolve(in.Chain, in.Network)
	if err != nil {
		return ports.BalanceResult{}, err
	}
	acc, err := withRetry(ctx, "get-account", func() (ports.AccountResult, error) {
		return binding.Adapter.GetAccount(ctx, ports.AccountInput{
			Chain:           binding.Chain,
			Coin:            in.Coin,
			Network:         in.Network,
			Address:         in.Address,
			ContractAddress: in.ContractAddress,
		})
	})
	if err != nil {
		return ports.BalanceResult{}, err
	}
	return ports.BalanceResult{
		Balance:  acc.Balance,
		Network:  acc.Network,
		Sequence: acc.Sequence,
	}, nil
}

func (s *ChainService) fetchIncomingRaw(ctx context.Context, binding ports.PluginBinding, in ports.IncomingTransferInput) (map[string]any, error) {
	switch binding.Model {
	case ports.ModelUTXO:
		rawMsg, err := withRetry(ctx, "get-unspent-outputs", func() (json.RawMessage, error) {
			return binding.Adapter.GetUnspentOutputs(ctx, binding.Chain, in.Network, in.Address)
		})
		if err != nil {
			return nil, err
		}
		return decodeRawObject(rawMsg)
	default:
		page := in.Page
		if page == 0 {
			page = 1
		}
		pageSize := in.PageSize
		if pageSize == 0 {
			pageSize = 50
		}
		rawMsg, err := withRetry(ctx, "get-tx-by-address", func() (json.RawMessage, error) {
			return binding.Adapter.GetTxByAddress(ctx, ports.TxQueryInput{
				Chain:    binding.Chain,
				Coin:     in.Coin,
				Network:  in.Network,
				Address:  in.Address,
				Page:     page,
				PageSize: pageSize,
				Cursor:   in.Cursor,
			})
		})
		if err != nil {
			return nil, err
		}
		return decodeRawObject(rawMsg)
	}
}

func decodeRawObject(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func allowEmptyAddressScan(model ports.ChainModel, chain string) bool {
	if model != ports.ModelAccount {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "ethereum", "binance", "polygon", "arbitrum", "optimism", "linea", "scroll", "mantle", "zksync", "base", "avalanche":
		return true
	default:
		return false
	}
}
