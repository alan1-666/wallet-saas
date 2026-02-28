package bootstrap

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	accountaptos "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/aptos"
	accountbtt "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/btt"
	accountsourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/config"
	accountcosmos "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/cosmos"
	accountevm "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/evm"
	accountsol "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/solana"
	accountsui "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/sui"
	accountton "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/ton"
	accounttron "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/tron"
	accountxlm "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/xlm"
	"wallet-saas-v2/services/chain-gateway/internal/adapters/utxo"
	utxobitcoin "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/bitcoin"
	utxobitcoincash "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/bitcoincash"
	utxosourcecfg "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/config"
	utxodash "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/dash"
	utxolitecoin "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/litecoin"
	utxozen "wallet-saas-v2/services/chain-gateway/internal/adapters/utxo/chains/zen"
	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/config"
	"wallet-saas-v2/services/chain-gateway/internal/controlplane"
	"wallet-saas-v2/services/chain-gateway/internal/dispatcher"
	"wallet-saas-v2/services/chain-gateway/internal/endpoint"
	pb "wallet-saas-v2/services/chain-gateway/internal/pb/chaingateway"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
	"wallet-saas-v2/services/chain-gateway/internal/service"
	grpctransport "wallet-saas-v2/services/chain-gateway/internal/transport/grpc"
	httptransport "wallet-saas-v2/services/chain-gateway/internal/transport/http"

	"google.golang.org/grpc"
)

func Run() error {
	cfg := config.Load()
	var endpointManager *endpoint.Manager
	if cfg.PostgresDSN != "" {
		controlPlaneStore, err := controlplane.NewPostgres(cfg.PostgresDSN)
		if err != nil {
			return err
		}
		defer controlPlaneStore.Close()
		endpointManager = endpoint.NewManager(
			controlPlaneStore,
			time.Duration(cfg.EndpointRefreshSeconds)*time.Second,
			time.Duration(cfg.EndpointProbeSeconds)*time.Second,
			time.Duration(cfg.EndpointOpenSeconds)*time.Second,
			cfg.EndpointFailThreshold,
		)
		go endpointManager.Run(context.Background())
		log.Printf("chain-gateway control-plane enabled refresh=%ds probe=%ds fail-threshold=%d open=%ds",
			cfg.EndpointRefreshSeconds, cfg.EndpointProbeSeconds, cfg.EndpointFailThreshold, cfg.EndpointOpenSeconds)
	} else {
		log.Printf("chain-gateway control-plane disabled: CHAIN_GATEWAY_DB_DSN/WALLET_DB_DSN is empty")
	}

	evmReader := accountevm.NewRPCReader(endpointManager)
	solReader := accountsol.NewRPCReader(endpointManager)

	accountPlugins := make([]accountadapter.ChainPlugin, 0, len(evmAccountChains)+1)
	for _, chain := range evmAccountChains {
		accountPlugins = append(accountPlugins, accountevm.New(chain, evmReader))
	}
	accountPlugins = append(accountPlugins, accountsol.New(solReader))
	accountSourceConfig := buildAccountSourceConfig()
	accountPlugins = append(accountPlugins,
		accountaptos.New(accountSourceConfig),
		accountcosmos.New(accountSourceConfig),
		accounttron.New(accountSourceConfig),
		accountsui.New(accountSourceConfig),
		accountton.New(accountSourceConfig),
		accountxlm.New(accountSourceConfig),
		accountbtt.New(accountSourceConfig),
	)
	accountAdapter, err := accountadapter.NewAdapter(accountPlugins...)
	if err != nil {
		return fmt.Errorf("init account adapter: %w", err)
	}

	utxoSourceConfig := buildUTXOSourceConfig()
	utxoPlugins := []utxo.ChainPlugin{
		utxobitcoin.New(utxoSourceConfig),
		utxobitcoincash.New(utxoSourceConfig),
		utxodash.New(utxoSourceConfig),
		utxolitecoin.New(utxoSourceConfig),
		utxozen.New(utxoSourceConfig),
	}
	utxoAdapter, err := utxo.NewAdapter(utxoPlugins...)
	if err != nil {
		return fmt.Errorf("init utxo adapter: %w", err)
	}

	router := dispatcher.NewRouter()
	if err := registerDefaultBindings(router, accountAdapter, utxoAdapter); err != nil {
		return err
	}
	chainSvc := &service.ChainService{
		Router: router,
	}

	chainHandler := &httptransport.ChainHandler{Chain: chainSvc, Endpoints: endpointManager}
	go func() {
		if err := http.ListenAndServe(cfg.HTTPAddr, httptransport.NewMux(chainHandler)); err != nil {
			log.Printf("chain-gateway http stopped: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterChainGatewayServiceServer(grpcSrv, grpctransport.NewGRPC(chainSvc))
	return grpcSrv.Serve(lis)
}

func registerDefaultBindings(router *dispatcher.Router, accountAdapter ports.ChainAdapter, utxoAdapter ports.ChainAdapter) error {
	accountChains := append([]string{}, evmAccountChains...)
	accountChains = append(accountChains, "solana")
	accountChains = append(accountChains, extraAccountChains...)
	for _, chain := range accountChains {
		if err := router.Register(ports.PluginBinding{
			Chain:   clients.NormalizeChain(chain),
			Network: "*",
			Model:   ports.ModelAccount,
			Adapter: accountAdapter,
		}); err != nil {
			return err
		}
	}
	for _, chain := range defaultUTXOChains {
		if err := router.Register(ports.PluginBinding{
			Chain:   clients.NormalizeChain(chain),
			Network: "*",
			Model:   ports.ModelUTXO,
			Adapter: utxoAdapter,
		}); err != nil {
			return err
		}
	}
	return nil
}

var evmAccountChains = []string{
	"ethereum", "binance", "polygon", "arbitrum", "optimism", "linea", "scroll", "mantle", "zksync",
}

var extraAccountChains = []string{
	"cosmos", "tron", "aptos", "sui", "ton", "xlm", "btt",
}

var defaultUTXOChains = []string{
	"bitcoin", "bitcoincash", "dash", "litecoin", "zen",
}

func buildAccountSourceConfig() *accountsourcecfg.Config {
	return &accountsourcecfg.Config{
		NetWork: firstNonEmpty(
			os.Getenv("CHAIN_GATEWAY_ACCOUNT_NETWORK"),
			os.Getenv("CHAIN_GATEWAY_NETWORK"),
			"mainnet",
		),
		Chains: []string{
			"aptos", "cosmos", "sui", "ton", "tron", "xlm", "btt",
		},
		WalletNode: accountsourcecfg.WalletNode{
			Cosmos: accountsourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_COSMOS_RPC_URL"),
				),
				DataApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_COSMOS_DATA_API_URL"),
				),
				DataApiKey: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_COSMOS_DATA_API_KEY"),
				),
			},
			Tron: accountsourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_TRON_RPC_URL"),
				),
				RpcUser: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_TRON_RPC_USER"),
				),
				RpcPass: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_TRON_RPC_PASS"),
				),
				DataApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_TRON_DATA_API_URL"),
				),
				DataApiKey: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_TRON_DATA_API_KEY"),
				),
			},
			Aptos: accountsourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_APTOS_RPC_URL"),
				),
				DataApiKey: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_APTOS_DATA_API_KEY"),
				),
			},
			Sui: accountsourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_SUI_RPC_URL"),
				),
			},
			Ton: accountsourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_TON_RPC_URL"),
				),
				DataApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_TON_DATA_API_URL"),
				),
			},
			Xlm: accountsourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_XLM_RPC_URL"),
				),
				DataApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_XLM_DATA_API_URL"),
				),
			},
			Btt: accountsourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_BTT_RPC_URL"),
				),
				DataApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_BTT_DATA_API_URL"),
				),
				DataApiKey: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_ACCOUNT_BTT_DATA_API_KEY"),
				),
			},
		},
	}
}

func buildUTXOSourceConfig() *utxosourcecfg.Config {
	return &utxosourcecfg.Config{
		NetWork: firstNonEmpty(
			os.Getenv("CHAIN_GATEWAY_UTXO_NETWORK"),
			os.Getenv("CHAIN_GATEWAY_NETWORK"),
			"mainnet",
		),
		Chains: []string{"bitcoin", "bitcoincash", "dash", "litecoin", "zen"},
		WalletNode: utxosourcecfg.WalletNode{
			Btc: utxosourcecfg.Node{
				RpcUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_UTXO_BTC_RPC_URL"),
				),
				RpcUser: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_UTXO_BTC_RPC_USER"),
				),
				RpcPass: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_UTXO_BTC_RPC_PASS"),
				),
				DataApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_UTXO_BTC_DATA_API_URL"),
				),
				DataApiKey: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_UTXO_BTC_DATA_API_KEY"),
				),
				TpApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_UTXO_BTC_TP_API_URL"),
				),
			},
			Zen: utxosourcecfg.Node{
				TpApiUrl: firstNonEmpty(
					os.Getenv("CHAIN_GATEWAY_UTXO_ZEN_TP_API_URL"),
				),
			},
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
