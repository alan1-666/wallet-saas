package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	retryAttempts = 3
	retryBackoff  = 120 * time.Millisecond
)

func validateChainNetwork(chain, network string) error {
	if strings.TrimSpace(chain) == "" || strings.TrimSpace(network) == "" {
		return fmt.Errorf("chain and network are required")
	}
	return nil
}

func withRetry[T any](ctx context.Context, op string, fn func() (T, error)) (T, error) {
	var zero T
	backoff := retryBackoff
	for i := 0; i < retryAttempts; i++ {
		out, err := fn()
		if err == nil {
			return out, nil
		}
		if i == retryAttempts-1 || !isRetryable(err) {
			return zero, fmt.Errorf("%s: %w", op, err)
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return zero, fmt.Errorf("%s: retry exhausted", op)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded {
			return true
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	msg := strings.ToLower(err.Error())
	retryHints := []string{
		"timeout",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"broken pipe",
		"transport is closing",
		"eof",
	}
	for _, hint := range retryHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}
	return false
}
