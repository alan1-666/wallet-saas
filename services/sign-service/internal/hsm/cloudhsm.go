package hsm

import (
	"errors"
	"fmt"
	"strings"
	"sync"
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
	cfg             CloudHSMConfig
	sessionProvider PKCS11Provider

	mu      sync.Mutex
	session PKCS11Session
}

func NewCloudHSMBackend(cfg CloudHSMConfig) (*CloudHSMBackend, error) {
	return NewCloudHSMBackendWithSessionProvider(cfg, NewDefaultPKCS11Provider())
}

func NewCloudHSMBackendWithSessionProvider(cfg CloudHSMConfig, provider PKCS11Provider) (*CloudHSMBackend, error) {
	cfg.ClusterID = strings.TrimSpace(cfg.ClusterID)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.User = strings.TrimSpace(cfg.User)
	cfg.PIN = strings.TrimSpace(cfg.PIN)
	cfg.PKCS11Lib = strings.TrimSpace(cfg.PKCS11Lib)
	if cfg.ClusterID == "" || cfg.Region == "" || cfg.User == "" || cfg.PIN == "" || cfg.PKCS11Lib == "" {
		return nil, fmt.Errorf("%w: cluster_id/region/user/pin/pkcs11_lib are required", ErrCloudHSMNotConfigured)
	}
	if provider == nil {
		provider = NewDefaultPKCS11Provider()
	}
	return &CloudHSMBackend{cfg: cfg, sessionProvider: provider}, nil
}

func (b *CloudHSMBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.session != nil {
		err := b.session.Close()
		b.session = nil
		return err
	}
	return nil
}

func (b *CloudHSMBackend) LoadOrCreateSeed(slotID string) ([]byte, error) {
	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return nil, fmt.Errorf("slot id is required")
	}
	session, err := b.openSession()
	if err != nil {
		return nil, err
	}

	seed, err := session.LoadSeed(slotID)
	switch {
	case err == nil && len(seed) > 0:
		return append([]byte(nil), seed...), nil
	case err != nil && !errors.Is(err, ErrPKCS11ObjectNotFound):
		return nil, err
	}

	return nil, fmt.Errorf("seed slot %q is not provisioned; provision it via the admin bootstrap path", slotID)
}

func (b *CloudHSMBackend) openSession() (PKCS11Session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.session != nil {
		return b.session, nil
	}
	session, err := b.sessionProvider.Open(PKCS11Config{
		ClusterID:  b.cfg.ClusterID,
		Region:     b.cfg.Region,
		User:       b.cfg.User,
		PIN:        b.cfg.PIN,
		ModulePath: b.cfg.PKCS11Lib,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: cluster_id=%s region=%s", err, b.cfg.ClusterID, b.cfg.Region)
	}
	b.session = session
	return b.session, nil
}
