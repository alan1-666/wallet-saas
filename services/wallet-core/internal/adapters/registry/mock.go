package registry

import (
	"context"
	"fmt"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type MockRegistry struct{}

func NewMock() *MockRegistry {
	return &MockRegistry{}
}

func (m *MockRegistry) UpsertWatchAddress(_ context.Context, _ ports.WatchAddressInput) error {
	return nil
}

func (m *MockRegistry) UpsertAccount(_ context.Context, in ports.WalletAccount) (ports.WalletAccount, error) {
	if in.Status == "" {
		in.Status = "ACTIVE"
	}
	return in, nil
}

func (m *MockRegistry) GetAccount(_ context.Context, tenantID, accountID string) (ports.WalletAccount, error) {
	if tenantID == "" || accountID == "" {
		return ports.WalletAccount{}, fmt.Errorf("tenant_id/account_id is required")
	}
	return ports.WalletAccount{
		TenantID:  tenantID,
		AccountID: accountID,
		Status:    "ACTIVE",
	}, nil
}

func (m *MockRegistry) ListAccounts(_ context.Context, tenantID string, _ int, _ int) ([]ports.WalletAccount, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	return []ports.WalletAccount{
		{TenantID: tenantID, AccountID: "mock-account", Status: "ACTIVE"},
	}, nil
}

func (m *MockRegistry) ListAccountAddresses(_ context.Context, tenantID, accountID string) ([]ports.WalletAddress, error) {
	if tenantID == "" || accountID == "" {
		return nil, fmt.Errorf("tenant_id/account_id is required")
	}
	return []ports.WalletAddress{}, nil
}
