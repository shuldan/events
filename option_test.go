package events

import (
	"testing"
)

func TestWithPanicHandler(t *testing.T) {
	t.Parallel()
	ph := &testPanicHandler{}
	opts := &options{}

	opt := WithPanicHandler(ph)
	opt(opts)

	if opts.panicHandler != ph {
		t.Error("expected panic handler to be set")
	}
}

func TestWithErrorHandler(t *testing.T) {
	t.Parallel()
	eh := &testErrorHandler{}
	opts := &options{}

	opt := WithErrorHandler(eh)
	opt(opts)

	if opts.errorHandler != eh {
		t.Error("expected error handler to be set")
	}
}

func TestWithAsyncMode(t *testing.T) {
	t.Parallel()
	opts := &options{}

	opt := WithAsyncMode()
	opt(opts)

	if !opts.asyncMode {
		t.Error("expected async mode to be true")
	}
}

func TestWithSyncMode(t *testing.T) {
	t.Parallel()
	opts := &options{asyncMode: true}

	opt := WithSyncMode()
	opt(opts)

	if opts.asyncMode {
		t.Error("expected async mode to be false")
	}
}

func TestWithWorkerCount_ValidCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		count    int
		expected int
	}{
		{"positive", 5, 5},
		{"max", 100, 100},
		{"one", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &options{}
			opt := WithWorkerCount(tt.count)
			opt(opts)

			if opts.workerCount != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, opts.workerCount)
			}
		})
	}
}

func TestWithWorkerCount_InvalidCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		count int
	}{
		{"zero", 0},
		{"negative", -1},
		{"large_negative", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &options{}
			opt := WithWorkerCount(tt.count)
			opt(opts)

			if opts.workerCount != 1 {
				t.Errorf("expected 1 for invalid count, got %d", opts.workerCount)
			}
		})
	}
}

func TestOptions_MultipleOptions(t *testing.T) {
	t.Parallel()
	ph := &testPanicHandler{}
	eh := &testErrorHandler{}

	d := New(
		WithPanicHandler(ph),
		WithErrorHandler(eh),
		WithAsyncMode(),
		WithWorkerCount(3),
	)
	defer d.Close()

	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
}

func TestOptions_NilOptions(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
}

func TestOptions_DefaultAsyncMode(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	if !d.opts.asyncMode {
		t.Error("expected default async mode to be true")
	}
}

func TestOptions_DefaultWorkerCount(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	if d.opts.workerCount <= 0 {
		t.Errorf("expected positive worker count, got %d", d.opts.workerCount)
	}
}
