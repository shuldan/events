package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type plainTestEvent struct {
	Value string
}

type keyedTestEvent struct {
	key   string
	Value string
}

func (e *keyedTestEvent) EventKey() string { return e.key }

type anotherTestEvent struct {
	Data int
}

type collectHandler[E Event] struct {
	mu     sync.Mutex
	events []E
}

func newCollectHandler[E Event]() *collectHandler[E] {
	return &collectHandler[E]{}
}

func (h *collectHandler[E]) Handle(ctx context.Context, event E) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
	return nil
}

func (h *collectHandler[E]) Events() []E {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]E, len(h.events))
	copy(result, h.events)
	return result
}

type errorHandler[E Event] struct {
	err error
}

func newErrorHandler[E Event](err error) *errorHandler[E] {
	return &errorHandler[E]{err: err}
}

func (h *errorHandler[E]) Handle(_ context.Context, _ E) error {
	return h.err
}

type countHandler[E Event] struct {
	count atomic.Int64
}

func newCountHandler[E Event]() *countHandler[E] {
	return &countHandler[E]{}
}

func (h *countHandler[E]) Handle(_ context.Context, _ E) error {
	h.count.Add(1)
	return nil
}

type slowHandler[E Event] struct {
	delay time.Duration
}

func newSlowHandler[E Event](delay time.Duration) *slowHandler[E] {
	return &slowHandler[E]{delay: delay}
}

func (h *slowHandler[E]) Handle(ctx context.Context, _ E) error {
	select {
	case <-time.After(h.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type orderRecorder[E Event] struct {
	mu    sync.Mutex
	order []string
	label string
}

func newOrderRecorder[E Event](label string) *orderRecorder[E] {
	return &orderRecorder[E]{label: label}
}

func (h *orderRecorder[E]) Handle(_ context.Context, _ E) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.order = append(h.order, h.label)
	return nil
}

func (h *orderRecorder[E]) Labels() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]string, len(h.order))
	copy(result, h.order)
	return result
}

type capturedError struct {
	mu     sync.Mutex
	events []Event
	errs   []error
}

func newCapturedError() *capturedError {
	return &capturedError{}
}

func (c *capturedError) handler() ErrorHandler {
	return func(_ context.Context, event Event, err error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.events = append(c.events, event)
		c.errs = append(c.errs, err)
	}
}

func (c *capturedError) errors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]error, len(c.errs))
	copy(result, c.errs)
	return result
}

func waitFor(t interface{ Fatal(...any) }, ch <-chan struct{}, timeout time.Duration) {
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal("timed out waiting")
	}
}

func waitForCondition(timeout time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

var (
	errTest  = errors.New("test error")
	errOther = errors.New("other error")
)

type testMiddleware struct {
	called bool
}

type testMiddlewareNext struct {
	mw   *testMiddleware
	next Next
}

func (m *testMiddleware) Wrap(next Next) Next {
	return &testMiddlewareNext{mw: m, next: next}
}

func (n *testMiddlewareNext) Handle(ctx context.Context, event Event) error {
	n.mw.called = true
	return n.next.Handle(ctx, event)
}

type testCodec struct{}

func (c *testCodec) Encode(_ Event) ([]byte, error) { return []byte("{}"), nil }
func (c *testCodec) Decode(_ []byte, _ Event) error { return nil }
func (c *testCodec) ContentType() string            { return "application/test" }

type testTransport struct {
	published []Envelope
	handler   TransportHandler
}

func (t *testTransport) Publish(_ context.Context, e Envelope) error {
	t.published = append(t.published, e)
	return nil
}

func (t *testTransport) Subscribe(_ context.Context, h TransportHandler) error {
	t.handler = h
	<-make(chan struct{})
	return nil
}

func (t *testTransport) Close(_ context.Context) error { return nil }
