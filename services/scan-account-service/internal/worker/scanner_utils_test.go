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

func TestResolveDepositScanStatus(t *testing.T) {
	if got := resolveDepositScanStatus("", -1, 6, 6); got != depositScanStatusSeen {
		t.Fatalf("expected SEEN for unknown pending tx, got %s", got)
	}
	if got := resolveDepositScanStatus("pending", 3, 6, 6); got != depositScanStatusPending {
		t.Fatalf("expected PENDING below min conf, got %s", got)
	}
	if got := resolveDepositScanStatus("confirmed", 6, 6, 6); got != depositScanStatusConfirmed {
		t.Fatalf("expected CONFIRMED at min conf, got %s", got)
	}
	if got := resolveDepositScanStatus("confirmed", 12, 6, 6); got != depositScanStatusFinalized {
		t.Fatalf("expected FINALIZED after reorg window, got %s", got)
	}
	if got := resolveDepositScanStatus("reverted", 0, 6, 6); got != depositScanStatusReorged {
		t.Fatalf("expected REORGED for reverted tx, got %s", got)
	}
}

func TestMapDepositLedgerStatus(t *testing.T) {
	if got := mapDepositLedgerStatus(depositScanStatusFinalized); got != "CONFIRMED" {
		t.Fatalf("expected FINALIZED to map to CONFIRMED, got %s", got)
	}
	if got := mapDepositLedgerStatus(depositScanStatusReorged); got != "REVERTED" {
		t.Fatalf("expected REORGED to map to REVERTED, got %s", got)
	}
	if got := mapDepositLedgerStatus(depositScanStatusSeen); got != "PENDING" {
		t.Fatalf("expected SEEN to map to PENDING, got %s", got)
	}
}

func TestPaginationGuardMaxEmptyPages(t *testing.T) {
	guard := newPaginationGuard(2, 1)
	advance, stop, reason := guard.Observe("", "cursor-1", 0)
	if !advance || stop || reason != "" {
		t.Fatalf("expected first empty page to continue, got advance=%v stop=%v reason=%s", advance, stop, reason)
	}
	advance, stop, reason = guard.Observe("cursor-1", "cursor-2", 0)
	if advance || !stop || reason != "max_empty_pages" {
		t.Fatalf("expected second empty page to stop on max_empty_pages, got advance=%v stop=%v reason=%s", advance, stop, reason)
	}
}

func TestPaginationGuardCursorStall(t *testing.T) {
	guard := newPaginationGuard(0, 1)
	if advance, stop, reason := guard.Observe("", "cursor-1", 1); !advance || stop || reason != "" {
		t.Fatalf("expected first cursor to advance, got advance=%v stop=%v reason=%s", advance, stop, reason)
	}
	if advance, stop, reason := guard.Observe("cursor-1", "cursor-2", 1); !advance || stop || reason != "" {
		t.Fatalf("expected second cursor to advance, got advance=%v stop=%v reason=%s", advance, stop, reason)
	}
	advance, stop, reason := guard.Observe("cursor-2", "cursor-1", 1)
	if advance || !stop || reason != "cursor_stall_guard" {
		t.Fatalf("expected repeated cursor to trigger stall guard, got advance=%v stop=%v reason=%s", advance, stop, reason)
	}
}

func TestSupportsChainWideAccountScan(t *testing.T) {
	if !supportsChainWideAccountScan("ethereum", "ETH") {
		t.Fatalf("expected ethereum native asset to support chain-wide scan")
	}
	if supportsChainWideAccountScan("ethereum", "USDT") {
		t.Fatalf("did not expect ethereum token asset to use chain-wide scan")
	}
	if supportsChainWideAccountScan("solana", "SOL") {
		t.Fatalf("did not expect solana to use chain-wide scan")
	}
}
