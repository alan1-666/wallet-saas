package httptransport

import "time"

func retryLedgerMutation(attempts int, delay time.Duration, fn func() error) error {
	if attempts <= 0 {
		attempts = 1
	}
	if delay <= 0 {
		delay = 200 * time.Millisecond
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i == attempts-1 {
				break
			}
			time.Sleep(delay)
			continue
		}
		return nil
	}
	return lastErr
}
