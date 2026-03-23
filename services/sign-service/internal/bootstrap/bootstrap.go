package bootstrap

import (
	"context"
	"fmt"

	"wallet-saas-v2/services/sign-service/internal/config"
	"wallet-saas-v2/services/sign-service/internal/custody"
	"wallet-saas-v2/services/sign-service/internal/hsm"
	"wallet-saas-v2/services/sign-service/internal/policy"
	grpctransport "wallet-saas-v2/services/sign-service/internal/transport/grpc"
)

func Run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}

	backend, err := hsm.NewBackend(hsm.FactoryConfig{
		Backend:     cfg.HSMBackend,
		LevelDBPath: cfg.LevelDBPath,
		Namespace:   "software",
		CloudHSM: hsm.CloudHSMConfig{
			ClusterID: cfg.CloudHSMClusterID,
			Region:    cfg.CloudHSMRegion,
			User:      cfg.CloudHSMUser,
			PIN:       cfg.CloudHSMPIN,
			PKCS11Lib: cfg.CloudHSMPKCS11Lib,
		},
	})
	if err != nil {
		return err
	}

	var provider custody.Provider
	switch cfg.CustodyProvider {
	case "local-hsm":
		p, err := custody.NewLocalHSM(backend, cfg.HSMSlotPrefix, cfg.CustodyScheme)
		if err != nil {
			_ = backend.Close()
			return err
		}
		provider = p
	default:
		_ = backend.Close()
		return fmt.Errorf("unsupported custody provider: %s", cfg.CustodyProvider)
	}
	defer provider.Close()

	policyEngine := policy.New(policy.Config{
		AuthToken:            cfg.AuthToken,
		RateLimitWindow:      cfg.RateLimitWindow,
		RateLimitMaxRequests: cfg.RateLimitMaxRequests,
	})

	grpcServer := grpctransport.New(cfg, provider, policyEngine)
	return grpcServer.Start(context.Background())
}
