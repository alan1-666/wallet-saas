package app

import (
	"context"

	"wallet-saas-v2/services/sign-service/internal/config"
	"wallet-saas-v2/services/sign-service/internal/keystore"
	"wallet-saas-v2/services/sign-service/internal/server"
)

func Run() error {
	cfg := config.Load()

	store, err := keystore.New(cfg.LevelDBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	grpcServer := server.New(cfg, store)
	return grpcServer.Start(context.Background())
}
