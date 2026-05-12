package resilience

import (
	"context"
	"math/rand"
	"time"
)

func Retry(ctx context.Context, maxAttempts int, baseDelay, maxDelay time.Duration, fn func(attempt int) error) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = fn(attempt)
		if lastErr == nil {
			return nil
		}

		if attempt == maxAttempts-1 {
			break
		}

		delay := backoff(attempt, baseDelay, maxDelay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

func backoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := baseDelay * (1 << uint(attempt))
	if delay > maxDelay {
		delay = maxDelay
	}

	jitter := float64(delay) * 0.25
	delay = time.Duration(float64(delay) - jitter + rand.Float64()*2*jitter)

	return delay
}
