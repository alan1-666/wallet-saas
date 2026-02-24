package chain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type MockChain struct{}

func NewMock() *MockChain {
	return &MockChain{}
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
