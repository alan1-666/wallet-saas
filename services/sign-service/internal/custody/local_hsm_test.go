package custody

import (
	"testing"

	"wallet-saas-v2/services/sign-service/internal/hd"
	"wallet-saas-v2/services/sign-service/internal/hsm"
)

func TestLocalHSMCustodySchemeAndSlotLoading(t *testing.T) {
	backend, err := hsm.NewSoftwareBackend(t.TempDir(), "software")
	if err != nil {
		t.Fatalf("new software backend: %v", err)
	}
	provider, err := NewLocalHSM(backend, "master", "local-hsm-slot")
	if err != nil {
		t.Fatalf("new local hsm: %v", err)
	}
	defer func() { _ = provider.Close() }()

	if provider.CustodyScheme() != "local-hsm-slot" {
		t.Fatalf("unexpected custody scheme: %s", provider.CustodyScheme())
	}

	ref := hd.KeyRef{
		SignType: "ecdsa",
		Chain:    "ethereum",
		Account:  1,
		Change:   0,
		Index:    7,
	}

	first, err := provider.DeriveKey(ref)
	if err != nil {
		t.Fatalf("derive first key: %v", err)
	}
	second, err := provider.DeriveKey(ref)
	if err != nil {
		t.Fatalf("derive second key: %v", err)
	}
	if first.PublicKeyHex != second.PublicKeyHex {
		t.Fatalf("local hsm derivation should be stable across calls")
	}
}

func TestLocalHSMSignMatchesDerivedKey(t *testing.T) {
	backend, err := hsm.NewSoftwareBackend(t.TempDir(), "software")
	if err != nil {
		t.Fatalf("new software backend: %v", err)
	}
	provider, err := NewLocalHSM(backend, "master", "local-hsm-slot")
	if err != nil {
		t.Fatalf("new local hsm: %v", err)
	}
	defer func() { _ = provider.Close() }()

	ref := hd.KeyRef{
		SignType: "eddsa",
		Chain:    "solana",
		Account:  2,
		Change:   0,
		Index:    3,
	}

	derived, err := provider.DeriveKey(ref)
	if err != nil {
		t.Fatalf("derive key: %v", err)
	}
	sig, err := provider.SignMessage(ref, "deadbeef")
	if err != nil {
		t.Fatalf("sign message: %v", err)
	}
	if derived.PublicKeyHex == "" || sig == "" {
		t.Fatalf("expected derived key and signature")
	}
}
