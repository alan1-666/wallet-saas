package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr         string
	SignServiceAddr  string
	ChainGatewayHTTP string
	ChainGatewayGRPC string
	PostgresDSN      string
	RiskMaxAmount    string
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

	chainGateway := os.Getenv("CHAIN_GATEWAY_HTTP_ADDR")
	if chainGateway == "" {
		chainGateway = "http://127.0.0.1:8082"
	}
	chainGatewayGRPC := os.Getenv("CHAIN_GATEWAY_GRPC_ADDR")
	if chainGatewayGRPC == "" {
		chainGatewayGRPC = "127.0.0.1:9082"
	}

	postgresDSN := os.Getenv("WALLET_DB_DSN")
	riskMaxAmount := os.Getenv("RISK_MAX_WITHDRAW_AMOUNT")
	if riskMaxAmount == "" {
		riskMaxAmount = "1000000000000"
	}

	return Config{
		HTTPAddr:         host + ":" + strconv.Itoa(port),
		SignServiceAddr:  signAddr,
		ChainGatewayHTTP: chainGateway,
		ChainGatewayGRPC: chainGatewayGRPC,
		PostgresDSN:      postgresDSN,
		RiskMaxAmount:    riskMaxAmount,
	}
}
