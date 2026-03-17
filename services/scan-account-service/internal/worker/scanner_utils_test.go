package worker

import (
	"testing"
	"time"
)

func TestShouldSkipInternalTransfer(t *testing.T) {
	managed := map[string]struct{}{
		"from-managed": {},
	}
	if !shouldSkipInternalTransfer("from-managed", managed) {
		t.Fatalf("expected managed address to be treated as internal")
	}
	if shouldSkipInternalTransfer("external-address", managed) {
		t.Fatalf("did not expect external address to be treated as internal")
	}
}

func TestShouldFailOutgoingNotFound(t *testing.T) {
	if shouldFailOutgoingNotFound("ethereum", time.Minute, 5, 3, 30*time.Second) {
		t.Fatalf("non-solana chains should not use missing-tx watchdog failure")
	}
	if shouldFailOutgoingNotFound("solana", 10*time.Second, 3, 3, 30*time.Second) {
		t.Fatalf("solana tx should not fail before grace period")
	}
	if shouldFailOutgoingNotFound("solana", time.Minute, 2, 3, 30*time.Second) {
		t.Fatalf("solana tx should not fail before threshold")
	}
	if !shouldFailOutgoingNotFound("solana", time.Minute, 3, 3, 30*time.Second) {
		t.Fatalf("solana tx should fail after threshold and grace")
	}
}
