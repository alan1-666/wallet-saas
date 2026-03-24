package keystore

import (
	"bytes"
	"testing"
)

func TestLoadOrCreateSeedStableAcrossCalls(t *testing.T) {
	keys, err := New(t.TempDir(), "software", "test-password", true)
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
	keys, err := New(t.TempDir(), "software", "test-password", true)
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

func TestProvisionSeedRequiresExplicitControlWhenAutoCreateDisabled(t *testing.T) {
	keys, err := New(t.TempDir(), "software", "test-password", false)
	if err != nil {
		t.Fatalf("new keystore: %v", err)
	}
	defer func() { _ = keys.Close() }()

	if _, err := keys.LoadOrCreateSeed("tenant-a:ecdsa"); err != ErrSeedNotProvisioned {
		t.Fatalf("expected not provisioned error, got %v", err)
	}
	if err := keys.ProvisionSeed("tenant-a:ecdsa", bytes.Repeat([]byte{0x42}, 64)); err != nil {
		t.Fatalf("provision seed: %v", err)
	}
	seed, err := keys.LoadOrCreateSeed("tenant-a:ecdsa")
	if err != nil {
		t.Fatalf("load provisioned seed: %v", err)
	}
	if !bytes.Equal(seed, bytes.Repeat([]byte{0x42}, 64)) {
		t.Fatalf("unexpected provisioned seed content")
	}
}

func TestLoadSeedExportsProvisionedSeed(t *testing.T) {
	keys, err := New(t.TempDir(), "software", "test-password", false)
	if err != nil {
		t.Fatalf("new keystore: %v", err)
	}
	defer func() { _ = keys.Close() }()

	expected := bytes.Repeat([]byte{0x24}, 64)
	if err := keys.ProvisionSeed("tenant-a:ecdsa", expected); err != nil {
		t.Fatalf("provision seed: %v", err)
	}
	got, err := keys.LoadSeed("tenant-a:ecdsa")
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("exported seed mismatch")
	}
}

func TestReplaceSeedOverwritesProvisionedSeed(t *testing.T) {
	keys, err := New(t.TempDir(), "software", "test-password", false)
	if err != nil {
		t.Fatalf("new keystore: %v", err)
	}
	defer func() { _ = keys.Close() }()

	if err := keys.ProvisionSeed("tenant-a:ecdsa", bytes.Repeat([]byte{0x42}, 64)); err != nil {
		t.Fatalf("provision seed: %v", err)
	}
	expected := bytes.Repeat([]byte{0x99}, 64)
	if err := keys.ReplaceSeed("tenant-a:ecdsa", expected); err != nil {
		t.Fatalf("replace seed: %v", err)
	}
	got, err := keys.LoadSeed("tenant-a:ecdsa")
	if err != nil {
		t.Fatalf("load replaced seed: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("replaced seed mismatch")
	}
}
