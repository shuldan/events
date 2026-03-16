package events

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPublish_Async_KeyedEvent_OrderPreserved(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerPool(4))
	defer d.Close(context.Background())

	var mu sync.Mutex
	var received []string

	h := &keyedCollector{mu: &mu, received: &received}
	Subscribe(d, h)

	for i := range 20 {
		d.Publish(context.Background(), &keyedTestEvent{
			key:   "same-key",
			Value: string('A' + rune(i)),
		})
	}

	ok := waitForCondition(2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 20
	})
	if !ok {
		mu.Lock()
		t.Fatalf("expected 20 events, got %d", len(received))
		mu.Unlock()
	}

	mu.Lock()
	defer mu.Unlock()
	for i := range 20 {
		expected := string('A' + rune(i))
		if received[i] != expected {
			t.Errorf("index %d: expected %q, got %q", i, expected, received[i])
		}
	}
}

func TestPublish_Async_DifferentKeys_Parallel(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerPool(4))
	defer d.Close(context.Background())

	h := newCountHandler[*keyedTestEvent]()
	Subscribe(d, h)

	for i := range 50 {
		key := string('A' + rune(i%5))
		d.Publish(context.Background(), &keyedTestEvent{key: key, Value: "v"})
	}

	ok := waitForCondition(2*time.Second, func() bool {
		return h.count.Load() == 50
	})
	if !ok {
		t.Errorf("expected 50, got %d", h.count.Load())
	}
}

func TestPublish_Async_NonKeyed_NoOrdering(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerPool(4))
	defer d.Close(context.Background())

	h := newCountHandler[*plainTestEvent]()
	Subscribe(d, h)

	for range 20 {
		d.Publish(context.Background(), &plainTestEvent{Value: "v"})
	}

	ok := waitForCondition(2*time.Second, func() bool {
		return h.count.Load() == 20
	})
	if !ok {
		t.Errorf("expected 20, got %d", h.count.Load())
	}
}

func TestPublish_Sync_KeyedEvent_Delivered(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h := newCollectHandler[*keyedTestEvent]()
	var handler Handler[*keyedTestEvent] = h
	Subscribe(d, handler)

	d.Publish(context.Background(), &keyedTestEvent{key: "k1", Value: "v1"})
	d.Publish(context.Background(), &keyedTestEvent{key: "k2", Value: "v2"})

	events := h.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2, got %d", len(events))
	}
}

type keyedCollector struct {
	mu       *sync.Mutex
	received *[]string
}

func (h *keyedCollector) Handle(_ context.Context, e *keyedTestEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.received = append(*h.received, e.Value)
	return nil
}
