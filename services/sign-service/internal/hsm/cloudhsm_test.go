package hsm

import (
	"errors"
	"testing"
)

func TestNewCloudHSMBackendRequiresConfiguration(t *testing.T) {
	_, err := NewCloudHSMBackend(CloudHSMConfig{})
	if err == nil {
		t.Fatalf("expected config validation error")
	}
	if !errors.Is(err, ErrCloudHSMNotConfigured) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloudHSMBackendReturnsNotImplementedUntilRealIntegrationExists(t *testing.T) {
	backend, err := NewCloudHSMBackend(CloudHSMConfig{
		ClusterID: "cluster-1",
		Region:    "ap-southeast-1",
		User:      "crypto-user",
		PIN:       "secret-pin",
		PKCS11Lib: "/opt/cloudhsm/lib/libcloudhsm_pkcs11.so",
	})
	if err != nil {
		t.Fatalf("new cloudhsm backend: %v", err)
	}
	err = func() error {
		_, err := backend.LoadOrCreateSeed("master:ecdsa")
		return err
	}()
	if err == nil {
		t.Fatalf("expected not implemented error")
	}
	if !errors.Is(err, ErrCloudHSMNotImplemented) {
		t.Fatalf("unexpected error: %v", err)
	}
}
