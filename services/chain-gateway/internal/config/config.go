package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr               string
	GRPCAddr               string
	PostgresDSN            string
	EndpointRefreshSeconds int
	EndpointProbeSeconds   int
	EndpointFailThreshold  int
	EndpointOpenSeconds    int
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

	postgresDSN := os.Getenv("CHAIN_GATEWAY_DB_DSN")
	if postgresDSN == "" {
		postgresDSN = os.Getenv("WALLET_DB_DSN")
	}

	return Config{
		HTTPAddr:               httpAddr,
		GRPCAddr:               grpcAddr,
		PostgresDSN:            postgresDSN,
		EndpointRefreshSeconds: atoi(os.Getenv("CHAIN_GATEWAY_ENDPOINT_REFRESH_SECONDS"), 15),
		EndpointProbeSeconds:   atoi(os.Getenv("CHAIN_GATEWAY_ENDPOINT_PROBE_SECONDS"), 20),
		EndpointFailThreshold:  atoi(os.Getenv("CHAIN_GATEWAY_ENDPOINT_FAIL_THRESHOLD"), 3),
		EndpointOpenSeconds:    atoi(os.Getenv("CHAIN_GATEWAY_ENDPOINT_OPEN_SECONDS"), 30),
	}
}

func atoi(v string, fallback int) int {
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
