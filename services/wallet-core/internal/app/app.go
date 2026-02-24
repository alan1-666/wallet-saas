package app

import (
	"net/http"

	"wallet-saas-v2/services/wallet-core/internal/adapters/auth"
	"wallet-saas-v2/services/wallet-core/internal/adapters/chain"
	"wallet-saas-v2/services/wallet-core/internal/adapters/idempotency"
	"wallet-saas-v2/services/wallet-core/internal/adapters/ledger"
	"wallet-saas-v2/services/wallet-core/internal/adapters/registry"
	"wallet-saas-v2/services/wallet-core/internal/adapters/risk"
	signadapter "wallet-saas-v2/services/wallet-core/internal/adapters/sign"
	"wallet-saas-v2/services/wallet-core/internal/config"
	"wallet-saas-v2/services/wallet-core/internal/handler"
	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
	"wallet-saas-v2/services/wallet-core/internal/server"
)

func Run() error {
	cfg := config.Load()

	signClient, err := signadapter.NewGRPC(cfg.SignServiceAddr)
	if err != nil {
		return err
	}
	defer signClient.Close()

	var riskAdapter ports.RiskPort = risk.NewMock()
	var ledgerAdapter ports.LedgerPort = ledger.NewMock()
	var authAdapter ports.AuthPort = auth.NewMock()
	var idemAdapter ports.IdempotencyPort = idempotency.NewMock()
	var registryAdapter ports.AddressRegistryPort = registry.NewMock()
	if cfg.PostgresDSN != "" {
		pgRisk, err := risk.NewPostgres(cfg.PostgresDSN, cfg.RiskMaxAmount)
		if err != nil {
			return err
		}
		defer pgRisk.Close()
		riskAdapter = pgRisk

		pgLedger, err := ledger.NewPostgres(cfg.PostgresDSN)
		if err != nil {
			return err
		}
		defer pgLedger.Close()
		ledgerAdapter = pgLedger

		pgAuth, err := auth.NewPostgres(cfg.PostgresDSN)
		if err != nil {
			return err
		}
		defer pgAuth.Close()
		authAdapter = pgAuth

		pgIdem, err := idempotency.NewPostgres(cfg.PostgresDSN)
		if err != nil {
			return err
		}
		defer pgIdem.Close()
		idemAdapter = pgIdem

		pgRegistry, err := registry.NewPostgres(cfg.PostgresDSN)
		if err != nil {
			return err
		}
		defer pgRegistry.Close()
		registryAdapter = pgRegistry
	}

	chainClient, err := chain.NewGRPC(cfg.ChainGatewayGRPC)
	if err != nil {
		return err
	}
	defer chainClient.Close()

	orch := &orchestrator.WithdrawOrchestrator{
		Risk:   riskAdapter,
		Ledger: ledgerAdapter,
		Sign:   signClient,
		Chain:  chainClient,
	}

	withdrawHandler := &handler.WithdrawHandler{
		Orchestrator: orch,
		Risk:         riskAdapter,
		Ledger:       ledgerAdapter,
		Auth:         authAdapter,
		Idem:         idemAdapter,
		KeyManager:   signClient,
		ChainAddr:    chainClient,
		Registry:     registryAdapter,
	}
	return http.ListenAndServe(cfg.HTTPAddr, server.NewMux(withdrawHandler))
}
