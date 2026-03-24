package ledger

import (
	"context"
	"time"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type MockLedger struct{}

func NewMock() *MockLedger {
	return &MockLedger{}
}

func (m *MockLedger) FreezeWithdraw(_ context.Context, _, _, _, _, _, _, _ string, _ int64) error {
	return nil
}

func (m *MockLedger) QueueWithdraw(_ context.Context, _ ports.WithdrawQueueInput) error {
	return nil
}

func (m *MockLedger) ClaimQueuedWithdraws(_ context.Context, _ int) ([]ports.WithdrawJob, error) {
	return nil, nil
}

func (m *MockLedger) MarkQueuedWithdrawDone(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *MockLedger) RescheduleQueuedWithdraw(_ context.Context, _, _, _ string, _ time.Duration) error {
	return nil
}

func (m *MockLedger) MarkQueuedWithdrawFailed(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *MockLedger) ClaimStaleBroadcastedWithdraws(_ context.Context, _ int, _ time.Duration, _ int) ([]ports.WithdrawJob, error) {
	return nil, nil
}

func (m *MockLedger) ReplaceBroadcastedWithdraw(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

func (m *MockLedger) ResetBroadcastedWithdraw(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *MockLedger) ConfirmWithdraw(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *MockLedger) ConfirmWithdrawOnChain(_ context.Context, _, _, _ string, _, _ int64) error {
	return nil
}

func (m *MockLedger) FailWithdrawOnChain(_ context.Context, _, _, _, _ string, _ int64) error {
	return nil
}

func (m *MockLedger) ReleaseWithdraw(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *MockLedger) GetWithdrawStatus(_ context.Context, _, _ string) (ports.LedgerStatus, error) {
	return ports.LedgerStatus{Status: "MOCK", QueueStatus: "MOCK"}, nil
}

func (m *MockLedger) CreditDeposit(_ context.Context, _ ports.DepositCreditInput) error {
	return nil
}

func (m *MockLedger) ReserveSweep(_ context.Context, _ ports.SweepReserveInput) error {
	return nil
}

func (m *MockLedger) StartSweep(_ context.Context, _ ports.SweepCollectInput) error {
	return nil
}

func (m *MockLedger) ConfirmSweepOnChain(_ context.Context, _ ports.SweepConfirmInput) error {
	return nil
}

func (m *MockLedger) FailSweepOnChain(_ context.Context, _, _, _ string, _ int64) error {
	return nil
}

func (m *MockLedger) ReserveTreasuryTransfer(_ context.Context, _ ports.TreasuryTransferReserveInput) error {
	return nil
}

func (m *MockLedger) MarkTreasuryTransferBroadcasted(_ context.Context, _, _, _ string, _ int64) error {
	return nil
}

func (m *MockLedger) ConfirmTreasuryTransferOnChain(_ context.Context, _ ports.TreasuryTransferConfirmInput) error {
	return nil
}

func (m *MockLedger) FailTreasuryTransferOnChain(_ context.Context, _, _, _ string, _ int64) error {
	return nil
}

func (m *MockLedger) GetTreasuryTransferStatus(_ context.Context, _, _ string) (ports.TreasuryTransferStatus, error) {
	return ports.TreasuryTransferStatus{Status: "MOCK"}, nil
}

func (m *MockLedger) GetBalance(_ context.Context, _, _, _ string) (ports.BalanceSnapshot, error) {
	return ports.BalanceSnapshot{Available: "0", Frozen: "0", WithdrawLocked: "0", Withdrawable: "0"}, nil
}

func (m *MockLedger) ListAccountAssets(_ context.Context, _, _ string) ([]ports.AccountAsset, error) {
	return []ports.AccountAsset{}, nil
}
