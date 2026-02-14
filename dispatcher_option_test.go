package events

import (
	"testing"
	"time"
)

func TestWithAsyncMode(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	o.asyncMode = false
	WithAsyncMode()(o)
	if !o.asyncMode {
		t.Error("expected asyncMode true")
	}
}

func TestWithSyncMode(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	WithSyncMode()(o)
	if o.asyncMode {
		t.Error("expected asyncMode false")
	}
}

func TestWithWorkerCount_Valid(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	WithWorkerCount(8)(o)
	if o.workerCount != 8 {
		t.Errorf("expected 8, got %d", o.workerCount)
	}
}

func TestWithWorkerCount_BelowMinimum(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero", 0, 1},
		{"negative", -5, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := defaultOptions()
			WithWorkerCount(tc.input)(o)
			if o.workerCount != tc.want {
				t.Errorf("expected %d, got %d", tc.want, o.workerCount)
			}
		})
	}
}

func TestWithBufferSize_Valid(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	WithBufferSize(100)(o)
	if o.bufferSize != 100 {
		t.Errorf("expected 100, got %d", o.bufferSize)
	}
}

func TestWithBufferSize_Negative(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	WithBufferSize(-10)(o)
	if o.bufferSize != 0 {
		t.Errorf("expected 0, got %d", o.bufferSize)
	}
}

func TestWithPublishTimeout(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	WithPublishTimeout(10 * time.Second)(o)
	if o.publishTimeout != 10*time.Second {
		t.Errorf("expected 10s, got %v", o.publishTimeout)
	}
}

func TestWithOrderedDelivery(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	WithOrderedDelivery()(o)
	if !o.ordered {
		t.Error("expected ordered true")
	}
}

func TestWithMiddleware(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	mw := func(next HandleFunc) HandleFunc { return next }
	WithMiddleware(mw)(o)
	if len(o.middlewares) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(o.middlewares))
	}
}

func TestWithPanicHandler(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	ph := &spyPanicHandler{}
	WithPanicHandler(ph)(o)
	if o.panicHandler != ph {
		t.Error("expected custom panic handler")
	}
}

func TestWithErrorHandler(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	eh := &spyErrorHandler{}
	WithErrorHandler(eh)(o)
	if o.errorHandler != eh {
		t.Error("expected custom error handler")
	}
}

func TestWithMetrics(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	m := &spyMetrics{}
	WithMetrics(m)(o)
	if o.metrics != m {
		t.Error("expected custom metrics")
	}
}

func TestDefaultOptions(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	if !o.asyncMode {
		t.Error("expected async mode by default")
	}
	if o.publishTimeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", o.publishTimeout)
	}
	if o.ordered {
		t.Error("expected ordered false by default")
	}
	if o.panicHandler == nil {
		t.Error("expected non-nil panic handler")
	}
	if o.errorHandler == nil {
		t.Error("expected non-nil error handler")
	}
	if o.metrics == nil {
		t.Error("expected non-nil metrics")
	}
}
