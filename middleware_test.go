package events

import (
	"context"
	"sync"
	"testing"
)

type trackingMiddleware struct {
	mu    sync.Mutex
	order *[]string
	label string
}

type trackingNext struct {
	mw   *trackingMiddleware
	next Next
}

func (m *trackingMiddleware) Wrap(next Next) Next {
	return &trackingNext{mw: m, next: next}
}

func (n *trackingNext) Handle(ctx context.Context, event Event) error {
	n.mw.mu.Lock()
	*n.mw.order = append(*n.mw.order, n.mw.label+":before")
	n.mw.mu.Unlock()

	err := n.next.Handle(ctx, event)

	n.mw.mu.Lock()
	*n.mw.order = append(*n.mw.order, n.mw.label+":after")
	n.mw.mu.Unlock()

	return err
}

type finalHandler struct {
	mu    sync.Mutex
	order *[]string
}

func (h *finalHandler) Handle(_ context.Context, _ Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.order = append(*h.order, "handler")
	return nil
}

func TestBuildChain_Empty(t *testing.T) {
	t.Parallel()

	var order []string
	final := &finalHandler{order: &order}

	chain := buildChain(nil, final)
	err := chain.Handle(context.Background(), &plainTestEvent{})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if len(order) != 1 || order[0] != "handler" {
		t.Errorf("expected [handler], got %v", order)
	}
}

func TestBuildChain_SingleMiddleware(t *testing.T) {
	t.Parallel()

	var order []string
	final := &finalHandler{order: &order}
	mw := &trackingMiddleware{order: &order, label: "mw1"}

	chain := buildChain([]Middleware{mw}, final)
	err := chain.Handle(context.Background(), &plainTestEvent{})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	expected := []string{"mw1:before", "handler", "mw1:after"}
	if !sliceEqual(order, expected) {
		t.Errorf("expected %v, got %v", expected, order)
	}
}

func TestBuildChain_MultipleMiddleware_Order(t *testing.T) {
	t.Parallel()

	var order []string
	final := &finalHandler{order: &order}
	mw1 := &trackingMiddleware{order: &order, label: "mw1"}
	mw2 := &trackingMiddleware{order: &order, label: "mw2"}

	chain := buildChain([]Middleware{mw1, mw2}, final)
	err := chain.Handle(context.Background(), &plainTestEvent{})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	expected := []string{"mw1:before", "mw2:before", "handler", "mw2:after", "mw1:after"}
	if !sliceEqual(order, expected) {
		t.Errorf("expected %v, got %v", expected, order)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
