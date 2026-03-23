package keystore

import (
	"bytes"
	"testing"
)

func TestLoadOrCreateSeedStableAcrossCalls(t *testing.T) {
	keys, err := New(t.TempDir(), "software")
	if err != nil {
		t.Fatalf("new keystore: %v", err)
	}
	defer func() { _ = keys.Close() }()

	first, err := keys.LoadOrCreateSeed("master:ecdsa")
	if err != nil {
		t.Fatalf("load first seed: %v", err)
	}
	second, err := keys.LoadOrCreateSeed("master:ecdsa")
	if err != nil {
		t.Fatalf("load second seed: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("stored seed should be stable across calls")
	}
}

func TestLoadOrCreateSeedUsesDifferentSlots(t *testing.T) {
	keys, err := New(t.TempDir(), "software")
	if err != nil {
		t.Fatalf("new keystore: %v", err)
	}
	defer func() { _ = keys.Close() }()

	ecdsaSeed, err := keys.LoadOrCreateSeed("master:ecdsa")
	if err != nil {
		t.Fatalf("load ecdsa seed: %v", err)
	}
	eddsaSeed, err := keys.LoadOrCreateSeed("master:eddsa")
	if err != nil {
		t.Fatalf("load eddsa seed: %v", err)
	}
	if bytes.Equal(ecdsaSeed, eddsaSeed) {
		t.Fatalf("different slots should use different seed material")
	}
}
