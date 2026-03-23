package hsm

import (
	"bytes"
	"testing"
)

func TestSoftwareBackendLoadOrCreateSeed(t *testing.T) {
	backend, err := NewSoftwareBackend(t.TempDir(), "software")
	if err != nil {
		t.Fatalf("new software backend: %v", err)
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
		t.Fatalf("software backend should return stable seed for same slot")
	}
}
