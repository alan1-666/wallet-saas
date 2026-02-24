package config

import "os"

type Config struct {
	Addr           string
	WalletCoreAddr string
}

func Load() Config {
	addr := os.Getenv("API_GATEWAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	walletCoreAddr := os.Getenv("WALLET_CORE_HTTP_ADDR")
	if walletCoreAddr == "" {
		walletCoreAddr = "http://127.0.0.1:8081"
	}

	return Config{Addr: addr, WalletCoreAddr: walletCoreAddr}
}
