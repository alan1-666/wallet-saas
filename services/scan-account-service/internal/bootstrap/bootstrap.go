package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"wallet-saas-v2/services/scan-account-service/internal/client"
	"wallet-saas-v2/services/scan-account-service/internal/config"
	"wallet-saas-v2/services/scan-account-service/internal/store"
	"wallet-saas-v2/services/scan-account-service/internal/worker"
)

func Run() error {
	return runWithMode("account")
}

func RunAccount() error {
	return runWithMode("account")
}

func RunUTXO() error {
	return runWithMode("utxo")
}

func runWithMode(mode string) error {
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
	var projectNotify *client.ProjectNotify
	if cfg.ProjectNotifyBaseURL != "" {
		projectNotify = client.NewProjectNotify(cfg.ProjectNotifyBaseURL, cfg.ProjectNotifyToken, time.Duration(cfg.ProjectNotifyTimeoutMS)*time.Millisecond)
	}
	cg, err := client.NewChainGateway(cfg.ChainGatewayGRPCAddr, time.Duration(cfg.ChainGatewayTimeoutMS)*time.Millisecond)
	if err != nil {
		return fmt.Errorf("init chain-gateway grpc client failed: %w", err)
	}
	defer cg.Close()
	enableAccountScan := false
	enableUTXOScan := false
	enableReorg := false
	enableOutbox := false
	enableOutgoing := false
	switch mode {
	case "account":
		enableAccountScan = true
		enableReorg = true
		enableOutbox = true
		enableOutgoing = true
	case "utxo":
		enableUTXOScan = true
	default:
		enableAccountScan = true
		enableUTXOScan = true
		enableReorg = true
		enableOutbox = true
		enableOutgoing = true
	}

	scanner := &worker.Scanner{
		Store:                     st,
		WalletCore:                wc,
		ChainGateway:              cg,
		ProjectNotify:             projectNotify,
		Interval:                  time.Duration(cfg.IntervalSeconds) * time.Second,
		AccountPageSize:           cfg.AccountPageSize,
		AccountMaxPages:           cfg.AccountMaxPages,
		WatchLimit:                cfg.WatchLimit,
		AddrConcurrency:           cfg.AddrConcurrency,
		ReorgWindow:               int64(cfg.ReorgWindow),
		ReorgCandidateLimit:       cfg.ReorgCandidateLimit,
		ReorgNotFoundThreshold:    int64(cfg.ReorgNotFoundThreshold),
		OutgoingNotFoundThreshold: int64(cfg.OutgoingNotFoundThreshold),
		OutgoingNotFoundGrace:     time.Duration(cfg.OutgoingNotFoundGraceSeconds) * time.Second,
		EnableAccountScan:         enableAccountScan,
		EnableUTXOScan:            enableUTXOScan,
		EnableReorgReconcile:      enableReorg,
		EnableOutboxDispatch:      enableOutbox,
		EnableOutgoingScan:        enableOutgoing,
		SweepMinBalance:           cfg.SweepMinBalance,
		ProjectChainIDMap:         cfg.ProjectNotifyChainMap,
		ProjectDefaultChainID:     cfg.ProjectNotifyDefaultID,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("service=scan-account-service mode=%s wallet-core=%s chain-gateway-grpc=%s project-notify=%s interval=%ds addr-concurrency=%d sweep-min-balance=%s reorg-window=%d reorg-candidate-limit=%d reorg-not-found-threshold=%d outgoing-not-found-threshold=%d outgoing-not-found-grace=%ds",
		mode, cfg.WalletCoreAddr, cfg.ChainGatewayGRPCAddr, cfg.ProjectNotifyBaseURL, cfg.IntervalSeconds, cfg.AddrConcurrency, cfg.SweepMinBalance, cfg.ReorgWindow, cfg.ReorgCandidateLimit, cfg.ReorgNotFoundThreshold, cfg.OutgoingNotFoundThreshold, cfg.OutgoingNotFoundGraceSeconds)
	err = scanner.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
