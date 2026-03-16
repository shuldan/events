package events

import (
	"testing"
	"time"
)

func TestWithRetry(t *testing.T) {
	t.Parallel()

	cfg := defaultSubscribeConfig()
	policy := RetryPolicy{MaxRetries: 3, InitialDelay: time.Second}
	WithRetry(policy)(&cfg)

	if cfg.retry == nil {
		t.Fatal("expected retry to be set")
	}
	if cfg.retry.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.retry.MaxRetries)
	}
	if cfg.retry.InitialDelay != time.Second {
		t.Errorf("expected InitialDelay %v, got %v", time.Second, cfg.retry.InitialDelay)
	}
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	cfg := defaultSubscribeConfig()
	WithTimeout(5 * time.Second)(&cfg)

	if cfg.timeout != 5*time.Second {
		t.Errorf("expected %v, got %v", 5*time.Second, cfg.timeout)
	}
}

func TestWithSubscribeMiddleware(t *testing.T) {
	t.Parallel()

	cfg := defaultSubscribeConfig()
	mw := &testMiddleware{}
	WithSubscribeMiddleware(mw)(&cfg)

	if len(cfg.middleware) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(cfg.middleware))
	}
}

func TestWithSubscribeMiddleware_Appends(t *testing.T) {
	t.Parallel()

	cfg := defaultSubscribeConfig()
	WithSubscribeMiddleware(&testMiddleware{})(&cfg)
	WithSubscribeMiddleware(&testMiddleware{})(&cfg)

	if len(cfg.middleware) != 2 {
		t.Errorf("expected 2 middleware, got %d", len(cfg.middleware))
	}
}

func TestDefaultSubscribeConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultSubscribeConfig()
	if cfg.retry != nil {
		t.Error("expected retry to be nil by default")
	}
	if cfg.timeout != 0 {
		t.Error("expected timeout to be 0 by default")
	}
	if len(cfg.middleware) != 0 {
		t.Error("expected no middleware by default")
	}
}
