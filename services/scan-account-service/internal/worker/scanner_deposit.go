package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"wallet-saas-v2/services/scan-account-service/internal/store"
)

type watchGroup struct {
	Model   string
	Chain   string
	Coin    string
	Network string
	Watches []store.WatchAddress
}

func (s *Scanner) scanModel(ctx context.Context, model string) error {
	watches, err := s.Store.ListWatchAddresses(ctx, model, s.WatchLimit, s.AllowedChains)
	if err != nil {
		return err
	}
	log.Printf("scan %s tick: watches=%d", model, len(watches))
	if len(watches) == 0 {
		return nil
	}

	if strings.EqualFold(strings.TrimSpace(model), "account") {
		return s.scanAccountModel(ctx, watches)
	}
	s.scanWatchesIndividually(ctx, model, watches)
	return nil
}

func (s *Scanner) scanAccountModel(ctx context.Context, watches []store.WatchAddress) error {
	groups := make(map[string]*watchGroup)
	for _, w := range watches {
		key := strings.ToLower(strings.TrimSpace(w.Model)) + "|" +
			strings.ToLower(strings.TrimSpace(w.Chain)) + "|" +
			strings.ToUpper(strings.TrimSpace(w.Coin)) + "|" +
			strings.ToLower(strings.TrimSpace(w.Network))
		g := groups[key]
		if g == nil {
			g = &watchGroup{
				Model:   strings.ToLower(strings.TrimSpace(w.Model)),
				Chain:   strings.ToLower(strings.TrimSpace(w.Chain)),
				Coin:    strings.ToUpper(strings.TrimSpace(w.Coin)),
				Network: strings.ToLower(strings.TrimSpace(w.Network)),
				Watches: make([]store.WatchAddress, 0, 8),
			}
			groups[key] = g
		}
		g.Watches = append(g.Watches, w)
	}

	sem := make(chan struct{}, s.AddrConcurrency)
	var wg sync.WaitGroup
	for _, g := range groups {
		group := g
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if supportsChainWideAccountScan(group.Chain, group.Coin) {
				if err := s.scanOneAccountGroup(ctx, group); err != nil {
					log.Printf("account group scan failed chain=%s network=%s coin=%s watches=%d err=%v",
						group.Chain, group.Network, group.Coin, len(group.Watches), err)
				}
				return
			}
			managedAddrs, err := s.Store.ListManagedAddresses(ctx, group.Model, group.Chain, group.Network)
			if err != nil {
				log.Printf("managed address preload failed chain=%s network=%s coin=%s watches=%d err=%v",
					group.Chain, group.Network, group.Coin, len(group.Watches), err)
				return
			}
			s.scanWatchesIndividuallyWithManaged(ctx, "account", group.Watches, managedAddrs)
		}()
	}
	wg.Wait()
	return nil
}

func (s *Scanner) scanWatchesIndividually(ctx context.Context, model string, watches []store.WatchAddress) {
	s.scanWatchesIndividuallyWithManaged(ctx, model, watches, nil)
}

func (s *Scanner) scanWatchesIndividuallyWithManaged(ctx context.Context, model string, watches []store.WatchAddress, managedAddrs map[string]struct{}) {
	sem := make(chan struct{}, s.AddrConcurrency)
	var wg sync.WaitGroup
	for _, w := range watches {
		watch := w
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := s.scanOneWatch(ctx, watch, managedAddrs); err != nil {
				log.Printf("%s scan failed chain=%s coin=%s addr=%s err=%v", model, watch.Chain, watch.Coin, watch.Address, err)
			}
		}()
	}
	wg.Wait()
}

func (s *Scanner) scanOneAccountGroup(ctx context.Context, group *watchGroup) error {
	if group == nil || len(group.Watches) == 0 {
		return nil
	}
	if strings.TrimSpace(group.Network) == "" {
		return fmt.Errorf("group network is required model=%s chain=%s coin=%s", group.Model, group.Chain, group.Coin)
	}

	chainCursor, _, err := s.Store.GetChainCheckpoint(ctx, group.Model, group.Chain, group.Coin, group.Network)
	if err != nil {
		return err
	}
	currentCursor := chainCursor
	nextCheckpoint := chainCursor
	lastTx := ""

	watchByAddr := make(map[string][]store.WatchAddress)
	for _, w := range group.Watches {
		addr := strings.ToLower(strings.TrimSpace(w.Address))
		if addr == "" {
			continue
		}
		watchByAddr[addr] = append(watchByAddr[addr], w)
	}
	if len(watchByAddr) == 0 {
		return nil
	}
	managedAddrs, err := s.Store.ListManagedAddresses(ctx, group.Model, group.Chain, group.Network)
	if err != nil {
		return err
	}

	maxPages := s.AccountMaxPages
	if maxPages <= 0 {
		maxPages = 1
	}
	guard := newPaginationGuard(s.AccountMaxEmptyPages, s.AccountCursorStallGuard)
	for i := 0; i < maxPages; i++ {
		result, err := s.ChainGateway.ListIncomingTransfers(
			ctx,
			group.Model,
			group.Chain,
			group.Coin,
			group.Network,
			"",
			currentCursor,
			s.AccountPageSize,
		)
		if err != nil {
			return err
		}
		txs := result.Items
		nextCursor := result.NextCursor
		if len(txs) > 0 {
			lastTx = strings.TrimSpace(txs[len(txs)-1].TxHash)
		}

		for _, bm := range result.Blocks {
			if bm.Hash != "" {
				prev, found, _ := s.Store.GetBlockHash(ctx, group.Chain, group.Network, bm.Number-1)
				if found && prev.BlockHash != "" && bm.ParentHash != "" && prev.BlockHash != bm.ParentHash {
					log.Printf("REORG detected chain=%s network=%s block=%d expected_parent=%s actual_parent=%s", group.Chain, group.Network, bm.Number, prev.BlockHash, bm.ParentHash)
				}
				_ = s.Store.UpsertBlockHash(ctx, group.Chain, group.Network, bm.Number, bm.Hash, bm.ParentHash)
			}
		}

		for idx, tx := range txs {
			if shouldSkipInternalTransfer(tx.FromAddress, managedAddrs) {
				log.Printf("skip internal transfer tenant_scope=platform chain=%s network=%s tx=%s from=%s to=%s amount=%s",
					group.Chain, group.Network, tx.TxHash, tx.FromAddress, tx.ToAddress, tx.Amount)
				continue
			}
			toAddr := strings.ToLower(strings.TrimSpace(tx.ToAddress))
			if toAddr == "" {
				continue
			}
			matched := watchByAddr[toAddr]
			if len(matched) == 0 {
				continue
			}
			eventIdx := tx.Index
			if eventIdx <= 0 {
				eventIdx = int64(idx)
			}
			for _, w := range matched {
				minConf := w.MinConfirmations
				if minConf <= 0 {
					minConf = 1
				}
				unlockConf := w.UnlockConfirmations
				if unlockConf > 0 && unlockConf < minConf {
					unlockConf = minConf
				}
				status := resolveDepositScanStatus(tx.Status, tx.Confirmations, minConf, unlockConf, s.ReorgWindow)
				change, err := s.Store.UpsertSeenEvent(ctx, w, tx.TxHash, eventIdx, status, tx.Confirmations, tx.Amount, tx.FromAddress, fallback(tx.ToAddress, w.Address))
				if err != nil {
					return err
				}
				if !change.Notify {
					continue
				}
				log.Printf("deposit state transition tenant=%s account=%s order=%s tx=%s old_scan_status=%s new_scan_status=%s old_ledger_status=%s new_ledger_status=%s old_conf=%d new_conf=%d source=scan",
					w.TenantID, w.AccountID, depositOrderID(tx.TxHash, eventIdx, w.AccountID, w.Network), tx.TxHash, change.OldStatus, change.NewStatus, mapDepositLedgerStatus(change.OldStatus), mapDepositLedgerStatus(change.NewStatus), change.OldConfirms, change.NewConfirms)
				if err := s.enqueueDepositEvent(
					ctx,
					w,
					tx.TxHash,
					eventIdx,
					tx.Amount,
					tx.FromAddress,
					fallback(tx.ToAddress, w.Address),
					tx.Confirmations,
					minConf,
					unlockConf,
					status,
				); err != nil {
					return err
				}
			}
		}

		advanceCursor, stop, reason := guard.Observe(currentCursor, nextCursor, len(txs))
		if advanceCursor || reason == "max_empty_pages" {
			nextCheckpoint = nextCursor
		}
		if stop {
			if reason != "" {
				log.Printf("account group scan pagination stop chain=%s network=%s coin=%s reason=%s current_cursor=%s next_cursor=%s page=%d txs=%d",
					group.Chain, group.Network, group.Coin, reason, currentCursor, nextCursor, i+1, len(txs))
			}
			break
		}
		if !advanceCursor {
			break
		}
		currentCursor = nextCursor
	}

	if err := s.Store.UpsertChainCheckpoint(ctx, group.Model, group.Chain, group.Coin, group.Network, nextCheckpoint, lastTx); err != nil {
		return err
	}
	for _, w := range group.Watches {
		_ = s.Store.UpsertCheckpoint(ctx, w, nextCheckpoint, lastTx)
	}

	if cursorNum, err := strconv.ParseInt(nextCheckpoint, 10, 64); err == nil && cursorNum > 512 {
		_ = s.Store.PruneBlockHashes(ctx, group.Chain, group.Network, cursorNum-512)
	}
	return nil
}

func (s *Scanner) scanOneWatch(ctx context.Context, w store.WatchAddress, managedAddrs map[string]struct{}) error {
	if strings.TrimSpace(w.Network) == "" {
		return fmt.Errorf("watch network is required tenant=%s account=%s chain=%s address=%s", w.TenantID, w.AccountID, w.Chain, w.Address)
	}
	// For EVM non-native coins without a contract address, skip per-address scan
	// since the old native-only reader would mislabel deposits.
	if isEVMChain(w.Chain) && !isNativeEVMAsset(w.Chain, w.Coin) && strings.TrimSpace(w.ContractAddress) == "" {
		return nil
	}
	cursor, err := s.Store.GetCheckpoint(ctx, w)
	if err != nil {
		return err
	}
	lastTx := ""
	if managedAddrs == nil {
		managedAddrs, err = s.Store.ListManagedAddresses(ctx, w.Model, w.Chain, w.Network)
		if err != nil {
			return err
		}
	}
	maxPages := s.AccountMaxPages
	if strings.EqualFold(strings.TrimSpace(w.Model), "utxo") {
		maxPages = 1
	}
	guard := newPaginationGuard(s.AccountMaxEmptyPages, s.AccountCursorStallGuard)
	for i := 0; i < maxPages; i++ {
		result, err := s.ChainGateway.ListIncomingTransfers(ctx, w.Model, w.Chain, w.Coin, w.Network, w.Address, cursor, s.AccountPageSize, w.ContractAddress)
		if err != nil {
			return err
		}
		txs := result.Items
		nextCursor := result.NextCursor
		for idx, tx := range txs {
			if shouldSkipInternalTransfer(tx.FromAddress, managedAddrs) {
				log.Printf("skip internal transfer tenant=%s account=%s chain=%s network=%s tx=%s from=%s to=%s amount=%s",
					w.TenantID, w.AccountID, w.Chain, w.Network, tx.TxHash, tx.FromAddress, tx.ToAddress, tx.Amount)
				continue
			}
			minConf := w.MinConfirmations
			if minConf <= 0 {
				minConf = 1
			}
			unlockConf := w.UnlockConfirmations
			if unlockConf > 0 && unlockConf < minConf {
				unlockConf = minConf
			}
			eventIdx := tx.Index
			if eventIdx <= 0 {
				eventIdx = int64(idx)
			}
			status := resolveDepositScanStatus(tx.Status, tx.Confirmations, minConf, unlockConf, s.ReorgWindow)
			change, err := s.Store.UpsertSeenEvent(ctx, w, tx.TxHash, eventIdx, status, tx.Confirmations, tx.Amount, tx.FromAddress, fallback(tx.ToAddress, w.Address))
			if err != nil {
				return err
			}
			if !change.Notify {
				continue
			}
			log.Printf("deposit state transition tenant=%s account=%s order=%s tx=%s old_scan_status=%s new_scan_status=%s old_ledger_status=%s new_ledger_status=%s old_conf=%d new_conf=%d source=scan",
				w.TenantID, w.AccountID, depositOrderID(tx.TxHash, eventIdx, w.AccountID, w.Network), tx.TxHash, change.OldStatus, change.NewStatus, mapDepositLedgerStatus(change.OldStatus), mapDepositLedgerStatus(change.NewStatus), change.OldConfirms, change.NewConfirms)
			if err := s.enqueueDepositEvent(ctx, w, tx.TxHash, eventIdx, tx.Amount, tx.FromAddress, fallback(tx.ToAddress, w.Address), tx.Confirmations, minConf, unlockConf, status); err != nil {
				return err
			}
			lastTx = tx.TxHash
		}
		advanceCursor, stop, reason := guard.Observe(cursor, nextCursor, len(txs))
		if reason == "max_empty_pages" {
			cursor = nextCursor
		}
		if stop {
			if reason != "" {
				log.Printf("watch scan pagination stop tenant=%s account=%s chain=%s coin=%s network=%s address=%s reason=%s current_cursor=%s next_cursor=%s page=%d txs=%d",
					w.TenantID, w.AccountID, w.Chain, w.Coin, w.Network, w.Address, reason, cursor, nextCursor, i+1, len(txs))
			}
			break
		}
		if !advanceCursor {
			break
		}
		cursor = nextCursor
	}
	return s.Store.UpsertCheckpoint(ctx, w, cursor, lastTx)
}

func (s *Scanner) enqueueDepositEvent(ctx context.Context, w store.WatchAddress, txHash string, eventIndex int64, amount, fromAddr, toAddr string, confirmations, requiredConfirmations, unlockConfirmations int64, scanStatus string) error {
	orderID := depositOrderID(txHash, eventIndex, w.AccountID, w.Network)
	payload := DepositOutboxPayload{
		TenantID:       w.TenantID,
		AccountID:      w.AccountID,
		OrderID:        orderID,
		EventIndex:     eventIndex,
		Chain:          w.Chain,
		Network:        w.Network,
		Coin:           w.Coin,
		Amount:         amount,
		TxHash:         txHash,
		FromAddress:    fromAddr,
		ToAddress:      toAddr,
		WatchAddress:   w.Address,
		TreasuryID:     fallback(w.TreasuryAccountID, "treasury-main"),
		ColdAccountID:  strings.TrimSpace(w.ColdAccountID),
		AutoSweep:      w.AutoSweep,
		SweepThreshold: strings.TrimSpace(w.SweepThreshold),
		HotBalanceCap:  strings.TrimSpace(w.HotBalanceCap),
		Confirmations:  confirmations,
		ScanStatus:     scanStatus,
		Status:         mapDepositLedgerStatus(scanStatus),
		ProjectNotify:  strings.TrimSpace(w.AccountID) != strings.TrimSpace(fallback(w.TreasuryAccountID, "treasury-main")),
		SweepTrigger:   w.AutoSweep && strings.TrimSpace(w.AccountID) != strings.TrimSpace(fallback(w.TreasuryAccountID, "treasury-main")),
		RequiredConfs:  requiredConfirmations,
		UnlockConfs:    unlockConfirmations,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	eventKey := depositEventKey(w.TenantID, w.Chain, w.Network, txHash, eventIndex)
	if err := s.Store.UpsertOutboxEvent(ctx, store.OutboxEvent{
		EventKey:    eventKey,
		TenantID:    w.TenantID,
		Chain:       strings.ToLower(strings.TrimSpace(w.Chain)),
		Network:     strings.ToLower(strings.TrimSpace(w.Network)),
		EventType:   OutboxEventDepositNotify,
		Payload:     string(raw),
		MaxAttempts: 24,
	}); err != nil {
		return err
	}
	log.Printf("deposit event enqueued tenant=%s account=%s order=%s tx=%s scan_status=%s ledger_status=%s conf=%d amount=%s key=%s",
		w.TenantID, w.AccountID, orderID, txHash, scanStatus, payload.Status, confirmations, amount, eventKey)
	return nil
}

func isEVMChain(chain string) bool {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "ethereum", "binance", "polygon", "arbitrum", "optimism", "linea", "scroll", "mantle", "zksync", "base", "avalanche":
		return true
	default:
		return false
	}
}

func supportsChainWideAccountScan(chain, coin string) bool {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "ethereum", "binance", "polygon", "arbitrum", "optimism", "linea", "scroll", "mantle", "zksync", "base", "avalanche":
		return isNativeEVMAsset(chain, coin)
	default:
		return false
	}
}

func isNativeEVMAsset(chain, coin string) bool {
	c := strings.ToUpper(strings.TrimSpace(coin))
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "binance":
		return c == "BNB"
	case "polygon":
		return c == "MATIC" || c == "POL"
	case "avalanche":
		return c == "AVAX"
	case "mantle":
		return c == "MNT"
	default:
		return c == "ETH"
	}
}
