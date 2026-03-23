package bootstrap

import (
	"net/http"

	"wallet-saas-v2/services/wallet-core/internal/adapters/auth"
	"wallet-saas-v2/services/wallet-core/internal/adapters/chain"
	"wallet-saas-v2/services/wallet-core/internal/adapters/ledger"
	"wallet-saas-v2/services/wallet-core/internal/adapters/registry"
	signadapter "wallet-saas-v2/services/wallet-core/internal/adapters/sign"
	"wallet-saas-v2/services/wallet-core/internal/config"
	"wallet-saas-v2/services/wallet-core/internal/orchestrator"
	"wallet-saas-v2/services/wallet-core/internal/ports"
	httptransport "wallet-saas-v2/services/wallet-core/internal/transport/http"
)

func Run() error {
	cfg := config.Load()

	signClient, err := signadapter.NewGRPC(cfg.SignServiceAddr, cfg.SignServiceToken)
	if err != nil {
		return err
	}
	defer signClient.Close()

	var ledgerAdapter ports.LedgerPort = ledger.NewMock()
	var authAdapter ports.AuthPort = auth.NewMock()
	var registryAdapter ports.AddressRegistryPort = registry.NewMock()
	if cfg.PostgresDSN != "" {
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
		Ledger: ledgerAdapter,
		Sign:   signClient,
		Chain:  chainClient,
	}

	withdrawHandler := &httptransport.WithdrawHandler{
		Orchestrator: orch,
		Ledger:       ledgerAdapter,
		Auth:         authAdapter,
		KeyManager:   signClient,
		ChainAddr:    chainClient,
		Registry:     registryAdapter,
	}
	return http.ListenAndServe(cfg.HTTPAddr, httptransport.NewMux(withdrawHandler))
}
