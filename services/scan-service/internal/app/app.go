package app

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

	wc := client.NewWalletCore(cfg.WalletCoreAddr, cfg.APIToken)
	cg := client.NewChainGateway(cfg.ChainGatewayAddr)
	ethRPC := client.NewEthRPC(cfg.EthRPCURL)
	scanner := &worker.Scanner{
		Store:            st,
		WalletCore:       wc,
		ChainGateway:     cg,
		EthRPC:           ethRPC,
		Interval:         time.Duration(cfg.IntervalSeconds) * time.Second,
		MinConfirmations: cfg.MinConfirmations,
		AccountPageSize:  cfg.AccountPageSize,
		AccountMaxPages:  cfg.AccountMaxPages,
		WatchLimit:       cfg.WatchLimit,
		EthLookback:      uint64(cfg.EthLookback),
		EthMaxBlocksTick: uint64(cfg.EthMaxBlocksTick),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("scan-service started, wallet-core=%s chain-gateway=%s eth-rpc=%t interval=%ds", cfg.WalletCoreAddr, cfg.ChainGatewayAddr, ethRPC.Enabled(), cfg.IntervalSeconds)
	err = scanner.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
