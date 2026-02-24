package risk

import (
	"context"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type MockRisk struct{}

func NewMock() *MockRisk {
	return &MockRisk{}
}

func (m *MockRisk) CheckWithdraw(_ context.Context, _, _, _, _, _, _ string) (string, error) {
	return "ALLOW", nil
}

func (m *MockRisk) GetWithdrawDecision(_ context.Context, _, _ string) (ports.RiskDecision, error) {
	return ports.RiskDecision{Decision: "ALLOW"}, nil
}
