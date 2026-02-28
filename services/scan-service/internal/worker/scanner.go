package worker

import (
	"context"
	"log"
	"time"

	"wallet-saas-v2/services/scan-service/internal/client"
	"wallet-saas-v2/services/scan-service/internal/store"
)

type Scanner struct {
	Store           *store.Postgres
	WalletCore      *client.WalletCore
	ChainGateway    *client.ChainGateway
	Interval        time.Duration
	AccountPageSize int
	AccountMaxPages int
	WatchLimit      int
	AddrConcurrency int
	SweepMinBalance string
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
	if s.AddrConcurrency <= 0 {
		s.AddrConcurrency = 8
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
	if err := s.scanModel(ctx, "account"); err != nil {
		return err
	}
	if err := s.scanModel(ctx, "utxo"); err != nil {
		return err
	}
	if err := s.dispatchOutbox(ctx); err != nil {
		return err
	}
	if err := s.scanOutgoingConfirmations(ctx); err != nil {
		return err
	}
	return nil
}
