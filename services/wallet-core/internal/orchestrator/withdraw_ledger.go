package orchestrator

import (
	"context"
	"fmt"
)

func (o *WithdrawOrchestrator) checkRisk(ctx context.Context, req WithdrawRequest) error {
	decision, err := o.Risk.CheckWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, req.Tx.Chain, req.Tx.Coin, req.Tx.Amount)
	if err != nil {
		return err
	}
	if decision != "ALLOW" {
		return fmt.Errorf("withdraw blocked by risk: %s", decision)
	}
	return nil
}

func (o *WithdrawOrchestrator) freezeWithdraw(ctx context.Context, req WithdrawRequest) error {
	requiredConfs := req.RequiredConfs
	if requiredConfs <= 0 {
		requiredConfs = 1
	}
	return o.Ledger.FreezeWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, req.Tx.Chain, req.Tx.Network, req.Tx.Coin, req.Tx.Amount, requiredConfs)
}

func (o *WithdrawOrchestrator) releaseWithdraw(ctx context.Context, req WithdrawRequest, reason string) error {
	return o.Ledger.ReleaseWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, reason)
}

func (o *WithdrawOrchestrator) confirmWithdraw(ctx context.Context, req WithdrawRequest, txHash string) error {
	return o.Ledger.ConfirmWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, txHash)
}
