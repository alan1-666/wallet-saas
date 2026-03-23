package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr         string
	SignServiceAddr  string
	SignServiceToken string
	ChainGatewayGRPC string
	PostgresDSN      string
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

	return Config{
		HTTPAddr:         host + ":" + strconv.Itoa(port),
		SignServiceAddr:  signAddr,
		SignServiceToken: signToken,
		ChainGatewayGRPC: chainGatewayGRPC,
		PostgresDSN:      postgresDSN,
	}
}
