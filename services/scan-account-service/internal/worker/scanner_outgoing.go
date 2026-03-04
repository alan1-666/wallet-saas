package worker

import (
	"context"
	"fmt"
	"log"
	"strings"

	"wallet-saas-v2/services/scan-account-service/internal/client"
)

func (s *Scanner) scanOutgoingConfirmations(ctx context.Context) error {
	withdraws, err := s.Store.ListPendingWithdraws(ctx, s.WatchLimit)
	if err != nil {
		return err
	}
	for _, it := range withdraws {
		if strings.TrimSpace(it.Network) == "" {
			log.Printf("skip withdraw onchain check tenant=%s order=%s tx=%s reason=missing_network", it.TenantID, it.OrderID, it.TxHash)
			continue
		}
		finality, err := s.ChainGateway.TxFinality(ctx, it.Chain, "", it.Network, it.TxHash)
		if err != nil || !finality.Found {
			continue
		}
		status := "CONFIRMED"
		reason := ""
		switch strings.ToUpper(strings.TrimSpace(finality.Status)) {
		case "REVERTED", "FAILED":
			status = "FAILED"
			reason = "onchain tx reverted"
		case "PENDING":
			continue
		default:
			status = "CONFIRMED"
		}
		reqID := fmt.Sprintf("scan-withdraw-onchain-%s-%s-%d", it.TenantID, it.OrderID, finality.Confirmations)
		if err := s.WalletCore.WithdrawOnchainNotify(ctx, reqID, client.WithdrawOnchainNotifyRequest{
			TenantID:      it.TenantID,
			OrderID:       it.OrderID,
			TxHash:        it.TxHash,
			Status:        status,
			Reason:        reason,
			Confirmations: finality.Confirmations,
			RequiredConfs: max(it.RequiredConfs, 1),
		}); err != nil {
			log.Printf("withdraw onchain notify failed tenant=%s order=%s tx=%s err=%v", it.TenantID, it.OrderID, it.TxHash, err)
			continue
		}
		log.Printf("withdraw onchain notified tenant=%s order=%s tx=%s status=%s conf=%d", it.TenantID, it.OrderID, it.TxHash, status, finality.Confirmations)
	}

	sweeps, err := s.Store.ListPendingSweeps(ctx, s.WatchLimit)
	if err != nil {
		return err
	}
	for _, it := range sweeps {
		if strings.TrimSpace(it.Network) == "" {
			log.Printf("skip sweep onchain check tenant=%s order=%s tx=%s reason=missing_network", it.TenantID, it.SweepOrderID, it.TxHash)
			continue
		}
		finality, err := s.ChainGateway.TxFinality(ctx, it.Chain, "", it.Network, it.TxHash)
		if err != nil || !finality.Found {
			continue
		}
		status := "CONFIRMED"
		reason := ""
		switch strings.ToUpper(strings.TrimSpace(finality.Status)) {
		case "REVERTED", "FAILED":
			status = "FAILED"
			reason = "onchain tx reverted"
		case "PENDING":
			continue
		default:
			status = "CONFIRMED"
		}
		reqID := fmt.Sprintf("scan-sweep-onchain-%s-%s-%d", it.TenantID, it.SweepOrderID, finality.Confirmations)
		if err := s.WalletCore.SweepOnchainNotify(ctx, reqID, client.SweepOnchainNotifyRequest{
			TenantID:      it.TenantID,
			SweepOrderID:  it.SweepOrderID,
			TxHash:        it.TxHash,
			Status:        status,
			Reason:        reason,
			Confirmations: finality.Confirmations,
			RequiredConfs: max(it.RequiredConfs, 1),
		}); err != nil {
			log.Printf("sweep onchain notify failed tenant=%s order=%s tx=%s err=%v", it.TenantID, it.SweepOrderID, it.TxHash, err)
			continue
		}
		log.Printf("sweep onchain notified tenant=%s order=%s tx=%s status=%s conf=%d", it.TenantID, it.SweepOrderID, it.TxHash, status, finality.Confirmations)
	}
	return nil
}
