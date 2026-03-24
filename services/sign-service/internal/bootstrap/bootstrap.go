package bootstrap

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"wallet-saas-v2/services/sign-service/internal/config"
	"wallet-saas-v2/services/sign-service/internal/custody"
	"wallet-saas-v2/services/sign-service/internal/hsm"
	"wallet-saas-v2/services/sign-service/internal/keystore"
	"wallet-saas-v2/services/sign-service/internal/policy"
	grpctransport "wallet-saas-v2/services/sign-service/internal/transport/grpc"
)

func Run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	backend, err := OpenBackend(cfg)
	if err != nil {
		return err
	}
	if err := BootstrapSeeds(backend, cfg); err != nil {
		_ = backend.Close()
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
		AllowedTenants:       cfg.AllowedTenants,
	})

	grpcServer := grpctransport.New(cfg, provider, policyEngine)
	return grpcServer.Start(context.Background())
}

type bootstrapSeed struct {
	SlotID   string `json:"slot_id"`
	TenantID string `json:"tenant_id"`
	SignType string `json:"sign_type"`
	SeedHex  string `json:"seed_hex"`
}

func OpenBackend(cfg config.Config) (hsm.Backend, error) {
	vaultPassword, err := ResolveVaultPassword(cfg)
	if err != nil {
		return nil, err
	}
	return hsm.NewBackend(hsm.FactoryConfig{
		Backend: cfg.HSMBackend,
		Software: hsm.SoftwareConfig{
			Path:       cfg.LevelDBPath,
			Namespace:  "software",
			Password:   vaultPassword,
			AutoCreate: cfg.SoftwareVaultAutoCreate,
		},
		CloudHSM: hsm.CloudHSMConfig{
			ClusterID: cfg.CloudHSMClusterID,
			Region:    cfg.CloudHSMRegion,
			User:      cfg.CloudHSMUser,
			PIN:       cfg.CloudHSMPIN,
			PKCS11Lib: cfg.CloudHSMPKCS11Lib,
		},
	})
}

func ResolveVaultPassword(cfg config.Config) (string, error) {
	if strings.TrimSpace(cfg.HSMBackend) != "" && strings.TrimSpace(cfg.HSMBackend) != "software" {
		return "", nil
	}
	if password := strings.TrimSpace(cfg.SoftwareVaultPassword); password != "" {
		return password, nil
	}
	if path := strings.TrimSpace(cfg.SoftwareVaultPasswordFile); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	}
	return promptVaultPassword()
}

func promptVaultPassword() (string, error) {
	if _, err := fmt.Fprint(os.Stderr, "Enter sign-service vault password: "); err != nil {
		return "", err
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", err
	}
	password := strings.TrimSpace(line)
	if password == "" {
		return "", fmt.Errorf("vault password is required")
	}
	return password, nil
}

func BootstrapSeeds(backend hsm.Backend, cfg config.Config) error {
	path := strings.TrimSpace(cfg.SoftwareBootstrapFile)
	if path == "" {
		return nil
	}
	provisioner, ok := backend.(hsm.SeedProvisioner)
	if !ok {
		return fmt.Errorf("backend does not support seed provisioning")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []bootstrapSeed
	if err := json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	for _, entry := range entries {
		slotID := strings.TrimSpace(entry.SlotID)
		if slotID == "" {
			slotID = hsm.BuildTenantSlotID(cfg.HSMSlotPrefix, entry.TenantID, entry.SignType)
		}
		seedHex := strings.TrimPrefix(strings.TrimSpace(entry.SeedHex), "0x")
		seed, err := hex.DecodeString(seedHex)
		if err != nil {
			return fmt.Errorf("decode bootstrap seed slot=%s: %w", slotID, err)
		}
		if err := provisioner.ProvisionSeed(slotID, seed); err != nil && err != keystore.ErrSeedAlreadyExists {
			return fmt.Errorf("provision bootstrap seed slot=%s: %w", slotID, err)
		}
	}
	return nil
}
