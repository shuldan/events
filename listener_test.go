package events

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestListener_Execute_NoRetry(t *testing.T) {
	t.Parallel()
	var called int
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			called++
			return nil
		},
	}
	err := l.execute(context.Background(), newTestEvent("e", "1"))
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestListener_Execute_RetryZeroMax(t *testing.T) {
	t.Parallel()
	var called int
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			called++
			return errors.New("fail")
		},
		retry: &RetryPolicy{MaxRetries: 0},
	}
	err := l.execute(context.Background(), newTestEvent("e", "1"))
	if err == nil {
		t.Error("expected error")
	}
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestListener_ExecuteWithRetry_SuccessFirstAttempt(t *testing.T) {
	t.Parallel()
	var called int
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			called++
			return nil
		},
		retry: &RetryPolicy{
			MaxRetries:   3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		},
	}
	err := l.execute(context.Background(), newTestEvent("e", "1"))
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestListener_ExecuteWithRetry_SuccessOnRetry(t *testing.T) {
	t.Parallel()
	var called int
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			called++
			if called < 3 {
				return errors.New("fail")
			}
			return nil
		},
		retry: &RetryPolicy{
			MaxRetries:   5,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		},
	}
	err := l.execute(context.Background(), newTestEvent("e", "1"))
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
}

func TestListener_ExecuteWithRetry_AllRetriesExhausted(t *testing.T) {
	t.Parallel()
	var called int
	expectedErr := errors.New("always fail")
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			called++
			return expectedErr
		},
		retry: &RetryPolicy{
			MaxRetries:   2,
			InitialDelay: time.Millisecond,
			MaxDelay:     5 * time.Millisecond,
			Multiplier:   2.0,
		},
	}
	err := l.execute(context.Background(), newTestEvent("e", "1"))
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
	if called != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", called)
	}
}

func TestListener_ExecuteWithRetry_ContextCancelledBeforeHandler(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			return nil
		},
		retry: &RetryPolicy{
			MaxRetries:   3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		},
	}
	err := l.execute(ctx, newTestEvent("e", "1"))
	if err == nil {
		t.Error("expected context error")
	}
}

func TestListener_ExecuteWithRetry_ContextCancelledDuringDelay(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var called int
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			called++
			return errors.New("fail")
		},
		retry: &RetryPolicy{
			MaxRetries:   10,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     time.Second,
			Multiplier:   2.0,
		},
	}
	err := l.execute(ctx, newTestEvent("e", "1"))
	if err == nil {
		t.Error("expected error")
	}
	if called < 1 {
		t.Errorf("expected at least 1 call, got %d", called)
	}
}

func TestListener_ExecuteWithRetry_DelayCapping(t *testing.T) {
	t.Parallel()
	var called int
	l := &listener{
		handler: func(_ context.Context, _ Event) error {
			called++
			if called <= 4 {
				return errors.New("fail")
			}
			return nil
		},
		retry: &RetryPolicy{
			MaxRetries:   5,
			InitialDelay: time.Millisecond,
			MaxDelay:     2 * time.Millisecond,
			Multiplier:   10.0,
		},
	}
	start := time.Now()
	err := l.execute(context.Background(), newTestEvent("e", "1"))
	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("delay capping not working, took %v", elapsed)
	}
}
