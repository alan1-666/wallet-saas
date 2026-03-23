package ledger

import "testing"

func TestComputeWithdrawable(t *testing.T) {
	got, err := computeWithdrawable("100", "40")
	if err != nil {
		t.Fatalf("computeWithdrawable returned error: %v", err)
	}
	if got != "60" {
		t.Fatalf("expected withdrawable=60, got %s", got)
	}
}

func TestNormalizeDepositLifecycleStatus(t *testing.T) {
	if got := normalizeDepositLifecycleStatus("CONFIRMED", "", 6, 6, 12); got != "CONFIRMED" {
		t.Fatalf("expected CONFIRMED at credit threshold, got %s", got)
	}
	if got := normalizeDepositLifecycleStatus("CONFIRMED", "", 12, 6, 12); got != "FINALIZED" {
		t.Fatalf("expected FINALIZED at unlock threshold, got %s", got)
	}
	if got := normalizeDepositLifecycleStatus("REORGED", "", 0, 6, 12); got != "REVERTED" {
		t.Fatalf("expected REVERTED for reorged input, got %s", got)
	}
	if got := normalizeDepositLifecycleStatus("", "CONFIRMED", 12, 6, 12); got != "FINALIZED" {
		t.Fatalf("expected fallback CONFIRMED + confirmations to resolve FINALIZED, got %s", got)
	}
}

func TestResolveEffectiveDepositStatus(t *testing.T) {
	if got := resolveEffectiveDepositStatus("CONFIRMED", "PENDING"); got != "CONFIRMED" {
		t.Fatalf("expected status to stay CONFIRMED, got %s", got)
	}
	if got := resolveEffectiveDepositStatus("CONFIRMED", "FINALIZED"); got != "FINALIZED" {
		t.Fatalf("expected status to advance to FINALIZED, got %s", got)
	}
	if got := resolveEffectiveDepositStatus("FINALIZED", "CONFIRMED"); got != "FINALIZED" {
		t.Fatalf("expected FINALIZED to be sticky, got %s", got)
	}
	if got := resolveEffectiveDepositStatus("FINALIZED", "REVERTED"); got != "REVERTED" {
		t.Fatalf("expected REVERTED to override FINALIZED, got %s", got)
	}
}
