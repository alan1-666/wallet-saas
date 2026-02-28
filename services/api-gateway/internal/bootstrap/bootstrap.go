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
	return http.ListenAndServe(cfg.Addr, httptransport.NewMux(proxy))
}
