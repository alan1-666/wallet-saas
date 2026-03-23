package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr                      string
	SignServiceAddr               string
	SignServiceToken              string
	ChainGatewayGRPC              string
	PostgresDSN                   string
	WithdrawDispatchIntervalMs    int
	WithdrawDispatchBatch         int
	WithdrawDispatchParallelism   int
	WithdrawDispatchMaxAttempts   int
	WithdrawDispatchBaseBackoffMs int
	WithdrawDispatchMaxBackoffMs  int
}

func Load() Config {
	host := os.Getenv("WALLET_CORE_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := 8081
	if p := os.Getenv("WALLET_CORE_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	signAddr := os.Getenv("SIGN_SERVICE_ADDR")
	if signAddr == "" {
		signAddr = "127.0.0.1:9091"
	}

	signToken := os.Getenv("SIGN_SERVICE_TOKEN")
	if signToken == "" {
		signToken = "dev-sign-token"
	}

	chainGatewayGRPC := os.Getenv("CHAIN_GATEWAY_GRPC_ADDR")
	if chainGatewayGRPC == "" {
		chainGatewayGRPC = "127.0.0.1:9082"
	}

	postgresDSN := os.Getenv("WALLET_DB_DSN")
	withdrawDispatchIntervalMs := 1000
	if v := os.Getenv("WALLET_WITHDRAW_DISPATCH_INTERVAL_MS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			withdrawDispatchIntervalMs = parsed
		}
	}
	withdrawDispatchBatch := 8
	if v := os.Getenv("WALLET_WITHDRAW_DISPATCH_BATCH"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			withdrawDispatchBatch = parsed
		}
	}
	withdrawDispatchParallelism := 4
	if v := os.Getenv("WALLET_WITHDRAW_DISPATCH_PARALLELISM"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			withdrawDispatchParallelism = parsed
		}
	}
	withdrawDispatchMaxAttempts := 5
	if v := os.Getenv("WALLET_WITHDRAW_DISPATCH_MAX_ATTEMPTS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			withdrawDispatchMaxAttempts = parsed
		}
	}
	withdrawDispatchBaseBackoffMs := 1000
	if v := os.Getenv("WALLET_WITHDRAW_DISPATCH_BASE_BACKOFF_MS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			withdrawDispatchBaseBackoffMs = parsed
		}
	}
	withdrawDispatchMaxBackoffMs := 30000
	if v := os.Getenv("WALLET_WITHDRAW_DISPATCH_MAX_BACKOFF_MS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			withdrawDispatchMaxBackoffMs = parsed
		}
	}

	return Config{
		HTTPAddr:                      host + ":" + strconv.Itoa(port),
		SignServiceAddr:               signAddr,
		SignServiceToken:              signToken,
		ChainGatewayGRPC:              chainGatewayGRPC,
		PostgresDSN:                   postgresDSN,
		WithdrawDispatchIntervalMs:    withdrawDispatchIntervalMs,
		WithdrawDispatchBatch:         withdrawDispatchBatch,
		WithdrawDispatchParallelism:   withdrawDispatchParallelism,
		WithdrawDispatchMaxAttempts:   withdrawDispatchMaxAttempts,
		WithdrawDispatchBaseBackoffMs: withdrawDispatchBaseBackoffMs,
		WithdrawDispatchMaxBackoffMs:  withdrawDispatchMaxBackoffMs,
	}
}
