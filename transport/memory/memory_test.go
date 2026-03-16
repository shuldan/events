package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shuldan/events"
)

type fakeHandler struct {
	mu        sync.Mutex
	envelopes []events.Envelope
}

func (h *fakeHandler) Handle(_ context.Context, e events.Envelope) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.envelopes = append(h.envelopes, e)
	return nil
}

func (h *fakeHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.envelopes)
}

type errorTransportHandler struct{ err error }

func (h *errorTransportHandler) Handle(_ context.Context, _ events.Envelope) error {
	return h.err
}

func TestMemoryTransport_PublishBeforeSubscribe(t *testing.T) {
	t.Parallel()

	tr := New()
	defer tr.Close(context.Background())

	err := tr.Publish(context.Background(), events.Envelope{ID: "1"})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestMemoryTransport_PublishAfterSubscribe(t *testing.T) {
	t.Parallel()

	tr := New()

	h := &fakeHandler{}
	go tr.Subscribe(context.Background(), h)
	time.Sleep(50 * time.Millisecond)

	err := tr.Publish(context.Background(), events.Envelope{ID: "test-1"})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if h.count() != 1 {
		t.Errorf("expected 1, got %d", h.count())
	}

	tr.Close(context.Background())
}

func TestMemoryTransport_PublishAfterClose(t *testing.T) {
	t.Parallel()

	tr := New()
	tr.Close(context.Background())

	err := tr.Publish(context.Background(), events.Envelope{ID: "1"})
	if err != nil {
		t.Errorf("expected nil after close, got %v", err)
	}
}

func TestMemoryTransport_SubscribeCancelledContext(t *testing.T) {
	t.Parallel()

	tr := New()
	defer tr.Close(context.Background())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- tr.Subscribe(ctx, &fakeHandler{})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Subscribe to return")
	}
}

func TestMemoryTransport_SubscribeClosedTransport(t *testing.T) {
	t.Parallel()

	tr := New()

	done := make(chan error, 1)
	go func() {
		done <- tr.Subscribe(context.Background(), &fakeHandler{})
	}()

	time.Sleep(50 * time.Millisecond)
	tr.Close(context.Background())

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestMemoryTransport_CloseIdempotent(t *testing.T) {
	t.Parallel()

	tr := New()
	err1 := tr.Close(context.Background())
	err2 := tr.Close(context.Background())

	if err1 != nil {
		t.Errorf("first close: expected nil, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("second close: expected nil, got %v", err2)
	}
}

func TestMemoryTransport_PublishError_Propagated(t *testing.T) {
	t.Parallel()

	tr := New()

	testErr := errors.New("handler error")
	h := &errorTransportHandler{err: testErr}

	go tr.Subscribe(context.Background(), h)
	time.Sleep(50 * time.Millisecond)

	err := tr.Publish(context.Background(), events.Envelope{ID: "1"})

	if !errors.Is(err, testErr) {
		t.Errorf("expected %v, got %v", testErr, err)
	}

	tr.Close(context.Background())
}

func TestMemoryTransport_MultiplePublish(t *testing.T) {
	t.Parallel()

	tr := New()

	h := &fakeHandler{}
	go tr.Subscribe(context.Background(), h)
	time.Sleep(50 * time.Millisecond)

	for range 10 {
		tr.Publish(context.Background(), events.Envelope{ID: "e"})
	}

	if h.count() != 10 {
		t.Errorf("expected 10, got %d", h.count())
	}

	tr.Close(context.Background())
}
