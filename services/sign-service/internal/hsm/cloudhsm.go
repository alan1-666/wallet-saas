package hsm

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrCloudHSMNotConfigured  = errors.New("cloudhsm backend is not configured")
	ErrCloudHSMNotImplemented = errors.New("cloudhsm backend is not implemented yet")
)

type CloudHSMConfig struct {
	ClusterID string
	Region    string
	User      string
	PIN       string
	PKCS11Lib string
}

type CloudHSMBackend struct {
	cfg CloudHSMConfig
}

func NewCloudHSMBackend(cfg CloudHSMConfig) (*CloudHSMBackend, error) {
	cfg.ClusterID = strings.TrimSpace(cfg.ClusterID)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.User = strings.TrimSpace(cfg.User)
	cfg.PIN = strings.TrimSpace(cfg.PIN)
	cfg.PKCS11Lib = strings.TrimSpace(cfg.PKCS11Lib)
	if cfg.ClusterID == "" || cfg.Region == "" || cfg.User == "" || cfg.PIN == "" || cfg.PKCS11Lib == "" {
		return nil, fmt.Errorf("%w: cluster_id/region/user/pin/pkcs11_lib are required", ErrCloudHSMNotConfigured)
	}
	return &CloudHSMBackend{cfg: cfg}, nil
}

func (b *CloudHSMBackend) Close() error {
	return nil
}

func (b *CloudHSMBackend) LoadOrCreateSeed(slotID string) ([]byte, error) {
	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return nil, fmt.Errorf("slot id is required")
	}
	return nil, fmt.Errorf("%w: slot=%s cluster_id=%s region=%s", ErrCloudHSMNotImplemented, slotID, b.cfg.ClusterID, b.cfg.Region)
}
