package hsm

import (
	"bytes"
	"errors"
	"testing"
)

type fakePKCS11Provider struct {
	openCount int
	session   *fakePKCS11Session
}

func (p *fakePKCS11Provider) Open(cfg PKCS11Config) (PKCS11Session, error) {
	p.openCount++
	if p.session == nil {
		p.session = &fakePKCS11Session{slots: make(map[string][]byte)}
	}
	return p.session, nil
}

type fakePKCS11Session struct {
	slots  map[string][]byte
	closed bool
}

func (s *fakePKCS11Session) Close() error {
	s.closed = true
	return nil
}

func (s *fakePKCS11Session) LoadSeed(slotID string) ([]byte, error) {
	seed, ok := s.slots[slotID]
	if !ok {
		return nil, ErrPKCS11ObjectNotFound
	}
	return append([]byte(nil), seed...), nil
}

func (s *fakePKCS11Session) StoreSeed(slotID string, seed []byte) error {
	s.slots[slotID] = append([]byte(nil), seed...)
	return nil
}

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

func TestCloudHSMBackendUsesPKCS11SessionProvider(t *testing.T) {
	fakeProvider := &fakePKCS11Provider{}
	backend, err := NewCloudHSMBackendWithSessionProvider(CloudHSMConfig{
		ClusterID: "cluster-1",
		Region:    "ap-southeast-1",
		User:      "crypto-user",
		PIN:       "secret-pin",
		PKCS11Lib: "/opt/cloudhsm/lib/libcloudhsm_pkcs11.so",
	}, fakeProvider)
	if err != nil {
		t.Fatalf("new cloudhsm backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	first, err := backend.LoadOrCreateSeed("master:ecdsa")
	if err != nil {
		t.Fatalf("load first seed: %v", err)
	}
	second, err := backend.LoadOrCreateSeed("master:ecdsa")
	if err != nil {
		t.Fatalf("load second seed: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("expected stable seed across repeated session loads")
	}
	if fakeProvider.openCount != 1 {
		t.Fatalf("expected provider to open a single session, got %d", fakeProvider.openCount)
	}
}
