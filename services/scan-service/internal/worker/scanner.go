package worker

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"wallet-saas-v2/services/scan-service/internal/client"
	"wallet-saas-v2/services/scan-service/internal/store"
)

type Scanner struct {
	Store            *store.Postgres
	WalletCore       *client.WalletCore
	ChainGateway     *client.ChainGateway
	EthRPC           *client.EthRPC
	Interval         time.Duration
	MinConfirmations int64
	AccountPageSize  int
	AccountMaxPages  int
	WatchLimit       int
	EthLookback      uint64
	EthMaxBlocksTick uint64
}

func (s *Scanner) Run(ctx context.Context) error {
	if s.Interval <= 0 {
		s.Interval = 5 * time.Second
	}
	if s.AccountPageSize <= 0 {
		s.AccountPageSize = 50
	}
	if s.AccountMaxPages <= 0 {
		s.AccountMaxPages = 2
	}
	if s.WatchLimit <= 0 {
		s.WatchLimit = 500
	}
	if s.EthLookback == 0 {
		s.EthLookback = 300
	}
	if s.EthMaxBlocksTick == 0 {
		s.EthMaxBlocksTick = 80
	}
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	if err := s.tick(ctx); err != nil {
		log.Printf("scan tick failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				log.Printf("scan tick failed: %v", err)
			}
		}
	}
}

func (s *Scanner) tick(ctx context.Context) error {
	if err := s.scanAccountModel(ctx); err != nil {
		return err
	}
	if err := s.scanUTXOModel(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Scanner) scanAccountModel(ctx context.Context) error {
	watches, err := s.Store.ListWatchAddresses(ctx, "account", s.WatchLimit)
	if err != nil {
		return err
	}
	log.Printf("scan account tick: watches=%d", len(watches))
	for _, w := range watches {
		if err := s.scanOneAccountWatch(ctx, w); err != nil {
			log.Printf("account scan failed chain=%s coin=%s addr=%s err=%v", w.Chain, w.Coin, w.Address, err)
		}
	}
	return nil
}

func (s *Scanner) scanOneAccountWatch(ctx context.Context, w store.WatchAddress) error {
	if isEthChain(w.Chain) && s.EthRPC != nil && s.EthRPC.Enabled() {
		return s.scanOneEthRPCWatch(ctx, w)
	}
	cursor, err := s.Store.GetCheckpoint(ctx, w)
	if err != nil {
		return err
	}
	lastTx := ""
	for i := 0; i < s.AccountMaxPages; i++ {
		txs, nextCursor, err := s.ChainGateway.TxByAddress(ctx, w.Chain, w.Coin, fallback(w.Network, "mainnet"), w.Address, cursor, s.AccountPageSize)
		if err != nil {
			return err
		}
		for idx, tx := range txs {
			minConf := max(w.MinConfirmations, s.MinConfirmations)
			eventIdx := tx.Index
			if eventIdx <= 0 {
				eventIdx = int64(idx)
			}
			status := resolveDepositStatus(tx.Status, tx.Confirmations, minConf)
			inserted, err := s.Store.UpsertSeenEvent(ctx, w, tx.TxHash, eventIdx, status, tx.Confirmations)
			if err != nil {
				return err
			}
			if !inserted {
				continue
			}
			if err := s.notifyDepositAndSweep(ctx, w, tx.TxHash, eventIdx, tx.Amount, tx.FromAddress, fallback(tx.ToAddress, w.Address), tx.Confirmations, minConf, status); err != nil {
				return err
			}
			lastTx = tx.TxHash
		}
		if nextCursor == "" || nextCursor == cursor || len(txs) == 0 {
			break
		}
		cursor = nextCursor
	}
	return s.Store.UpsertCheckpoint(ctx, w, cursor, lastTx)
}

func (s *Scanner) scanOneEthRPCWatch(ctx context.Context, w store.WatchAddress) error {
	cursor, err := s.Store.GetCheckpoint(ctx, w)
	if err != nil {
		return err
	}
	latest, err := s.EthRPC.LatestBlockNumber(ctx)
	if err != nil {
		return err
	}
	var start uint64
	if strings.TrimSpace(cursor) == "" {
		if latest > s.EthLookback {
			start = latest - s.EthLookback + 1
		} else {
			start = 1
		}
	} else {
		n, err := strconv.ParseUint(strings.TrimSpace(cursor), 10, 64)
		if err != nil {
			n = 0
		}
		start = n + 1
	}
	if start == 0 {
		start = 1
	}
	if start > latest {
		log.Printf("eth scan skip tenant=%s account=%s addr=%s latest=%d cursor=%s", w.TenantID, w.AccountID, w.Address, latest, cursor)
		return s.Store.UpsertCheckpoint(ctx, w, strconv.FormatUint(latest, 10), "")
	}
	end := latest
	if s.EthMaxBlocksTick > 0 && end-start+1 > s.EthMaxBlocksTick {
		end = start + s.EthMaxBlocksTick - 1
	}
	log.Printf("eth scan range tenant=%s account=%s addr=%s start=%d end=%d latest=%d", w.TenantID, w.AccountID, w.Address, start, end, latest)

	txs, processedEnd, err := s.EthRPC.BlockTransactionsTo(ctx, start, end, w.Address)
	if err != nil {
		return err
	}
	log.Printf("eth scan matched tenant=%s account=%s addr=%s txs=%d", w.TenantID, w.AccountID, w.Address, len(txs))
	minConf := max(w.MinConfirmations, s.MinConfirmations)
	lastTx := ""
	for _, tx := range txs {
		conf := int64(latest - tx.Number + 1)
		status := resolveDepositStatus("", conf, minConf)
		changed, err := s.Store.UpsertSeenEvent(ctx, w, tx.Hash, int64(tx.Index), status, conf)
		if err != nil {
			return err
		}
		if !changed {
			log.Printf("eth scan seen unchanged tenant=%s account=%s tx=%s status=%s conf=%d", w.TenantID, w.AccountID, tx.Hash, status, conf)
			continue
		}
		if err := s.notifyDepositAndSweep(ctx, w, tx.Hash, int64(tx.Index), tx.Value, tx.From, tx.To, conf, minConf, status); err != nil {
			return err
		}
		lastTx = tx.Hash
	}
	return s.Store.UpsertCheckpoint(ctx, w, strconv.FormatUint(processedEnd, 10), lastTx)
}

func (s *Scanner) scanUTXOModel(ctx context.Context) error {
	watches, err := s.Store.ListWatchAddresses(ctx, "utxo", s.WatchLimit)
	if err != nil {
		return err
	}
	log.Printf("scan utxo tick: watches=%d", len(watches))
	for _, w := range watches {
		if err := s.scanOneUTXOWatch(ctx, w); err != nil {
			log.Printf("utxo scan failed chain=%s coin=%s addr=%s err=%v", w.Chain, w.Coin, w.Address, err)
		}
	}
	return nil
}

func (s *Scanner) scanOneUTXOWatch(ctx context.Context, w store.WatchAddress) error {
	utxos, err := s.ChainGateway.UnspentOutputs(ctx, w.Chain, fallback(w.Network, "mainnet"), w.Address)
	if err != nil {
		return err
	}
	lastTx := ""
	for _, u := range utxos {
		minConf := max(w.MinConfirmations, s.MinConfirmations)
		status := resolveDepositStatus("", u.Confirmations, minConf)
		inserted, err := s.Store.UpsertSeenEvent(ctx, w, u.TxHash, u.Index, status, u.Confirmations)
		if err != nil {
			return err
		}
		if !inserted {
			continue
		}
		if err := s.notifyDepositAndSweep(ctx, w, u.TxHash, u.Index, u.Amount, "", fallback(u.Address, w.Address), u.Confirmations, minConf, status); err != nil {
			return err
		}
		lastTx = u.TxHash
	}
	return s.Store.UpsertCheckpoint(ctx, w, "", lastTx)
}

func (s *Scanner) notifyDepositAndSweep(ctx context.Context, w store.WatchAddress, txHash string, eventIndex int64, amount, fromAddr, toAddr string, confirmations, requiredConfirmations int64, status string) error {
	orderID := depositOrderID(txHash, eventIndex, w.AccountID)
	reqID := fmt.Sprintf("scan-deposit-%s-%d-%s-%s-%s-%d", txHash, eventIndex, w.TenantID, w.AccountID, strings.ToLower(status), confirmations)
	err := s.WalletCore.DepositNotify(ctx, reqID, client.DepositNotifyRequest{
		TenantID:      w.TenantID,
		AccountID:     w.AccountID,
		OrderID:       orderID,
		Chain:         w.Chain,
		Coin:          w.Coin,
		Amount:        amount,
		TxHash:        txHash,
		FromAddress:   fromAddr,
		ToAddress:     toAddr,
		Confirmations: confirmations,
		Status:        status,
		RequiredConfs: requiredConfirmations,
	})
	if err != nil {
		return err
	}
	log.Printf("deposit notified tenant=%s account=%s order=%s tx=%s status=%s conf=%d amount=%s", w.TenantID, w.AccountID, orderID, txHash, status, confirmations, amount)

	if status != "CONFIRMED" || !w.AutoSweep || !meetsThreshold(amount, w.SweepThreshold) {
		return nil
	}
	if strings.TrimSpace(w.AccountID) == strings.TrimSpace(w.TreasuryAccountID) {
		return nil
	}
	sweepReqID := fmt.Sprintf("scan-sweep-%s-%d-%s-%s", txHash, eventIndex, w.TenantID, w.AccountID)
	if err := s.WalletCore.SweepRun(ctx, sweepReqID, client.SweepRunRequest{
		TenantID:          w.TenantID,
		SweepOrderID:      sweepOrderID(txHash, eventIndex, w.AccountID),
		FromAccountID:     w.AccountID,
		TreasuryAccountID: fallback(w.TreasuryAccountID, "treasury-main"),
		Asset:             strings.ToUpper(w.Coin),
		Amount:            amount,
	}); err != nil {
		return err
	}
	log.Printf("sweep triggered tenant=%s from=%s to=%s tx=%s amount=%s", w.TenantID, w.AccountID, fallback(w.TreasuryAccountID, "treasury-main"), txHash, amount)
	return nil
}

func depositOrderID(txHash string, eventIndex int64, accountID string) string {
	return fmt.Sprintf("dep_%s_%d_%s", txHash, eventIndex, accountID)
}

func sweepOrderID(txHash string, eventIndex int64, accountID string) string {
	return fmt.Sprintf("sweep_%s_%d_%s", txHash, eventIndex, accountID)
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func meetsThreshold(amount, threshold string) bool {
	if strings.TrimSpace(threshold) == "" {
		return true
	}
	a, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok {
		return false
	}
	t, ok := new(big.Int).SetString(strings.TrimSpace(threshold), 10)
	if !ok {
		return false
	}
	return a.Cmp(t) >= 0
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func resolveDepositStatus(rawStatus string, confirmations, minConf int64) string {
	if strings.EqualFold(strings.TrimSpace(rawStatus), "REVERTED") {
		return "REVERTED"
	}
	if strings.EqualFold(strings.TrimSpace(rawStatus), "PENDING") {
		return "PENDING"
	}
	if confirmations >= 0 && confirmations < minConf {
		return "PENDING"
	}
	return "CONFIRMED"
}

func isEthChain(chain string) bool {
	c := strings.ToLower(strings.TrimSpace(chain))
	return c == "ethereum" || c == "eth"
}
