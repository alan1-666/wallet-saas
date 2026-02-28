package config

import (
	"fmt"
	"os"
)

type Config struct {
	Addr              string
	WalletCoreAddr    string
	PostgresDSN       string
	UpstreamTimeoutMS int
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

	postgresDSN := os.Getenv("API_GATEWAY_DB_DSN")
	if postgresDSN == "" {
		postgresDSN = os.Getenv("WALLET_DB_DSN")
	}
	timeoutMS := 10000
	if v := os.Getenv("API_GATEWAY_UPSTREAM_TIMEOUT_MS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			timeoutMS = n
		}
	}

	return Config{
		Addr:              addr,
		WalletCoreAddr:    walletCoreAddr,
		PostgresDSN:       postgresDSN,
		UpstreamTimeoutMS: timeoutMS,
	}
}
