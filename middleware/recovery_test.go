package middleware

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shuldan/events"
)

type panicNext struct{ msg string }

func (n *panicNext) Handle(_ context.Context, _ events.Event) error {
	panic(n.msg)
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	t.Parallel()

	mw := NewRecovery()
	next := &successNext{}
	wrapped := mw.Wrap(next)

	err := wrapped.Handle(context.Background(), &testEvent{})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if !next.called {
		t.Error("expected next to be called")
	}
}

func TestRecoveryMiddleware_WithPanic(t *testing.T) {
	t.Parallel()

	mw := NewRecovery()
	next := &panicNext{msg: "boom"}
	wrapped := mw.Wrap(next)

	err := wrapped.Handle(context.Background(), &testEvent{})

	if err == nil {
		t.Fatal("expected error from recovered panic")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error containing 'boom', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "panic recovered") {
		t.Errorf("expected 'panic recovered' in error, got %q", err.Error())
	}
}

func TestRecoveryMiddleware_PropagatesError(t *testing.T) {
	t.Parallel()

	mw := NewRecovery()
	testErr := errors.New("regular error")
	next := &errorNext{err: testErr}
	wrapped := mw.Wrap(next)

	err := wrapped.Handle(context.Background(), &testEvent{})

	if !errors.Is(err, testErr) {
		t.Errorf("expected %v, got %v", testErr, err)
	}
}
