package grpctransport

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"wallet-saas-v2/services/sign-service/internal/config"
	"wallet-saas-v2/services/sign-service/internal/hd"
	pb "wallet-saas-v2/services/sign-service/internal/pb"
	"wallet-saas-v2/services/sign-service/internal/policy"
)

type fakeCustody struct {
	scheme          string
	lastTenantID    string
	lastDeriveRef   hd.KeyRef
	lastSignRef     hd.KeyRef
	lastMessageHash string
}

func (f *fakeCustody) Close() error { return nil }

func (f *fakeCustody) CustodyScheme() string { return f.scheme }

func (f *fakeCustody) DeriveKey(tenantID string, ref hd.KeyRef) (hd.DerivedKey, error) {
	f.lastTenantID = tenantID
	f.lastDeriveRef = ref
	return hd.DerivedKey{
		KeyID:                     ref.ID(),
		DerivationPath:            "m/44'/60'/1'/0/7",
		PublicKeyHex:              "028d998f2c0b1f2c9a9d8f7e6c5b4a392817161514131211100f0e0d0c0b0a09ff",
		AlternatePublicKey:        "048d998f2c0b1f2c9a9d8f7e6c5b4a392817161514131211100f0e0d0c0b0a09ff8d998f2c0b1f2c9a9d8f7e6c5b4a392817161514131211100f0e0d0c0b0a09ff",
		PublicDerivationSupported: true,
		AccountPublicKeyHex:       "020102030405060708090001020304050607080900010203040506070809000102",
		AccountAlternatePublicKey: "0401020304050607080900010203040506070809000102030405060708090001020102030405060708090001020304050607080900010203040506070809000102",
		AccountChainCodeHex:       "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
		AccountDerivationPath:     "m/44'/60'/1'",
	}, nil
}

func (f *fakeCustody) SignMessage(tenantID string, ref hd.KeyRef, messageHash string) (string, error) {
	f.lastTenantID = tenantID
	f.lastSignRef = ref
	f.lastMessageHash = messageHash
	return "deadbeefsignature", nil
}

func TestDeriveKeyUsesCustodyProvider(t *testing.T) {
	provider := &fakeCustody{scheme: "fake-hsm"}
	engine := policy.New(policy.Config{
		AuthToken:            "token-123",
		RateLimitWindow:      time.Minute,
		RateLimitMaxRequests: 10,
	})
	server := New(config.Config{}, provider, engine)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token-123", "x-tenant-id", "tenant-a"))

	resp, err := server.DeriveKey(ctx, &pb.DeriveKeyRequest{
		KeyId:    "hd:ecdsa:ethereum:1:0:7",
		SignType: "ecdsa",
	})
	if err != nil {
		t.Fatalf("derive key: %v", err)
	}
	if resp.GetCustodyScheme() != "fake-hsm" {
		t.Fatalf("unexpected custody scheme: %s", resp.GetCustodyScheme())
	}
	if provider.lastDeriveRef.ID() != "hd:ecdsa:ethereum:1:0:7" {
		t.Fatalf("unexpected derive ref: %+v", provider.lastDeriveRef)
	}
	if provider.lastTenantID != "tenant-a" {
		t.Fatalf("unexpected derive tenant: %s", provider.lastTenantID)
	}
	if resp.GetPublicKey() == nil || resp.GetPublicKey().GetCompressedHex() == "" {
		t.Fatalf("expected derived public key")
	}
}

func TestSignMessageUsesCustodyProvider(t *testing.T) {
	provider := &fakeCustody{scheme: "fake-hsm"}
	engine := policy.New(policy.Config{
		AuthToken:            "token-123",
		RateLimitWindow:      time.Minute,
		RateLimitMaxRequests: 10,
	})
	server := New(config.Config{}, provider, engine)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token-123", "x-tenant-id", "tenant-b"))

	resp, err := server.SignMessage(ctx, &pb.SignMessageRequest{
		KeyId:       "hd:eddsa:solana:2:0:3",
		SignType:    "eddsa",
		MessageHash: "deadbeef",
	})
	if err != nil {
		t.Fatalf("sign message: %v", err)
	}
	if resp.GetSignature() != "deadbeefsignature" {
		t.Fatalf("unexpected signature: %s", resp.GetSignature())
	}
	if provider.lastSignRef.ID() != "hd:eddsa:solana:2:0:3" {
		t.Fatalf("unexpected sign ref: %+v", provider.lastSignRef)
	}
	if provider.lastTenantID != "tenant-b" {
		t.Fatalf("unexpected sign tenant: %s", provider.lastTenantID)
	}
	if provider.lastMessageHash != "deadbeef" {
		t.Fatalf("unexpected sign payload: %s", provider.lastMessageHash)
	}
	if resp.GetCustodyScheme() != "fake-hsm" {
		t.Fatalf("unexpected custody scheme: %s", resp.GetCustodyScheme())
	}
}
