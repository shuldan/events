package events

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{MaxRetries: 3, InitialDelay: time.Millisecond}
	var count atomic.Int32

	err := retry(context.Background(), policy, func(_ context.Context) error {
		count.Add(1)
		return nil
	})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if count.Load() != 1 {
		t.Errorf("expected 1 call, got %d", count.Load())
	}
}

func TestRetry_SuccessOnThirdAttempt(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{MaxRetries: 5, InitialDelay: time.Millisecond, Multiplier: 1.0}
	var count atomic.Int32

	err := retry(context.Background(), policy, func(_ context.Context) error {
		if count.Add(1) < 3 {
			return errTest
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if count.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", count.Load())
	}
}

func TestRetry_ExhaustedRetries(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{MaxRetries: 2, InitialDelay: time.Millisecond, Multiplier: 1.0}
	var count atomic.Int32

	err := retry(context.Background(), policy, func(_ context.Context) error {
		count.Add(1)
		return errTest
	})

	if !errors.Is(err, errTest) {
		t.Errorf("expected %v, got %v", errTest, err)
	}
	if count.Load() != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", count.Load())
	}
}

func TestRetry_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	policy := RetryPolicy{MaxRetries: 100, InitialDelay: time.Second, Multiplier: 1.0}
	var count atomic.Int32

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := retry(ctx, policy, func(_ context.Context) error {
		count.Add(1)
		return errTest
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRetry_ZeroRetries(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{MaxRetries: 0}
	var count atomic.Int32

	err := retry(context.Background(), policy, func(_ context.Context) error {
		count.Add(1)
		return errTest
	})

	if !errors.Is(err, errTest) {
		t.Errorf("expected %v, got %v", errTest, err)
	}
	if count.Load() != 1 {
		t.Errorf("expected 1 call, got %d", count.Load())
	}
}

func TestRetryPolicy_Delay_ExponentialBackoff(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     time.Second,
		Multiplier:   2.0,
	}

	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, time.Second},
		{5, time.Second},
	}

	for _, tc := range cases {
		got := policy.delay(tc.attempt)
		if got != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, got)
		}
	}
}

func TestRetryPolicy_Delay_ZeroInitial(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{InitialDelay: 0, Multiplier: 2.0}
	got := policy.delay(5)
	if got != 0 {
		t.Errorf("expected 0, got %v", got)
	}
}

func TestRetryPolicy_Delay_ZeroMaxDelay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     0,
		Multiplier:   2.0,
	}

	got := policy.delay(3)
	expected := 800 * time.Millisecond
	if got != expected {
		t.Errorf("expected %v, got %v", expected, got)
	}
}
