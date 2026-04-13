package bootstrap

import (
	"log"
	"net/http"
	"time"

	"wallet-saas-v2/services/api-gateway/internal/config"
	"wallet-saas-v2/services/api-gateway/internal/security"
	httptransport "wallet-saas-v2/services/api-gateway/internal/transport/http"
)

func Run() error {
	cfg := config.Load()
	sec := security.Provider(security.NewNoop())
	if cfg.PostgresDSN != "" {
		pg, err := security.NewPostgres(cfg.PostgresDSN)
		if err != nil {
			return err
		}
		defer pg.Close()
		sec = pg
	} else {
		log.Printf("warning: API_GATEWAY_DB_DSN/WALLET_DB_DSN is empty, requests will be rejected")
	}
	proxy := httptransport.NewProxyHandler(cfg.WalletCoreAddr, time.Duration(cfg.UpstreamTimeoutMS)*time.Millisecond, sec)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httptransport.NewMux(proxy),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return srv.ListenAndServe()
}
