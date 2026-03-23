package dispatcher

import (
	"encoding/base64"
	"encoding/json"
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

func TestBumpEVMReplacementPayload(t *testing.T) {
	raw, err := json.Marshal(evmDynamicFeePayload{
		ChainID:              "11155111",
		NonceMode:            "fixed",
		FromAddress:          "0x1111111111111111111111111111111111111111",
		Nonce:                7,
		MaxPriorityFeePerGas: "100",
		MaxFeePerGas:         "1000",
		GasLimit:             21000,
		ToAddress:            "0x2222222222222222222222222222222222222222",
		Amount:               "123",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	encoded, err := bumpEVMReplacementPayload(base64.StdEncoding.EncodeToString(raw), 2000)
	if err != nil {
		t.Fatalf("bumpEVMReplacementPayload returned error: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode bumped payload: %v", err)
	}
	var payload evmDynamicFeePayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("unmarshal bumped payload: %v", err)
	}
	if payload.Nonce != 7 || payload.NonceMode != "fixed" {
		t.Fatalf("expected fixed nonce payload to keep nonce, got %+v", payload)
	}
	if payload.MaxPriorityFeePerGas != "120" || payload.MaxFeePerGas != "1200" {
		t.Fatalf("unexpected bumped fees: %+v", payload)
	}
}
