package events

import (
	"testing"
	"time"
)

func TestWithRetry_ValidPolicy(t *testing.T) {
	t.Parallel()
	opts := defaultSubscribeOptions()
	WithRetry(RetryPolicy{
		MaxRetries:   3,
		InitialDelay: time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	})(opts)
	if opts.retry == nil {
		t.Fatal("expected retry policy")
	}
	if opts.retry.MaxRetries != 3 {
		t.Errorf("expected 3, got %d", opts.retry.MaxRetries)
	}
	if opts.retry.Multiplier != 2.0 {
		t.Errorf("expected 2.0, got %f", opts.retry.Multiplier)
	}
}

func TestWithRetry_ZeroMultiplier(t *testing.T) {
	t.Parallel()
	opts := defaultSubscribeOptions()
	WithRetry(RetryPolicy{
		MaxRetries:   2,
		InitialDelay: time.Millisecond,
		Multiplier:   0,
	})(opts)
	if opts.retry.Multiplier != 2.0 {
		t.Errorf("expected default 2.0, got %f", opts.retry.Multiplier)
	}
}

func TestWithRetry_NegativeMultiplier(t *testing.T) {
	t.Parallel()
	opts := defaultSubscribeOptions()
	WithRetry(RetryPolicy{
		MaxRetries: 1,
		Multiplier: -1.0,
	})(opts)
	if opts.retry.Multiplier != 2.0 {
		t.Errorf("expected default 2.0, got %f", opts.retry.Multiplier)
	}
}

func TestWithRetry_NegativeMaxRetries(t *testing.T) {
	t.Parallel()
	opts := defaultSubscribeOptions()
	WithRetry(RetryPolicy{
		MaxRetries: -5,
		Multiplier: 3.0,
	})(opts)
	if opts.retry.MaxRetries != 0 {
		t.Errorf("expected 0, got %d", opts.retry.MaxRetries)
	}
}

func TestDefaultSubscribeOptions(t *testing.T) {
	t.Parallel()
	opts := defaultSubscribeOptions()
	if opts.retry != nil {
		t.Error("expected nil retry by default")
	}
}
