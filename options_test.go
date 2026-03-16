package events

import (
	"context"
	"testing"
)

func TestWithAsyncMode(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	WithAsyncMode()(&cfg)
	if !cfg.async {
		t.Error("expected async to be true")
	}
}

func TestWithWorkerPool_Positive(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	WithWorkerPool(5)(&cfg)
	if cfg.workerPoolSize != 5 {
		t.Errorf("expected 5, got %d", cfg.workerPoolSize)
	}
}

func TestWithWorkerPool_Zero(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	original := cfg.workerPoolSize
	WithWorkerPool(0)(&cfg)
	if cfg.workerPoolSize != original {
		t.Errorf("expected %d, got %d", original, cfg.workerPoolSize)
	}
}

func TestWithWorkerPool_Negative(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	original := cfg.workerPoolSize
	WithWorkerPool(-1)(&cfg)
	if cfg.workerPoolSize != original {
		t.Errorf("expected %d, got %d", original, cfg.workerPoolSize)
	}
}

func TestWithErrorHandler(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	called := false
	fn := func(_ context.Context, _ Event, _ error) { called = true }
	WithErrorHandler(fn)(&cfg)

	if cfg.errorHandler == nil {
		t.Fatal("expected errorHandler to be set")
	}
	cfg.errorHandler(context.Background(), nil, nil)
	if !called {
		t.Error("expected errorHandler to be called")
	}
}

func TestWithMiddleware(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	mw := &testMiddleware{}
	WithMiddleware(mw)(&cfg)

	if len(cfg.middleware) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(cfg.middleware))
	}
}

func TestWithMiddleware_Appends(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	WithMiddleware(&testMiddleware{})(&cfg)
	WithMiddleware(&testMiddleware{})(&cfg)

	if len(cfg.middleware) != 2 {
		t.Errorf("expected 2 middleware, got %d", len(cfg.middleware))
	}
}

func TestWithCodec(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	c := &testCodec{}
	WithCodec(c)(&cfg)

	if cfg.codec == nil {
		t.Error("expected codec to be set")
	}
}

func TestWithTransport(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	tr := &testTransport{}
	WithTransport(tr)(&cfg)

	if cfg.transport == nil {
		t.Error("expected transport to be set")
	}
}

func TestWithDefaultSubscribeOptions(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	WithDefaultSubscribeOptions(WithTimeout(5))(&cfg)

	if len(cfg.defaultSubOpts) != 1 {
		t.Errorf("expected 1 default sub opt, got %d", len(cfg.defaultSubOpts))
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	if cfg.async {
		t.Error("expected async to be false by default")
	}
	if cfg.workerPoolSize != 1 {
		t.Errorf("expected workerPoolSize 1, got %d", cfg.workerPoolSize)
	}
	if cfg.errorHandler != nil {
		t.Error("expected errorHandler to be nil")
	}
	if cfg.transport != nil {
		t.Error("expected transport to be nil")
	}
	if cfg.codec != nil {
		t.Error("expected codec to be nil")
	}
}
