package middleware

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shuldan/events"
)

type fakeLogger struct {
	mu    sync.Mutex
	infos []string
	errs  []string
}

func (l *fakeLogger) Info(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
}

func (l *fakeLogger) Error(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, msg)
}

type testEvent struct{ Value string }

type successNext struct{ called bool }

func (n *successNext) Handle(_ context.Context, _ events.Event) error {
	n.called = true
	return nil
}

type errorNext struct{ err error }

func (n *errorNext) Handle(_ context.Context, _ events.Event) error {
	return n.err
}

func TestLoggingMiddleware_Success(t *testing.T) {
	t.Parallel()

	logger := &fakeLogger{}
	mw := NewLogging(logger)

	next := &successNext{}
	wrapped := mw.Wrap(next)

	err := wrapped.Handle(context.Background(), &testEvent{Value: "ok"})

	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if !next.called {
		t.Error("expected next to be called")
	}
	if len(logger.infos) != 2 {
		t.Errorf("expected 2 info logs, got %d", len(logger.infos))
	}
	if len(logger.errs) != 0 {
		t.Errorf("expected 0 error logs, got %d", len(logger.errs))
	}
}

func TestLoggingMiddleware_Error(t *testing.T) {
	t.Parallel()

	logger := &fakeLogger{}
	mw := NewLogging(logger)

	testErr := errors.New("fail")
	next := &errorNext{err: testErr}
	wrapped := mw.Wrap(next)

	err := wrapped.Handle(context.Background(), &testEvent{})

	if !errors.Is(err, testErr) {
		t.Errorf("expected %v, got %v", testErr, err)
	}
	if len(logger.infos) != 1 {
		t.Errorf("expected 1 info log, got %d", len(logger.infos))
	}
	if len(logger.errs) != 1 {
		t.Errorf("expected 1 error log, got %d", len(logger.errs))
	}
}

func TestNewSlogAdapter(t *testing.T) {
	t.Parallel()

	adapter := NewSlogAdapter()
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
}
