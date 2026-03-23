package dispatcher

import (
	"testing"
	"time"
)

func TestWithdrawDispatcherRetryDelay(t *testing.T) {
	d := &WithdrawDispatcher{
		MaxAttempts: 5,
		BaseBackoff: time.Second,
		MaxBackoff:  10 * time.Second,
	}
	if !d.shouldRetry(1) {
		t.Fatalf("expected attempt 1 to be retryable")
	}
	if d.shouldRetry(5) {
		t.Fatalf("expected attempt 5 to stop retrying")
	}
	if got := d.retryDelay(1); got != time.Second {
		t.Fatalf("expected first retry delay 1s, got %s", got)
	}
	if got := d.retryDelay(4); got != 8*time.Second {
		t.Fatalf("expected fourth retry delay 8s, got %s", got)
	}
	if got := d.retryDelay(10); got != 10*time.Second {
		t.Fatalf("expected capped retry delay 10s, got %s", got)
	}
}

func TestWithdrawDispatcherParallelism(t *testing.T) {
	d := &WithdrawDispatcher{Parallelism: 4}
	if got := d.parallelism(0); got != 4 {
		t.Fatalf("expected default configured parallelism 4 for empty batch, got %d", got)
	}
	if got := d.parallelism(2); got != 2 {
		t.Fatalf("expected parallelism to cap at job count, got %d", got)
	}
	d = &WithdrawDispatcher{}
	if got := d.parallelism(6); got != 4 {
		t.Fatalf("expected fallback parallelism 4, got %d", got)
	}
}
