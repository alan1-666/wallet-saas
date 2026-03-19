package worker

import (
	"context"
	"log"
	"strings"

	"wallet-saas-v2/services/scan-account-service/internal/store"
)

func (s *Scanner) reconcileReorgCandidates(ctx context.Context) error {
	limit := s.ReorgCandidateLimit
	if limit <= 0 {
		limit = s.WatchLimit
	}
	candidates, err := s.Store.ListReorgCandidates(ctx, s.ReorgWindow, limit)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	for _, c := range candidates {
		if strings.TrimSpace(c.Network) == "" {
			continue
		}
		minConf := c.MinConfirmations
		if minConf <= 0 {
			minConf = 1
		}

		finality, err := s.ChainGateway.TxFinality(ctx, c.Chain, c.Coin, c.Network, c.TxHash)
		if err != nil {
			log.Printf("reorg candidate finality failed tenant=%s account=%s tx=%s chain=%s network=%s err=%v",
				c.TenantID, c.AccountID, c.TxHash, c.Chain, c.Network, err)
			continue
		}

		nextStatus := strings.ToUpper(strings.TrimSpace(c.Status))
		nextConf := c.Confirmations
		if !finality.Found {
			miss, err := s.Store.IncrementSeenEventNotFound(ctx, c)
			if err != nil {
				log.Printf("reorg candidate mark-not-found failed tenant=%s account=%s tx=%s err=%v", c.TenantID, c.AccountID, c.TxHash, err)
				continue
			}
			log.Printf("reorg candidate tx missing tenant=%s account=%s tx=%s miss=%d threshold=%d",
				c.TenantID, c.AccountID, c.TxHash, miss, s.ReorgNotFoundThreshold)
			if miss < s.ReorgNotFoundThreshold {
				continue
			}
			nextStatus = depositScanStatusReorged
			nextConf = 0
		} else {
			nextStatus = resolveDepositScanStatus(finality.Status, finality.Confirmations, minConf, s.ReorgWindow)
			nextConf = finality.Confirmations
			if c.NotFoundCount > 0 {
				if err := s.Store.ResetSeenEventNotFound(ctx, c); err != nil {
					log.Printf("reorg candidate clear-not-found failed tenant=%s account=%s tx=%s err=%v", c.TenantID, c.AccountID, c.TxHash, err)
					continue
				}
			}
		}

		w := buildWatchFromCandidate(c, minConf)
		change, err := s.Store.UpsertSeenEvent(ctx, w, c.TxHash, c.EventIndex, nextStatus, nextConf, c.Amount, c.FromAddress, fallback(c.ToAddress, c.Address))
		if err != nil {
			log.Printf("reorg candidate upsert failed tenant=%s account=%s tx=%s err=%v", c.TenantID, c.AccountID, c.TxHash, err)
			continue
		}
		if !change.Notify {
			continue
		}

		if err := s.enqueueDepositEvent(
			ctx,
			w,
			c.TxHash,
			c.EventIndex,
			c.Amount,
			c.FromAddress,
			fallback(c.ToAddress, c.Address),
			nextConf,
			minConf,
			nextStatus,
		); err != nil {
			log.Printf("reorg candidate enqueue failed tenant=%s account=%s tx=%s status=%s err=%v", c.TenantID, c.AccountID, c.TxHash, nextStatus, err)
			continue
		}
		log.Printf("reorg candidate reconciled tenant=%s account=%s tx=%s old_status=%s new_status=%s old_conf=%d new_conf=%d",
			c.TenantID, c.AccountID, c.TxHash, change.OldStatus, change.NewStatus, change.OldConfirms, change.NewConfirms)
	}
	return nil
}

func buildWatchFromCandidate(c store.ReorgCandidate, minConf int64) store.WatchAddress {
	return store.WatchAddress{
		TenantID:          c.TenantID,
		AccountID:         c.AccountID,
		Model:             c.Model,
		Chain:             c.Chain,
		Coin:              c.Coin,
		Network:           c.Network,
		Address:           c.Address,
		MinConfirmations:  minConf,
		TreasuryAccountID: fallback(c.TreasuryAccountID, "treasury-main"),
		SweepThreshold:    c.SweepThreshold,
	}
}
