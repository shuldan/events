package events

import (
	"context"
	"testing"
	"time"
)

func TestSubscribe_BasicDelivery(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h := newCollectHandler[*plainTestEvent]()
	var handler Handler[*plainTestEvent] = h
	Subscribe(d, handler)

	event := &plainTestEvent{Value: "hello"}
	err := d.Publish(context.Background(), event)

	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(h.Events()) != 1 {
		t.Fatalf("expected 1 event, got %d", len(h.Events()))
	}
	if h.Events()[0].Value != "hello" {
		t.Errorf("expected %q, got %q", "hello", h.Events()[0].Value)
	}
}

func TestSubscribe_NilHandler_Panics(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil handler")
		}
		if r != ErrNilHandler {
			t.Errorf("expected ErrNilHandler, got %v", r)
		}
	}()

	Subscribe[*plainTestEvent](d, nil)
}

func TestSubscribe_MultipleHandlersSameEvent(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h1 := newCountHandler[*plainTestEvent]()
	h2 := newCountHandler[*plainTestEvent]()
	Subscribe(d, h1)
	Subscribe(d, h2)

	d.Publish(context.Background(), &plainTestEvent{})

	if h1.count.Load() != 1 {
		t.Errorf("h1: expected 1, got %d", h1.count.Load())
	}
	if h2.count.Load() != 1 {
		t.Errorf("h2: expected 1, got %d", h2.count.Load())
	}
}

func TestSubscribe_DifferentEventTypes(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h1 := newCountHandler[*plainTestEvent]()
	h2 := newCountHandler[*anotherTestEvent]()
	Subscribe(d, h1)
	Subscribe(d, h2)

	d.Publish(context.Background(), &plainTestEvent{})

	if h1.count.Load() != 1 {
		t.Errorf("h1: expected 1, got %d", h1.count.Load())
	}
	if h2.count.Load() != 0 {
		t.Errorf("h2: expected 0, got %d", h2.count.Load())
	}
}

func TestSubscribe_WithRetry(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h := newCountHandler[*plainTestEvent]()
	failTwice := &failNHandler[*plainTestEvent]{
		inner:    h,
		failLeft: 2,
	}

	Subscribe(d, failTwice, WithRetry(RetryPolicy{
		MaxRetries:   3,
		InitialDelay: time.Millisecond,
		Multiplier:   1.0,
	}))

	err := d.Publish(context.Background(), &plainTestEvent{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if h.count.Load() != 1 {
		t.Errorf("expected 1 success, got %d", h.count.Load())
	}
}

func TestSubscribe_WithTimeout_Expired(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h := newSlowHandler[*plainTestEvent](5 * time.Second)
	Subscribe(d, h, WithTimeout(50*time.Millisecond))

	err := d.Publish(context.Background(), &plainTestEvent{})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestSubscribe_WithTimeout_Success(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h := newSlowHandler[*plainTestEvent](10 * time.Millisecond)
	Subscribe(d, h, WithTimeout(5*time.Second))

	err := d.Publish(context.Background(), &plainTestEvent{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestSubscribe_GlobalAndLocalMiddleware(t *testing.T) {
	t.Parallel()

	var order []string
	globalMw := &trackingMiddleware{order: &order, label: "global"}
	localMw := &trackingMiddleware{order: &order, label: "local"}

	d := New(WithMiddleware(globalMw))
	defer d.Close(context.Background())

	h := newCountHandler[*plainTestEvent]()
	Subscribe(d, h, WithSubscribeMiddleware(localMw))

	d.Publish(context.Background(), &plainTestEvent{})

	expected := []string{"global:before", "local:before", "local:after", "global:after"}
	if !sliceEqual(order, expected) {
		t.Errorf("expected %v, got %v", expected, order)
	}
}

func TestSubscribe_DefaultSubOpts_Merged(t *testing.T) {
	t.Parallel()

	d := New(
		WithDefaultSubscribeOptions(WithTimeout(50 * time.Millisecond)),
	)
	defer d.Close(context.Background())

	h := newSlowHandler[*plainTestEvent](5 * time.Second)
	Subscribe(d, h)

	err := d.Publish(context.Background(), &plainTestEvent{})
	if err == nil {
		t.Fatal("expected timeout from default opts")
	}
}

func TestSubscribe_LocalOptsOverrideGlobal(t *testing.T) {
	t.Parallel()

	d := New(
		WithDefaultSubscribeOptions(WithTimeout(50 * time.Millisecond)),
	)
	defer d.Close(context.Background())

	h := newSlowHandler[*plainTestEvent](100 * time.Millisecond)
	Subscribe(d, h, WithTimeout(5*time.Second))

	err := d.Publish(context.Background(), &plainTestEvent{})
	if err != nil {
		t.Fatalf("expected nil (local override), got %v", err)
	}
}

type failNHandler[E Event] struct {
	inner    Handler[E]
	failLeft int
}

func (h *failNHandler[E]) Handle(ctx context.Context, event E) error {
	if h.failLeft > 0 {
		h.failLeft--
		return errTest
	}
	return h.inner.Handle(ctx, event)
}
