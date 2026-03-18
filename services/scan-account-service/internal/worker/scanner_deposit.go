package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	watches, err := s.Store.ListWatchAddresses(ctx, model, s.WatchLimit)
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
			if supportsChainWideAccountScan(group.Chain) {
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
	for i := 0; i < maxPages; i++ {
		txs, nextCursor, err := s.ChainGateway.ListIncomingTransfers(
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
		if nextCursor != "" {
			nextCheckpoint = nextCursor
		}
		if len(txs) > 0 {
			lastTx = strings.TrimSpace(txs[len(txs)-1].TxHash)
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
				status := resolveDepositStatus(tx.Status, tx.Confirmations, minConf)
				change, err := s.Store.UpsertSeenEvent(ctx, w, tx.TxHash, eventIdx, status, tx.Confirmations, tx.Amount, tx.FromAddress, fallback(tx.ToAddress, w.Address))
				if err != nil {
					return err
				}
				if !change.Notify {
					continue
				}
				log.Printf("deposit state transition tenant=%s account=%s order=%s tx=%s old_status=%s new_status=%s old_conf=%d new_conf=%d source=scan",
					w.TenantID, w.AccountID, depositOrderID(tx.TxHash, eventIdx, w.AccountID, w.Network), tx.TxHash, change.OldStatus, change.NewStatus, change.OldConfirms, change.NewConfirms)
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
					status,
				); err != nil {
					return err
				}
			}
		}

		if nextCursor == "" || nextCursor == currentCursor {
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
	return nil
}

func (s *Scanner) scanOneWatch(ctx context.Context, w store.WatchAddress, managedAddrs map[string]struct{}) error {
	if strings.TrimSpace(w.Network) == "" {
		return fmt.Errorf("watch network is required tenant=%s account=%s chain=%s address=%s", w.TenantID, w.AccountID, w.Chain, w.Address)
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
	for i := 0; i < maxPages; i++ {
		txs, nextCursor, err := s.ChainGateway.ListIncomingTransfers(ctx, w.Model, w.Chain, w.Coin, w.Network, w.Address, cursor, s.AccountPageSize)
		if err != nil {
			return err
		}
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
			eventIdx := tx.Index
			if eventIdx <= 0 {
				eventIdx = int64(idx)
			}
			status := resolveDepositStatus(tx.Status, tx.Confirmations, minConf)
			change, err := s.Store.UpsertSeenEvent(ctx, w, tx.TxHash, eventIdx, status, tx.Confirmations, tx.Amount, tx.FromAddress, fallback(tx.ToAddress, w.Address))
			if err != nil {
				return err
			}
			if !change.Notify {
				continue
			}
			log.Printf("deposit state transition tenant=%s account=%s order=%s tx=%s old_status=%s new_status=%s old_conf=%d new_conf=%d source=scan",
				w.TenantID, w.AccountID, depositOrderID(tx.TxHash, eventIdx, w.AccountID, w.Network), tx.TxHash, change.OldStatus, change.NewStatus, change.OldConfirms, change.NewConfirms)
			if err := s.enqueueDepositEvent(ctx, w, tx.TxHash, eventIdx, tx.Amount, tx.FromAddress, fallback(tx.ToAddress, w.Address), tx.Confirmations, minConf, status); err != nil {
				return err
			}
			lastTx = tx.TxHash
		}
		if nextCursor == "" || nextCursor == cursor || maxPages == 1 {
			break
		}
		cursor = nextCursor
	}
	if maxPages == 1 {
		cursor = ""
	}
	return s.Store.UpsertCheckpoint(ctx, w, cursor, lastTx)
}

func (s *Scanner) enqueueDepositEvent(ctx context.Context, w store.WatchAddress, txHash string, eventIndex int64, amount, fromAddr, toAddr string, confirmations, requiredConfirmations int64, status string) error {
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
		SweepThreshold: strings.TrimSpace(w.SweepThreshold),
		Confirmations:  confirmations,
		Status:         status,
		RequiredConfs:  requiredConfirmations,
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
	log.Printf("deposit event enqueued tenant=%s account=%s order=%s tx=%s status=%s conf=%d amount=%s key=%s",
		w.TenantID, w.AccountID, orderID, txHash, status, confirmations, amount, eventKey)
	return nil
}

func supportsChainWideAccountScan(chain string) bool {
	// EVM account reader currently performs expensive full-block scans when address is empty.
	// Keep chain-wide scan disabled to use per-address path and avoid long request timeouts.
	_ = chain
	return false
}
