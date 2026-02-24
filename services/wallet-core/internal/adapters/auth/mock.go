package auth

import (
	"context"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type MockAuth struct{}

func NewMock() *MockAuth {
	return &MockAuth{}
}

func (m *MockAuth) ValidateToken(_ context.Context, _ string) (ports.AuthScope, error) {
	return ports.AuthScope{
		TenantID:    "*",
		CanWithdraw: true,
		CanDeposit:  true,
		CanSweep:    true,
	}, nil
}

func (m *MockAuth) CheckSignPermission(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (m *MockAuth) BindTenantKey(_ context.Context, _, _ string) error {
	return nil
}

func (m *MockAuth) Audit(_ context.Context, _, _, _, _ string) error {
	return nil
}
