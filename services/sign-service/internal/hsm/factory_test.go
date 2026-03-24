package hsm

import (
	"errors"
	"testing"
)

func TestNewBackendCreatesSoftwareBackendByDefault(t *testing.T) {
	backend, err := NewBackend(FactoryConfig{
		Software: SoftwareConfig{
			Path:       t.TempDir(),
			Namespace:  "software",
			Password:   "test-password",
			AutoCreate: true,
		},
	})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	seed, err := backend.LoadOrCreateSeed("master:ecdsa")
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if len(seed) == 0 {
		t.Fatalf("expected non-empty seed")
	}
}

func TestNewBackendValidatesCloudHSMConfig(t *testing.T) {
	_, err := NewBackend(FactoryConfig{
		Backend:  "cloudhsm",
		CloudHSM: CloudHSMConfig{},
	})
	if err == nil {
		t.Fatalf("expected cloudhsm config error")
	}
	if !errors.Is(err, ErrCloudHSMNotConfigured) {
		t.Fatalf("unexpected error: %v", err)
	}
}
