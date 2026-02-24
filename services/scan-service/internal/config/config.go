package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBDSN            string
	WalletCoreAddr   string
	ChainGatewayAddr string
	EthRPCURL        string
	APIToken         string
	IntervalSeconds  int
	MinConfirmations int64
	AccountPageSize  int
	AccountMaxPages  int
	WatchLimit       int
	EthLookback      int
	EthMaxBlocksTick int
	EthRescanBlocks  int
}

func Load() Config {
	cfg := Config{
		DBDSN:            os.Getenv("SCAN_DB_DSN"),
		WalletCoreAddr:   getenv("WALLET_CORE_HTTP_ADDR", "http://127.0.0.1:8081"),
		ChainGatewayAddr: getenv("CHAIN_GATEWAY_HTTP_ADDR", "http://127.0.0.1:8082"),
		EthRPCURL:        os.Getenv("SCAN_ETH_RPC_URL"),
		APIToken:         os.Getenv("SCAN_API_TOKEN"),
		IntervalSeconds:  atoi(getenv("SCAN_INTERVAL_SECONDS", "5"), 5),
		MinConfirmations: int64(atoi(getenv("SCAN_MIN_CONFIRMATIONS", "1"), 1)),
		AccountPageSize:  atoi(getenv("SCAN_ACCOUNT_PAGE_SIZE", "50"), 50),
		AccountMaxPages:  atoi(getenv("SCAN_ACCOUNT_MAX_PAGES", "2"), 2),
		WatchLimit:       atoi(getenv("SCAN_WATCH_LIMIT", "500"), 500),
		EthLookback:      atoi(getenv("SCAN_ETH_LOOKBACK_BLOCKS", "300"), 300),
		EthMaxBlocksTick: atoi(getenv("SCAN_ETH_MAX_BLOCKS_PER_TICK", "80"), 80),
		EthRescanBlocks:  atoi(getenv("SCAN_ETH_RESCAN_BLOCKS", "500"), 500),
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
