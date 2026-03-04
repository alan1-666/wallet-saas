package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DBDSN                  string
	WalletCoreAddr         string
	ChainGatewayGRPCAddr   string
	APIToken               string
	ProjectNotifyBaseURL   string
	ProjectNotifyToken     string
	ProjectNotifyTimeoutMS int
	ProjectNotifyChainMap  map[string]int64
	ProjectNotifyDefaultID int64
	IntervalSeconds        int
	AccountPageSize        int
	AccountMaxPages        int
	WatchLimit             int
	AddrConcurrency        int
	ReorgWindow            int
	ReorgCandidateLimit    int
	ReorgNotFoundThreshold int
	SweepMinBalance        string
	WalletCoreTimeoutMS    int
	ChainGatewayTimeoutMS  int
}

func Load() Config {
	cfg := Config{
		DBDSN:                  os.Getenv("SCAN_DB_DSN"),
		WalletCoreAddr:         getenv("WALLET_CORE_HTTP_ADDR", getenv("WALLET_API_HTTP_ADDR", "http://127.0.0.1:8080")),
		ChainGatewayGRPCAddr:   getenv("CHAIN_GATEWAY_GRPC_ADDR", "127.0.0.1:9082"),
		APIToken:               os.Getenv("SCAN_API_TOKEN"),
		ProjectNotifyBaseURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("PROJECT_NOTIFY_BASE_URL")), "/"),
		ProjectNotifyToken:     strings.TrimSpace(os.Getenv("PROJECT_NOTIFY_TOKEN")),
		ProjectNotifyTimeoutMS: atoi(getenv("PROJECT_NOTIFY_TIMEOUT_MS", "5000"), 5000),
		ProjectNotifyChainMap:  parseChainIDMap(os.Getenv("PROJECT_NOTIFY_CHAIN_ID_MAP")),
		ProjectNotifyDefaultID: atoi64(getenv("PROJECT_NOTIFY_DEFAULT_CHAIN_ID", "0"), 0),
		IntervalSeconds:        atoi(getenv("SCAN_INTERVAL_SECONDS", "5"), 5),
		AccountPageSize:        atoi(getenv("SCAN_ACCOUNT_PAGE_SIZE", "12"), 12),
		AccountMaxPages:        atoi(getenv("SCAN_ACCOUNT_MAX_PAGES", "1"), 1),
		WatchLimit:             atoi(getenv("SCAN_WATCH_LIMIT", "500"), 500),
		AddrConcurrency:        atoi(getenv("SCAN_ADDR_CONCURRENCY", "2"), 2),
		ReorgWindow:            atoi(getenv("SCAN_REORG_WINDOW", "6"), 6),
		ReorgCandidateLimit:    atoi(getenv("SCAN_REORG_CANDIDATE_LIMIT", "500"), 500),
		ReorgNotFoundThreshold: atoi(getenv("SCAN_REORG_NOT_FOUND_THRESHOLD", "3"), 3),
		SweepMinBalance:        getenv("SCAN_SWEEP_MIN_BALANCE", "50"),
		WalletCoreTimeoutMS:    atoi(getenv("SCAN_WALLET_CORE_TIMEOUT_MS", "10000"), 10000),
		ChainGatewayTimeoutMS:  atoi(getenv("SCAN_CHAIN_GATEWAY_TIMEOUT_MS", "10000"), 10000),
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

func atoi64(v string, fallback int64) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func parseChainIDMap(raw string) map[string]int64 {
	out := make(map[string]int64)
	for _, item := range strings.Split(raw, ",") {
		pair := strings.TrimSpace(item)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		if key == "" {
			continue
		}
		val, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || val <= 0 {
			continue
		}
		out[key] = val
	}
	return out
}
