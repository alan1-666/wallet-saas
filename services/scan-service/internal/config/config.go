package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBDSN                 string
	WalletCoreAddr        string
	ChainGatewayGRPCAddr  string
	APIToken              string
	IntervalSeconds       int
	AccountPageSize       int
	AccountMaxPages       int
	WatchLimit            int
	AddrConcurrency       int
	SweepMinBalance       string
	WalletCoreTimeoutMS   int
	ChainGatewayTimeoutMS int
}

func Load() Config {
	cfg := Config{
		DBDSN:                 os.Getenv("SCAN_DB_DSN"),
		WalletCoreAddr:        getenv("WALLET_CORE_HTTP_ADDR", getenv("WALLET_API_HTTP_ADDR", "http://127.0.0.1:8080")),
		ChainGatewayGRPCAddr:  getenv("CHAIN_GATEWAY_GRPC_ADDR", "127.0.0.1:9082"),
		APIToken:              os.Getenv("SCAN_API_TOKEN"),
		IntervalSeconds:       atoi(getenv("SCAN_INTERVAL_SECONDS", "5"), 5),
		AccountPageSize:       atoi(getenv("SCAN_ACCOUNT_PAGE_SIZE", "12"), 12),
		AccountMaxPages:       atoi(getenv("SCAN_ACCOUNT_MAX_PAGES", "1"), 1),
		WatchLimit:            atoi(getenv("SCAN_WATCH_LIMIT", "500"), 500),
		AddrConcurrency:       atoi(getenv("SCAN_ADDR_CONCURRENCY", "2"), 2),
		SweepMinBalance:       getenv("SCAN_SWEEP_MIN_BALANCE", "50"),
		WalletCoreTimeoutMS:   atoi(getenv("SCAN_WALLET_CORE_TIMEOUT_MS", "10000"), 10000),
		ChainGatewayTimeoutMS: atoi(getenv("SCAN_CHAIN_GATEWAY_TIMEOUT_MS", "10000"), 10000),
	}
	return cfg
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func atoi(v string, fallback int) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
