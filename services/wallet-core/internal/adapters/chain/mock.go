package chain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type MockChain struct {
	Balances map[string]string
}

func NewMock() *MockChain {
	return &MockChain{Balances: map[string]string{}}
}

func (m *MockChain) BuildUnsignedTx(_ context.Context, params ports.BuildUnsignedParams) (ports.BuildUnsignedResult, error) {
	raw := params.Chain + ":" + params.From + ":" + params.To + ":" + params.Amount
	h := sha256.Sum256([]byte(raw))
	return ports.BuildUnsignedResult{UnsignedTx: hex.EncodeToString(h[:])}, nil
}

func (m *MockChain) Broadcast(_ context.Context, params ports.BroadcastParams) (string, error) {
	val := params.RawTx + ":" + params.UnsignedTx
	for _, s := range params.Signatures {
		val += ":" + s
	}
	h := sha256.Sum256([]byte(params.Chain + ":" + val))
	return "0x" + hex.EncodeToString(h[:]), nil
}

func (m *MockChain) GetBalance(_ context.Context, _chain, _coin, _network, address, _contractAddress string) (ports.ChainBalance, error) {
	key := address
	if _contractAddress != "" {
		key = address + "|" + _contractAddress
	}
	if m.Balances != nil {
		if balance, ok := m.Balances[key]; ok {
			return ports.ChainBalance{Balance: balance}, nil
		}
		if balance, ok := m.Balances[address]; ok {
			return ports.ChainBalance{Balance: balance}, nil
		}
	}
	return ports.ChainBalance{Balance: "1000000000"}, nil
}
