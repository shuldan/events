package events

import (
	"context"
	"errors"
	"testing"
)

func TestHandlerFunc_Handle_Success(t *testing.T) {
	t.Parallel()
	var called bool
	fn := HandlerFunc[*testEvent](func(ctx context.Context, e *testEvent) error {
		called = true
		return nil
	})
	err := fn.Handle(context.Background(), newTestEvent("test", "1"))
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestHandlerFunc_Handle_Error(t *testing.T) {
	t.Parallel()
	expected := errors.New("fail")
	fn := HandlerFunc[*testEvent](func(_ context.Context, _ *testEvent) error {
		return expected
	})
	err := fn.Handle(context.Background(), newTestEvent("test", "1"))
	if !errors.Is(err, expected) {
		t.Errorf("expected %v, got %v", expected, err)
	}
}

func TestHandlerFunc_ImplementsHandler(t *testing.T) {
	t.Parallel()
	var _ Handler[*testEvent] = HandlerFunc[*testEvent](nil)
}
