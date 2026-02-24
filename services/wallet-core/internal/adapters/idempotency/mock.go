package idempotency

import (
	"context"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type MockStore struct{}

func NewMock() *MockStore {
	return &MockStore{}
}

func (m *MockStore) Reserve(_ context.Context, _, requestID, _, _ string) (ports.IdemResult, error) {
	return ports.IdemResult{State: "NEW", RequestID: requestID}, nil
}

func (m *MockStore) Commit(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *MockStore) Reject(_ context.Context, _, _, _, _ string) error {
	return nil
}
