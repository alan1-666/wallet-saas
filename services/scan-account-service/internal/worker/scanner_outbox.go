package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"wallet-saas-v2/services/scan-account-service/internal/client"
	"wallet-saas-v2/services/scan-account-service/internal/store"
)

const (
	OutboxEventDepositNotify = "DEPOSIT_NOTIFY"
	OutboxEventSweepRun      = "SWEEP_RUN"
)

type DepositOutboxPayload struct {
	TenantID       string `json:"tenant_id"`
	AccountID      string `json:"account_id"`
	OrderID        string `json:"order_id"`
	EventIndex     int64  `json:"event_index"`
	Chain          string `json:"chain"`
	Network        string `json:"network"`
	Coin           string `json:"coin"`
	Amount         string `json:"amount"`
	TxHash         string `json:"tx_hash"`
	FromAddress    string `json:"from_address"`
	ToAddress      string `json:"to_address"`
	WatchAddress   string `json:"watch_address"`
	TreasuryID     string `json:"treasury_account_id"`
	SweepThreshold string `json:"sweep_threshold"`
	Confirmations  int64  `json:"confirmations"`
	RequiredConfs  int64  `json:"required_confirmations"`
	Status         string `json:"status"`
}

type SweepOutboxPayload struct {
	TenantID          string `json:"tenant_id"`
	SweepOrderID      string `json:"sweep_order_id"`
	FromAccountID     string `json:"from_account_id"`
	TreasuryAccountID string `json:"treasury_account_id"`
	Chain             string `json:"chain"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
	Amount            string `json:"amount"`
}

func (s *Scanner) dispatchOutbox(ctx context.Context) error {
	events, err := s.Store.ListPendingOutboxEvents(ctx, s.WatchLimit)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	for _, ev := range events {
		var handleErr error
		switch strings.ToUpper(strings.TrimSpace(ev.EventType)) {
		case OutboxEventDepositNotify:
			handleErr = s.handleDepositOutboxEvent(ctx, ev)
		case OutboxEventSweepRun:
			handleErr = s.handleSweepOutboxEvent(ctx, ev)
		default:
			handleErr = fmt.Errorf("unsupported outbox event type=%s", ev.EventType)
		}
		if handleErr != nil {
			_ = s.Store.MarkOutboxEventRetry(ctx, ev.ID, ev.Attempt, ev.MaxAttempts, handleErr.Error())
			log.Printf("outbox dispatch failed id=%d key=%s type=%s attempt=%d/%d err=%v", ev.ID, ev.EventKey, ev.EventType, ev.Attempt+1, ev.MaxAttempts, handleErr)
			continue
		}
		if err := s.Store.MarkOutboxEventDone(ctx, ev.ID); err != nil {
			log.Printf("outbox mark done failed id=%d key=%s err=%v", ev.ID, ev.EventKey, err)
			continue
		}
		log.Printf("outbox dispatched id=%d key=%s type=%s", ev.ID, ev.EventKey, ev.EventType)
	}
	return nil
}

func (s *Scanner) handleDepositOutboxEvent(ctx context.Context, ev store.OutboxEvent) error {
	var payload DepositOutboxPayload
	if err := json.Unmarshal([]byte(ev.Payload), &payload); err != nil {
		return fmt.Errorf("decode deposit outbox payload: %w", err)
	}
	if payload.TenantID == "" || payload.AccountID == "" || payload.OrderID == "" || payload.Chain == "" || payload.Network == "" {
		return fmt.Errorf("invalid deposit outbox payload")
	}
	reqID := fmt.Sprintf("scan-outbox-deposit-%d-%d", ev.ID, ev.Attempt+1)
	if err := s.WalletCore.DepositNotify(ctx, reqID, client.DepositNotifyRequest{
		TenantID:      payload.TenantID,
		AccountID:     payload.AccountID,
		OrderID:       payload.OrderID,
		Chain:         payload.Chain,
		Network:       payload.Network,
		Coin:          payload.Coin,
		Amount:        payload.Amount,
		TxHash:        payload.TxHash,
		FromAddress:   payload.FromAddress,
		ToAddress:     payload.ToAddress,
		Confirmations: payload.Confirmations,
		RequiredConfs: payload.RequiredConfs,
		Status:        payload.Status,
	}); err != nil {
		return err
	}
	if strings.ToUpper(strings.TrimSpace(payload.Status)) != "CONFIRMED" {
		return nil
	}
	if strings.TrimSpace(payload.AccountID) == strings.TrimSpace(payload.TreasuryID) {
		return nil
	}
	if err := s.notifyProjectDeposit(ctx, reqID, payload); err != nil {
		return err
	}

	threshold := strings.TrimSpace(payload.SweepThreshold)
	if threshold == "" || threshold == "0" {
		threshold = strings.TrimSpace(s.SweepMinBalance)
	}
	watchAddr := strings.TrimSpace(payload.WatchAddress)
	if watchAddr == "" {
		watchAddr = strings.TrimSpace(payload.ToAddress)
	}
	if watchAddr == "" {
		return nil
	}
	currentBalance, err := s.ChainGateway.GetBalance(ctx, payload.Chain, payload.Coin, payload.Network, watchAddr)
	if err != nil {
		return err
	}
	if !meetsThreshold(currentBalance, threshold) {
		log.Printf("skip sweep enqueue tenant=%s account=%s tx=%s reason=balance_below_threshold balance=%s threshold=%s",
			payload.TenantID, payload.AccountID, payload.TxHash, currentBalance, threshold)
		return nil
	}

	sweepPayload := SweepOutboxPayload{
		TenantID:          payload.TenantID,
		SweepOrderID:      sweepOrderID(payload.TxHash, payload.EventIndex, payload.AccountID, payload.Network),
		FromAccountID:     payload.AccountID,
		TreasuryAccountID: fallback(payload.TreasuryID, "treasury-main"),
		Chain:             payload.Chain,
		Network:           payload.Network,
		Asset:             strings.ToUpper(payload.Coin),
		Amount:            payload.Amount,
	}
	raw, err := json.Marshal(sweepPayload)
	if err != nil {
		return err
	}
	sweepKey := sweepEventKey(payload.TenantID, payload.Chain, payload.Network, payload.TxHash, payload.EventIndex)
	return s.Store.UpsertOutboxEvent(ctx, store.OutboxEvent{
		EventKey:    sweepKey,
		TenantID:    payload.TenantID,
		Chain:       strings.ToLower(strings.TrimSpace(payload.Chain)),
		Network:     strings.ToLower(strings.TrimSpace(payload.Network)),
		EventType:   OutboxEventSweepRun,
		Payload:     string(raw),
		MaxAttempts: 24,
	})
}

func (s *Scanner) handleSweepOutboxEvent(ctx context.Context, ev store.OutboxEvent) error {
	var payload SweepOutboxPayload
	if err := json.Unmarshal([]byte(ev.Payload), &payload); err != nil {
		return fmt.Errorf("decode sweep outbox payload: %w", err)
	}
	if payload.TenantID == "" || payload.SweepOrderID == "" || payload.FromAccountID == "" || payload.Chain == "" || payload.Network == "" {
		return fmt.Errorf("invalid sweep outbox payload")
	}
	reqID := fmt.Sprintf("scan-outbox-sweep-%d-%d", ev.ID, ev.Attempt+1)
	return s.WalletCore.SweepRun(ctx, reqID, client.SweepRunRequest{
		TenantID:          payload.TenantID,
		SweepOrderID:      payload.SweepOrderID,
		FromAccountID:     payload.FromAccountID,
		TreasuryAccountID: payload.TreasuryAccountID,
		Chain:             payload.Chain,
		Network:           payload.Network,
		Asset:             payload.Asset,
		Amount:            payload.Amount,
	})
}
