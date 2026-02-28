package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"wallet-saas-v2/services/scan-service/internal/client"
	"wallet-saas-v2/services/scan-service/internal/config"
	"wallet-saas-v2/services/scan-service/internal/store"
	"wallet-saas-v2/services/scan-service/internal/worker"
)

func Run() error {
	cfg := config.Load()
	if cfg.DBDSN == "" {
		return errors.New("SCAN_DB_DSN is required")
	}
	if cfg.APIToken == "" {
		log.Printf("warning: SCAN_API_TOKEN is empty, wallet-core requests may be unauthorized")
	}

	st, err := store.NewPostgres(cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("init postgres failed: %w", err)
	}
	defer st.Close()

	wc := client.NewWalletCore(cfg.WalletCoreAddr, cfg.APIToken, time.Duration(cfg.WalletCoreTimeoutMS)*time.Millisecond)
	cg, err := client.NewChainGateway(cfg.ChainGatewayGRPCAddr, time.Duration(cfg.ChainGatewayTimeoutMS)*time.Millisecond)
	if err != nil {
		return fmt.Errorf("init chain-gateway grpc client failed: %w", err)
	}
	defer cg.Close()
	scanner := &worker.Scanner{
		Store:            st,
		WalletCore:       wc,
		ChainGateway:     cg,
		Interval:         time.Duration(cfg.IntervalSeconds) * time.Second,
		AccountPageSize:  cfg.AccountPageSize,
		AccountMaxPages:  cfg.AccountMaxPages,
		WatchLimit:       cfg.WatchLimit,
		AddrConcurrency:  cfg.AddrConcurrency,
		SweepMinBalance:  cfg.SweepMinBalance,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("service=scan-service wallet-core=%s chain-gateway-grpc=%s interval=%ds addr-concurrency=%d sweep-min-balance=%s",
		cfg.WalletCoreAddr, cfg.ChainGatewayGRPCAddr, cfg.IntervalSeconds, cfg.AddrConcurrency, cfg.SweepMinBalance)
	err = scanner.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
