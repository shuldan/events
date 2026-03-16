package events

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNew_DefaultSync(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	if d.config.async {
		t.Error("expected sync mode by default")
	}
}

func TestNew_AsyncMode(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerPool(2))
	defer d.Close(context.Background())

	if !d.config.async {
		t.Error("expected async mode")
	}
}

func TestPublish_NoSubscribers(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	err := d.Publish(context.Background(), &plainTestEvent{Value: "no-one-cares"})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestPublish_ClosedDispatcher(t *testing.T) {
	t.Parallel()

	d := New()
	d.Close(context.Background())

	err := d.Publish(context.Background(), &plainTestEvent{})
	if !errors.Is(err, ErrDispatcherClosed) {
		t.Errorf("expected ErrDispatcherClosed, got %v", err)
	}
}

func TestPublish_Sync_AllHandlersCalled_OnError(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h1 := newErrorHandler[*plainTestEvent](errTest)
	h2 := newCountHandler[*plainTestEvent]()

	Subscribe(d, h1)
	Subscribe(d, h2)

	err := d.Publish(context.Background(), &plainTestEvent{})

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errTest) {
		t.Errorf("expected errTest, got %v", err)
	}
	if h2.count.Load() != 1 {
		t.Errorf("expected h2 called, got %d", h2.count.Load())
	}
}

func TestPublish_Sync_MultipleErrors_Joined(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	Subscribe(d, newErrorHandler[*plainTestEvent](errTest))
	Subscribe(d, newErrorHandler[*plainTestEvent](errOther))

	err := d.Publish(context.Background(), &plainTestEvent{})

	if !errors.Is(err, errTest) {
		t.Errorf("expected errTest in joined error")
	}
	if !errors.Is(err, errOther) {
		t.Errorf("expected errOther in joined error")
	}
}

func TestPublish_Sync_ErrorHandler_Called(t *testing.T) {
	t.Parallel()

	captured := newCapturedError()
	d := New(WithErrorHandler(captured.handler()))
	defer d.Close(context.Background())

	Subscribe(d, newErrorHandler[*plainTestEvent](errTest))

	d.Publish(context.Background(), &plainTestEvent{})

	errs := captured.errors()
	if len(errs) != 1 {
		t.Fatalf("expected 1 captured error, got %d", len(errs))
	}
	if !errors.Is(errs[0], errTest) {
		t.Errorf("expected errTest, got %v", errs[0])
	}
}

func TestPublish_Async_Delivery(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerPool(2))
	defer d.Close(context.Background())

	h := newCountHandler[*plainTestEvent]()
	Subscribe(d, h)

	d.Publish(context.Background(), &plainTestEvent{})

	ok := waitForCondition(time.Second, func() bool {
		return h.count.Load() == 1
	})
	if !ok {
		t.Errorf("expected 1 call, got %d", h.count.Load())
	}
}

func TestPublish_Async_ErrorHandler_Called(t *testing.T) {
	t.Parallel()

	captured := newCapturedError()
	d := New(WithAsyncMode(), WithWorkerPool(2), WithErrorHandler(captured.handler()))
	defer d.Close(context.Background())

	Subscribe(d, newErrorHandler[*plainTestEvent](errTest))

	d.Publish(context.Background(), &plainTestEvent{})

	ok := waitForCondition(time.Second, func() bool {
		return len(captured.errors()) == 1
	})
	if !ok {
		t.Fatal("expected error handler to be called")
	}
}

func TestPublishAll_MultipleEvents(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h := newCountHandler[*plainTestEvent]()
	Subscribe(d, h)

	err := d.PublishAll(context.Background(),
		&plainTestEvent{Value: "a"},
		&plainTestEvent{Value: "b"},
		&plainTestEvent{Value: "c"},
	)

	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if h.count.Load() != 3 {
		t.Errorf("expected 3, got %d", h.count.Load())
	}
}

func TestPublishAll_PartialErrors(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	Subscribe(d, newErrorHandler[*plainTestEvent](errTest))

	h2 := newCountHandler[*anotherTestEvent]()
	Subscribe(d, h2)

	err := d.PublishAll(context.Background(),
		&plainTestEvent{},
		&anotherTestEvent{Data: 42},
	)

	if !errors.Is(err, errTest) {
		t.Errorf("expected errTest in result")
	}
	if h2.count.Load() != 1 {
		t.Errorf("expected h2 called, got %d", h2.count.Load())
	}
}

func TestPublishAll_Empty(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	err := d.PublishAll(context.Background())
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	h := newCountHandler[*plainTestEvent]()
	sub := Subscribe(d, h)

	d.Publish(context.Background(), &plainTestEvent{})
	if h.count.Load() != 1 {
		t.Fatalf("expected 1, got %d", h.count.Load())
	}

	sub.Unsubscribe()

	d.Publish(context.Background(), &plainTestEvent{})
	if h.count.Load() != 1 {
		t.Errorf("expected still 1 after unsubscribe, got %d", h.count.Load())
	}
}

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()

	d := New()
	err1 := d.Close(context.Background())
	err2 := d.Close(context.Background())

	if err1 != nil {
		t.Errorf("first close: expected nil, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("second close: expected nil, got %v", err2)
	}
}

func TestClose_Async_WaitsForInFlight(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerPool(1))

	started := make(chan struct{})
	finished := make(chan struct{})

	handler := &signalHandler[*plainTestEvent]{
		started:  started,
		finished: finished,
		delay:    200 * time.Millisecond,
	}
	Subscribe(d, handler)

	d.Publish(context.Background(), &plainTestEvent{})
	waitFor(t, started, time.Second)

	err := d.Close(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	select {
	case <-finished:
	default:
		t.Error("expected handler to finish before Close returns")
	}
}

func TestClose_Async_ContextTimeout(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerPool(1))

	handler := newSlowHandler[*plainTestEvent](10 * time.Second)
	Subscribe(d, handler)

	d.Publish(context.Background(), &plainTestEvent{})
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := d.Close(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

type signalHandler[E Event] struct {
	started  chan struct{}
	finished chan struct{}
	delay    time.Duration
}

func (h *signalHandler[E]) Handle(_ context.Context, _ E) error {
	close(h.started)
	time.Sleep(h.delay)
	close(h.finished)
	return nil
}
