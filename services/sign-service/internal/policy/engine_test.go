package policy

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthorizeAllowsHDKeyWithValidToken(t *testing.T) {
	engine := New(Config{
		AuthToken:            "token-123",
		RateLimitWindow:      time.Minute,
		RateLimitMaxRequests: 10,
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token-123"))
	decision, err := engine.Authorize(ctx, "derive", "ecdsa", "hd:ecdsa:ethereum:12:0:7")
	if err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	if decision.SignType != "ecdsa" || decision.KeyID != "hd:ecdsa:ethereum:12:0:7" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestAuthorizeRejectsInvalidToken(t *testing.T) {
	engine := New(Config{
		AuthToken:            "token-123",
		RateLimitWindow:      time.Minute,
		RateLimitMaxRequests: 10,
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer wrong-token"))
	_, err := engine.Authorize(ctx, "sign", "ecdsa", "hd:ecdsa:ethereum:12:0:7")
	if err == nil {
		t.Fatalf("expected auth failure")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unexpected error code: %v", status.Code(err))
	}
}

func TestAuthorizeRateLimit(t *testing.T) {
	engine := New(Config{
		AuthToken:            "token-123",
		RateLimitWindow:      time.Hour,
		RateLimitMaxRequests: 1,
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token-123"))
	if _, err := engine.Authorize(ctx, "sign", "ecdsa", "hd:ecdsa:ethereum:12:0:7"); err != nil {
		t.Fatalf("first authorize failed: %v", err)
	}
	_, err := engine.Authorize(ctx, "sign", "ecdsa", "hd:ecdsa:ethereum:12:0:7")
	if err == nil {
		t.Fatalf("expected rate limit failure")
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("unexpected error code: %v", status.Code(err))
	}
}
