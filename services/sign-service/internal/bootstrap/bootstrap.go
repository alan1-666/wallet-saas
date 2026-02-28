package bootstrap

import (
	"context"

	"wallet-saas-v2/services/sign-service/internal/config"
	"wallet-saas-v2/services/sign-service/internal/keystore"
	grpctransport "wallet-saas-v2/services/sign-service/internal/transport/grpc"
)

func Run() error {
	cfg := config.Load()

	store, err := keystore.New(cfg.LevelDBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	grpcServer := grpctransport.New(cfg, store)
	return grpcServer.Start(context.Background())
}
