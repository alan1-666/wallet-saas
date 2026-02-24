package config

import "os"

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	AccountCfgPath string
	UtxoCfgPath    string
}

func Load() Config {
	httpAddr := os.Getenv("CHAIN_GATEWAY_ADDR")
	if httpAddr == "" {
		httpAddr = ":8082"
	}
	grpcAddr := os.Getenv("CHAIN_GATEWAY_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":9082"
	}

	accountCfg := os.Getenv("CHAIN_ACCOUNT_CONFIG_PATH")
	if accountCfg == "" {
		accountCfg = "/app/legacy/wallet-chain-account/config.yml"
	}

	utxoCfg := os.Getenv("CHAIN_UTXO_CONFIG_PATH")
	if utxoCfg == "" {
		utxoCfg = "/app/legacy/wallet-chain-utxo/config.yml"
	}

	return Config{HTTPAddr: httpAddr, GRPCAddr: grpcAddr, AccountCfgPath: accountCfg, UtxoCfgPath: utxoCfg}
}
