package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GRPCHost             string
	GRPCPort             int
	LevelDBPath          string
	AuthToken            string
	RateLimitWindow      time.Duration
	RateLimitMaxRequests int
	CustodyProvider      string
	CustodyScheme        string
	HSMBackend           string
	HSMSlotPrefix        string
	CloudHSMClusterID    string
	CloudHSMRegion       string
	CloudHSMUser         string
	CloudHSMPIN          string
	CloudHSMPKCS11Lib    string
}

func Load() Config {
	host := os.Getenv("SIGN_GRPC_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := 9091
	if p := os.Getenv("SIGN_GRPC_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	levelPath := os.Getenv("SIGN_LEVELDB_PATH")
	if levelPath == "" {
		levelPath = "./data/sign-leveldb"
	}

	authToken := strings.TrimSpace(os.Getenv("SIGN_AUTH_TOKEN"))
	if authToken == "" {
		authToken = "dev-sign-token"
	}

	rateLimitWindow := 60 * time.Second
	if raw := strings.TrimSpace(os.Getenv("SIGN_RATE_LIMIT_WINDOW_SECONDS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			rateLimitWindow = time.Duration(parsed) * time.Second
		}
	}

	rateLimitMaxRequests := 300
	if raw := strings.TrimSpace(os.Getenv("SIGN_RATE_LIMIT_MAX_REQUESTS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			rateLimitMaxRequests = parsed
		}
	}

	custodyProvider := strings.TrimSpace(os.Getenv("SIGN_CUSTODY_PROVIDER"))
	if custodyProvider == "" {
		custodyProvider = "local-hsm"
	}

	custodyScheme := strings.TrimSpace(os.Getenv("SIGN_CUSTODY_SCHEME"))
	if custodyScheme == "" {
		custodyScheme = "local-hsm-slot"
	}

	hsmBackend := strings.TrimSpace(os.Getenv("SIGN_HSM_BACKEND"))
	if hsmBackend == "" {
		hsmBackend = "software"
	}

	hsmSlotPrefix := strings.TrimSpace(os.Getenv("SIGN_HSM_SLOT_PREFIX"))
	if hsmSlotPrefix == "" {
		hsmSlotPrefix = "master"
	}

	cloudHSMClusterID := strings.TrimSpace(os.Getenv("SIGN_CLOUDHSM_CLUSTER_ID"))
	cloudHSMRegion := strings.TrimSpace(os.Getenv("SIGN_CLOUDHSM_REGION"))
	cloudHSMUser := strings.TrimSpace(os.Getenv("SIGN_CLOUDHSM_USER"))
	cloudHSMPIN := strings.TrimSpace(os.Getenv("SIGN_CLOUDHSM_PIN"))
	cloudHSMPKCS11Lib := strings.TrimSpace(os.Getenv("SIGN_CLOUDHSM_PKCS11_LIB"))

	return Config{
		GRPCHost:             host,
		GRPCPort:             port,
		LevelDBPath:          levelPath,
		AuthToken:            authToken,
		RateLimitWindow:      rateLimitWindow,
		RateLimitMaxRequests: rateLimitMaxRequests,
		CustodyProvider:      custodyProvider,
		CustodyScheme:        custodyScheme,
		HSMBackend:           hsmBackend,
		HSMSlotPrefix:        hsmSlotPrefix,
		CloudHSMClusterID:    cloudHSMClusterID,
		CloudHSMRegion:       cloudHSMRegion,
		CloudHSMUser:         cloudHSMUser,
		CloudHSMPIN:          cloudHSMPIN,
		CloudHSMPKCS11Lib:    cloudHSMPKCS11Lib,
	}
}

func (c Config) Validate() error {
	switch strings.TrimSpace(c.CustodyProvider) {
	case "", "local-hsm":
	default:
		return fmt.Errorf("unsupported custody provider: %s", c.CustodyProvider)
	}

	switch strings.TrimSpace(c.HSMBackend) {
	case "", "software":
		return nil
	case "cloudhsm":
		if strings.TrimSpace(c.CloudHSMClusterID) == "" ||
			strings.TrimSpace(c.CloudHSMRegion) == "" ||
			strings.TrimSpace(c.CloudHSMUser) == "" ||
			strings.TrimSpace(c.CloudHSMPIN) == "" ||
			strings.TrimSpace(c.CloudHSMPKCS11Lib) == "" {
			return fmt.Errorf("cloudhsm backend requires SIGN_CLOUDHSM_CLUSTER_ID, SIGN_CLOUDHSM_REGION, SIGN_CLOUDHSM_USER, SIGN_CLOUDHSM_PIN, SIGN_CLOUDHSM_PKCS11_LIB")
		}
		return nil
	default:
		return fmt.Errorf("unsupported hsm backend: %s", c.HSMBackend)
	}
}
