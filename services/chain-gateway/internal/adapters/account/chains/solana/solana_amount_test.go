package solana

import "testing"

func TestNormalizeTokenAmountRaw(t *testing.T) {
	got, err := normalizeTokenAmount("1500000", "raw", 6)
	if err != nil {
		t.Fatalf("normalizeTokenAmount returned error: %v", err)
	}
	if got != 1500000 {
		t.Fatalf("unexpected raw amount: %d", got)
	}
}

func TestNormalizeTokenAmountDisplay(t *testing.T) {
	got, err := normalizeTokenAmount("1.5", "display", 6)
	if err != nil {
		t.Fatalf("normalizeTokenAmount returned error: %v", err)
	}
	if got != 1500000 {
		t.Fatalf("unexpected display amount: %d", got)
	}
}

func TestNormalizeTokenAmountDefaultsDecimalStringsToDisplay(t *testing.T) {
	got, err := normalizeTokenAmount("0.000001", "", 6)
	if err != nil {
		t.Fatalf("normalizeTokenAmount returned error: %v", err)
	}
	if got != 1 {
		t.Fatalf("unexpected default display amount: %d", got)
	}
}

func TestNormalizeTokenAmountRejectsPrecisionOverflow(t *testing.T) {
	if _, err := normalizeTokenAmount("1.0000001", "display", 6); err == nil {
		t.Fatalf("expected precision overflow error")
	}
}
