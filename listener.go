package events

import (
	"context"
	"reflect"
	"time"
)

type listener struct {
	id        string
	eventType reflect.Type
	handler   HandleFunc
	retry     *RetryPolicy
}

func (l *listener) execute(ctx context.Context, event Event) error {
	if l.retry == nil || l.retry.MaxRetries == 0 {
		return l.handler(ctx, event)
	}
	return l.executeWithRetry(ctx, event)
}

func (l *listener) executeWithRetry(ctx context.Context, event Event) error {
	var lastErr error

	delay := l.retry.InitialDelay

	for attempt := 0; attempt <= l.retry.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = l.handler(ctx, event)
		if lastErr == nil {
			return nil
		}

		if attempt == l.retry.MaxRetries {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay = time.Duration(float64(delay) * l.retry.Multiplier)
		if delay > l.retry.MaxDelay {
			delay = l.retry.MaxDelay
		}
	}

	return lastErr
}
