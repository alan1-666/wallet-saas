package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

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
		if err != nil {
			continue
		}
		if !finality.Found {
			if strings.EqualFold(strings.TrimSpace(it.AttemptStatus), "REPLACED") {
				continue
			}
			if err := s.handleMissingOutgoingTx(ctx, outgoingKindWithdraw, it.TenantID, it.OrderID, it.Chain, it.Network, it.TxHash, it.RequiredConfs, it.BroadcastedAt); err != nil {
				log.Printf("withdraw missing tx handling failed tenant=%s order=%s tx=%s err=%v", it.TenantID, it.OrderID, it.TxHash, err)
			}
			continue
		}
		_ = s.Store.ClearOutgoingNotFound(ctx, outgoingKindWithdraw, it.TenantID, it.OrderID, it.TxHash)
		status := "CONFIRMED"
		reason := ""
		switch strings.ToUpper(strings.TrimSpace(finality.Status)) {
		case "REVERTED", "FAILED":
			if strings.EqualFold(strings.TrimSpace(it.AttemptStatus), "REPLACED") {
				continue
			}
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
		if err != nil {
			continue
		}
		if !finality.Found {
			if err := s.handleMissingOutgoingTx(ctx, outgoingKindSweep, it.TenantID, it.SweepOrderID, it.Chain, it.Network, it.TxHash, it.RequiredConfs, it.BroadcastedAt); err != nil {
				log.Printf("sweep missing tx handling failed tenant=%s order=%s tx=%s err=%v", it.TenantID, it.SweepOrderID, it.TxHash, err)
			}
			continue
		}
		_ = s.Store.ClearOutgoingNotFound(ctx, outgoingKindSweep, it.TenantID, it.SweepOrderID, it.TxHash)
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

	transfers, err := s.Store.ListPendingTreasuryTransfers(ctx, s.WatchLimit)
	if err != nil {
		return err
	}
	for _, it := range transfers {
		if strings.TrimSpace(it.Network) == "" {
			log.Printf("skip treasury transfer onchain check tenant=%s order=%s tx=%s reason=missing_network", it.TenantID, it.TransferOrderID, it.TxHash)
			continue
		}
		finality, err := s.ChainGateway.TxFinality(ctx, it.Chain, "", it.Network, it.TxHash)
		if err != nil {
			continue
		}
		if !finality.Found {
			if err := s.handleMissingOutgoingTx(ctx, outgoingKindTreasuryTransfer, it.TenantID, it.TransferOrderID, it.Chain, it.Network, it.TxHash, it.RequiredConfs, it.BroadcastedAt); err != nil {
				log.Printf("treasury transfer missing tx handling failed tenant=%s order=%s tx=%s err=%v", it.TenantID, it.TransferOrderID, it.TxHash, err)
			}
			continue
		}
		_ = s.Store.ClearOutgoingNotFound(ctx, outgoingKindTreasuryTransfer, it.TenantID, it.TransferOrderID, it.TxHash)
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
		reqID := fmt.Sprintf("scan-treasury-onchain-%s-%s-%d", it.TenantID, it.TransferOrderID, finality.Confirmations)
		if err := s.WalletCore.TreasuryTransferOnchainNotify(ctx, reqID, client.TreasuryTransferOnchainNotifyRequest{
			TenantID:        it.TenantID,
			TransferOrderID: it.TransferOrderID,
			TxHash:          it.TxHash,
			Status:          status,
			Reason:          reason,
			Confirmations:   finality.Confirmations,
			RequiredConfs:   max(it.RequiredConfs, 1),
		}); err != nil {
			log.Printf("treasury transfer onchain notify failed tenant=%s order=%s tx=%s err=%v", it.TenantID, it.TransferOrderID, it.TxHash, err)
			continue
		}
		log.Printf("treasury transfer onchain notified tenant=%s order=%s tx=%s status=%s conf=%d", it.TenantID, it.TransferOrderID, it.TxHash, status, finality.Confirmations)
	}
	return nil
}

const (
	outgoingKindWithdraw         = "withdraw"
	outgoingKindSweep            = "sweep"
	outgoingKindTreasuryTransfer = "treasury_transfer"
)

func (s *Scanner) handleMissingOutgoingTx(ctx context.Context, kind, tenantID, orderID, chain, network, txHash string, requiredConfs int64, broadcastedAt time.Time) error {
	state, err := s.Store.IncrementOutgoingNotFound(ctx, kind, tenantID, orderID, chain, network, txHash)
	if err != nil {
		return err
	}
	age := time.Since(broadcastedAt)
	log.Printf("outgoing tx not found kind=%s tenant=%s order=%s chain=%s network=%s tx=%s miss=%d age=%s threshold=%d grace=%s",
		kind, tenantID, orderID, chain, network, txHash, state.NotFoundCount, age.Round(time.Second), s.OutgoingNotFoundThreshold, s.OutgoingNotFoundGrace)
	if !shouldFailOutgoingNotFound(chain, age, state.NotFoundCount, s.OutgoingNotFoundThreshold, s.OutgoingNotFoundGrace) {
		return nil
	}
	reason := fmt.Sprintf("onchain tx not found after %d checks and %s since broadcast", state.NotFoundCount, age.Round(time.Second))
	reqID := fmt.Sprintf("scan-%s-notfound-%s-%s-%d", kind, tenantID, orderID, state.NotFoundCount)
	switch kind {
	case outgoingKindSweep:
		if err := s.WalletCore.SweepOnchainNotify(ctx, reqID, client.SweepOnchainNotifyRequest{
			TenantID:      tenantID,
			SweepOrderID:  orderID,
			TxHash:        txHash,
			Status:        "FAILED",
			Reason:        reason,
			Confirmations: 0,
			RequiredConfs: max(requiredConfs, 1),
		}); err != nil {
			return err
		}
		log.Printf("sweep onchain notified tenant=%s order=%s tx=%s status=FAILED reason=%s", tenantID, orderID, txHash, reason)
	case outgoingKindTreasuryTransfer:
		if err := s.WalletCore.TreasuryTransferOnchainNotify(ctx, reqID, client.TreasuryTransferOnchainNotifyRequest{
			TenantID:        tenantID,
			TransferOrderID: orderID,
			TxHash:          txHash,
			Status:          "FAILED",
			Reason:          reason,
			Confirmations:   0,
			RequiredConfs:   max(requiredConfs, 1),
		}); err != nil {
			return err
		}
		log.Printf("treasury transfer onchain notified tenant=%s order=%s tx=%s status=FAILED reason=%s", tenantID, orderID, txHash, reason)
	default:
		if err := s.WalletCore.WithdrawOnchainNotify(ctx, reqID, client.WithdrawOnchainNotifyRequest{
			TenantID:      tenantID,
			OrderID:       orderID,
			TxHash:        txHash,
			Status:        "FAILED",
			Reason:        reason,
			Confirmations: 0,
			RequiredConfs: max(requiredConfs, 1),
		}); err != nil {
			return err
		}
		log.Printf("withdraw onchain notified tenant=%s order=%s tx=%s status=FAILED reason=%s", tenantID, orderID, txHash, reason)
	}
	return s.Store.ClearOutgoingNotFound(ctx, kind, tenantID, orderID, txHash)
}
