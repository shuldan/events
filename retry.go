package events

import (
	"context"
	"time"
)

// RetryPolicy определяет политику повторных попыток.
type RetryPolicy struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// retry выполняет fn с повторными попытками согласно политике.
func retry(ctx context.Context, policy RetryPolicy, fn func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Последняя попытка — не ждём.
		if attempt == policy.MaxRetries {
			break
		}

		delay := policy.delay(attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// delay вычисляет задержку для попытки.
func (p RetryPolicy) delay(attempt int) time.Duration {
	if p.InitialDelay == 0 {
		return 0
	}

	delay := p.InitialDelay
	for range attempt {
		delay = time.Duration(float64(delay) * p.Multiplier)
		if p.MaxDelay > 0 && delay > p.MaxDelay {
			delay = p.MaxDelay
			break
		}
	}

	return delay
}
