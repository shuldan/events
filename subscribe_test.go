package events

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubscribeFunc_TypedEvent(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var received string
	SubscribeFunc[*testEvent](d, func(_ context.Context, e *testEvent) error {
		received = e.Data
		return nil
	})
	evt := newTestEvent("test", "1")
	evt.Data = "hello"
	_ = d.Publish(context.Background(), evt)
	if received != "hello" {
		t.Errorf("expected 'hello', got %q", received)
	}
}

func TestSubscribe_HandlerInterface(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var called atomic.Int32
	h := HandlerFunc[*testEvent](func(_ context.Context, _ *testEvent) error {
		called.Add(1)
		return nil
	})
	Subscribe[*testEvent](d, h)
	_ = d.Publish(context.Background(), newTestEvent("test", "1"))
	if called.Load() != 1 {
		t.Errorf("expected 1 call, got %d", called.Load())
	}
}

func TestSubscribeAll_ReceivesAllEvents(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var count atomic.Int32
	d.SubscribeAll(func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("e1", "1"))
	_ = d.Publish(context.Background(), newTestEvent2("e2", "2"))
	if count.Load() != 2 {
		t.Errorf("expected 2, got %d", count.Load())
	}
}

func TestUnsubscribe_Typed(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var count atomic.Int32
	sub := SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		count.Add(1)
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	sub.Unsubscribe()
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	if count.Load() != 1 {
		t.Errorf("expected 1 call after unsubscribe, got %d", count.Load())
	}
}

func TestUnsubscribe_Global(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var count atomic.Int32
	sub := d.SubscribeAll(func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	sub.Unsubscribe()
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	if count.Load() != 1 {
		t.Errorf("expected 1, got %d", count.Load())
	}
}

func TestSubscribe_MultipleListeners(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var c1, c2 atomic.Int32
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		c1.Add(1)
		return nil
	})
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		c2.Add(1)
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	if c1.Load() != 1 || c2.Load() != 1 {
		t.Errorf("expected both called once, got %d and %d", c1.Load(), c2.Load())
	}
}

func TestSubscribe_DifferentEventTypes(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var c1, c2 atomic.Int32
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		c1.Add(1)
		return nil
	})
	SubscribeFunc[*testEvent2](d, func(_ context.Context, _ *testEvent2) error {
		c2.Add(1)
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("e1", "1"))
	if c1.Load() != 1 {
		t.Errorf("expected testEvent handler called once, got %d", c1.Load())
	}
	if c2.Load() != 0 {
		t.Errorf("expected testEvent2 handler not called, got %d", c2.Load())
	}
}

func TestSubscribeFunc_WithRetry(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var count atomic.Int32
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		count.Add(1)
		if count.Load() < 3 {
			return errors.New("fail")
		}
		return nil
	}, WithRetry(RetryPolicy{
		MaxRetries:   5,
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
	}))
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	if count.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", count.Load())
	}
}

func TestSubscribeAll_WithRetry(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var count atomic.Int32
	d.SubscribeAll(func(_ context.Context, _ Event) error {
		count.Add(1)
		if count.Load() < 2 {
			return errors.New("fail")
		}
		return nil
	}, WithRetry(RetryPolicy{
		MaxRetries:   3,
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
	}))
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	if count.Load() != 2 {
		t.Errorf("expected 2, got %d", count.Load())
	}
}

func TestSubscribe_WithMiddleware(t *testing.T) {
	t.Parallel()
	var order []string
	mw := func(next HandleFunc) HandleFunc {
		return func(ctx context.Context, event Event) error {
			order = append(order, "mw")
			return next(ctx, event)
		}
	}
	d := syncDispatcher(WithMiddleware(mw))
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		order = append(order, "handler")
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("e", "1"))
	if len(order) != 2 || order[0] != "mw" || order[1] != "handler" {
		t.Errorf("expected [mw handler], got %v", order)
	}
}

func TestSubscribe_TypeMismatch(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	eh := &spyErrorHandler{}
	d.opts.errorHandler = eh
	l := &listener{
		id: "test-mismatch",
		handler: func(_ context.Context, event Event) error {
			_, ok := event.(*testEvent2)
			if !ok {
				return ErrEventTypeMismatch
			}
			return nil
		},
	}
	d.processEvent(eventTask{
		ctx:      context.Background(),
		event:    newTestEvent("e", "1"),
		listener: l,
	})
	if len(eh.errors) != 1 || !errors.Is(eh.errors[0], ErrEventTypeMismatch) {
		t.Errorf("expected ErrEventTypeMismatch, got %v", eh.errors)
	}
}
