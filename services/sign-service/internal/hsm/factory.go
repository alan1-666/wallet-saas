package hsm

import (
	"fmt"
	"strings"
)

type FactoryConfig struct {
	Backend     string
	LevelDBPath string
	Namespace   string
	CloudHSM    CloudHSMConfig
}

func NewBackend(cfg FactoryConfig) (Backend, error) {
	switch strings.TrimSpace(cfg.Backend) {
	case "", "software":
		return NewSoftwareBackend(cfg.LevelDBPath, cfg.Namespace)
	case "cloudhsm":
		return NewCloudHSMBackend(cfg.CloudHSM)
	default:
		return nil, fmt.Errorf("unsupported hsm backend: %s", cfg.Backend)
	}
}
