package app

import (
	"net/http"

	"wallet-saas-v2/services/api-gateway/internal/config"
	"wallet-saas-v2/services/api-gateway/internal/handler"
	"wallet-saas-v2/services/api-gateway/internal/server"
)

func Run() error {
	cfg := config.Load()
	proxy := handler.NewProxyHandler(cfg.WalletCoreAddr)
	return http.ListenAndServe(cfg.Addr, server.NewMux(proxy))
}
