package events

import (
	"context"
	"testing"
)

func TestBuildChain_NoMiddleware(t *testing.T) {
	t.Parallel()
	var called bool
	handler := func(_ context.Context, _ Event) error {
		called = true
		return nil
	}
	chain := buildChain(handler, nil)
	_ = chain(context.Background(), newTestEvent("e", "1"))
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestBuildChain_SingleMiddleware(t *testing.T) {
	t.Parallel()
	var order []string
	mw := func(next HandleFunc) HandleFunc {
		return func(ctx context.Context, event Event) error {
			order = append(order, "mw")
			return next(ctx, event)
		}
	}
	handler := func(_ context.Context, _ Event) error {
		order = append(order, "handler")
		return nil
	}
	chain := buildChain(handler, []Middleware{mw})
	_ = chain(context.Background(), newTestEvent("e", "1"))
	if len(order) != 2 || order[0] != "mw" || order[1] != "handler" {
		t.Errorf("expected [mw handler], got %v", order)
	}
}

func TestBuildChain_MultipleMiddlewares(t *testing.T) {
	t.Parallel()
	var order []string
	mw1 := func(next HandleFunc) HandleFunc {
		return func(ctx context.Context, event Event) error {
			order = append(order, "mw1")
			return next(ctx, event)
		}
	}
	mw2 := func(next HandleFunc) HandleFunc {
		return func(ctx context.Context, event Event) error {
			order = append(order, "mw2")
			return next(ctx, event)
		}
	}
	handler := func(_ context.Context, _ Event) error {
		order = append(order, "handler")
		return nil
	}
	chain := buildChain(handler, []Middleware{mw1, mw2})
	_ = chain(context.Background(), newTestEvent("e", "1"))
	expected := []string{"mw1", "mw2", "handler"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], order[i])
		}
	}
}
