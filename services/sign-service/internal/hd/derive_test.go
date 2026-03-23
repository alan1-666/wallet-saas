package hd

import (
	"bytes"
	"encoding/hex"
	"testing"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func TestParseAndBuildKeyID(t *testing.T) {
	keyID := BuildKeyID("ecdsa", "ethereum", 12, 0, 7)
	ref, err := ParseKeyID(keyID)
	if err != nil {
		t.Fatalf("parse key id: %v", err)
	}
	if ref.SignType != "ecdsa" || ref.Chain != "ethereum" || ref.Account != 12 || ref.Change != 0 || ref.Index != 7 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
	path, err := ref.DerivationPath()
	if err != nil {
		t.Fatalf("derivation path: %v", err)
	}
	if path != "m/44'/60'/12'/0/7" {
		t.Fatalf("unexpected derivation path: %s", path)
	}
}

func TestDeriveECDSAKeyIsDeterministic(t *testing.T) {
	seed := bytes.Repeat([]byte{0x11}, 64)
	ref := KeyRef{
		SignType: "ecdsa",
		Chain:    "ethereum",
		Account:  1,
		Change:   0,
		Index:    3,
	}
	first, err := DerivePublicKey(seed, ref)
	if err != nil {
		t.Fatalf("derive first key: %v", err)
	}
	second, err := DerivePublicKey(seed, ref)
	if err != nil {
		t.Fatalf("derive second key: %v", err)
	}
	if first.KeyID != ref.ID() {
		t.Fatalf("unexpected key id: %s", first.KeyID)
	}
	if first.PublicKeyHex != second.PublicKeyHex || first.AccountPublicKeyHex != second.AccountPublicKeyHex {
		t.Fatalf("ecdsa derivation is not deterministic")
	}
	if !first.PublicDerivationSupported {
		t.Fatalf("expected ecdsa derivation to expose public derivation material")
	}
	if len(first.PublicKeyHex) != 66 {
		t.Fatalf("unexpected ecdsa public key length: %d", len(first.PublicKeyHex))
	}
	if len(first.AlternatePublicKey) != 130 {
		t.Fatalf("unexpected uncompressed ecdsa public key length: %d", len(first.AlternatePublicKey))
	}
	if len(first.AccountPublicKeyHex) != 66 || len(first.AccountChainCodeHex) != 64 {
		t.Fatalf("unexpected account derivation material")
	}
	other, err := DerivePublicKey(seed, KeyRef{
		SignType: "ecdsa",
		Chain:    "ethereum",
		Account:  1,
		Change:   0,
		Index:    4,
	})
	if err != nil {
		t.Fatalf("derive other key: %v", err)
	}
	if other.PublicKeyHex == first.PublicKeyHex {
		t.Fatalf("expected different address index to produce different key")
	}
}

func TestDeriveEdDSAKeyIsDeterministic(t *testing.T) {
	seed := bytes.Repeat([]byte{0x22}, 64)
	ref := KeyRef{
		SignType: "eddsa",
		Chain:    "solana",
		Account:  2,
		Change:   0,
		Index:    5,
	}
	first, err := DerivePublicKey(seed, ref)
	if err != nil {
		t.Fatalf("derive first key: %v", err)
	}
	second, err := DerivePublicKey(seed, ref)
	if err != nil {
		t.Fatalf("derive second key: %v", err)
	}
	if first.PublicKeyHex != second.PublicKeyHex {
		t.Fatalf("eddsa derivation is not deterministic")
	}
	if first.DerivationPath != "m/44'/501'/2'/0'/5'" {
		t.Fatalf("unexpected eddsa derivation path: %s", first.DerivationPath)
	}
	if len(first.PublicKeyHex) != 64 {
		t.Fatalf("unexpected eddsa public key length: %d", len(first.PublicKeyHex))
	}
	if first.PublicDerivationSupported {
		t.Fatalf("eddsa should not advertise public derivation support")
	}
}

func TestDeriveSigningKeyMatchesDerivedPublicKey(t *testing.T) {
	seed := bytes.Repeat([]byte{0x33}, 64)
	ref := KeyRef{
		SignType: "ecdsa",
		Chain:    "ethereum",
		Account:  3,
		Change:   0,
		Index:    9,
	}
	publicKey, err := DerivePublicKey(seed, ref)
	if err != nil {
		t.Fatalf("derive public key: %v", err)
	}
	signingKey, err := DeriveSigningKey(seed, ref)
	if err != nil {
		t.Fatalf("derive signing key: %v", err)
	}
	rawPriv, err := hex.DecodeString(signingKey.PrivateKeyHex)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	priv, err := gethcrypto.ToECDSA(rawPriv)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}
	gotCompressed := hex.EncodeToString(gethcrypto.CompressPubkey(&priv.PublicKey))
	gotUncompressed := hex.EncodeToString(gethcrypto.FromECDSAPub(&priv.PublicKey))
	if gotCompressed != publicKey.PublicKeyHex || gotUncompressed != publicKey.AlternatePublicKey {
		t.Fatalf("public and signing derivation mismatch")
	}
}
