package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type TestEvent struct {
	Value string
}

type AnotherEvent struct {
	ID int
}

type testPanicHandler struct {
	mu    sync.Mutex
	calls []panicCall
}

type panicCall struct {
	event      any
	listener   any
	panicValue any
}

func (m *testPanicHandler) Handle(event any, listener any, panicValue any, stack []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, panicCall{event, listener, panicValue})
}

func (m *testPanicHandler) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

type testErrorHandler struct {
	mu    sync.Mutex
	calls []errorCall
}

type errorCall struct {
	event    any
	listener any
	err      error
}

func (m *testErrorHandler) Handle(event any, listener any, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, errorCall{event, listener, err})
}

func (m *testErrorHandler) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestNew_DefaultOptions(t *testing.T) {
	t.Parallel()
	d := New()
	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
	if d.listeners == nil {
		t.Error("expected listeners map to be initialized")
	}
	if d.closed {
		t.Error("expected dispatcher to not be closed")
	}
	if d.eventChan == nil {
		t.Error("expected event channel to be initialized")
	}
	d.Close()
}

func TestNew_WithOptions(t *testing.T) {
	t.Parallel()
	ph := &testPanicHandler{}
	eh := &testErrorHandler{}
	d := New(
		WithPanicHandler(ph),
		WithErrorHandler(eh),
		WithWorkerCount(5),
	)
	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
	d.Close()
}

func TestDispatcher_Subscribe_ValidFunction(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	fn := func(ctx context.Context, event TestEvent) error {
		return nil
	}

	err := d.Subscribe(&TestEvent{}, fn)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDispatcher_Subscribe_EventTypeMismatch(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	fnForTestEvent := func(ctx context.Context, event TestEvent) error {
		return nil
	}

	err := d.Subscribe(&AnotherEvent{}, fnForTestEvent)
	if !errors.Is(err, ErrInvalidListener) {
		t.Errorf("expected ErrInvalidListener, got %v", err)
	}
}

func TestDispatcher_Subscribe_InvalidEventType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		eventType any
	}{
		{"nil", nil},
		{"not_pointer", TestEvent{}},
		{"pointer_to_primitive", new(int)},
		{"string", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New()
			defer d.Close()

			fn := func(ctx context.Context, event TestEvent) error {
				return nil
			}

			err := d.Subscribe(tt.eventType, fn)
			if !errors.Is(err, ErrInvalidEventType) {
				t.Errorf("expected ErrInvalidEventType, got %v", err)
			}
		})
	}
}

func TestDispatcher_Subscribe_ClosedBus(t *testing.T) {
	t.Parallel()
	d := New()
	d.Close()

	fn := func(ctx context.Context, event TestEvent) error {
		return nil
	}

	err := d.Subscribe(&TestEvent{}, fn)
	if !errors.Is(err, ErrBusClosed) {
		t.Errorf("expected ErrBusClosed, got %v", err)
	}
}

func TestDispatcher_Subscribe_InvalidListener(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	err := d.Subscribe(&TestEvent{}, "not a function")
	if err == nil {
		t.Error("expected error for invalid listener")
	}
}

func TestDispatcher_Publish_AsyncMode(t *testing.T) {
	t.Parallel()
	d := New(WithAsyncMode())
	defer d.Close()

	var received atomic.Bool
	fn := func(ctx context.Context, event TestEvent) error {
		received.Store(true)
		return nil
	}

	d.Subscribe(&TestEvent{}, fn)
	err := d.Publish(context.Background(), TestEvent{Value: "test"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	waitForCondition(t, 100*time.Millisecond, func() bool {
		return received.Load()
	})
}

func TestDispatcher_Publish_SyncMode(t *testing.T) {
	t.Parallel()
	d := New(WithSyncMode())
	defer d.Close()

	var received atomic.Bool
	fn := func(ctx context.Context, event TestEvent) error {
		received.Store(true)
		return nil
	}

	d.Subscribe(&TestEvent{}, fn)
	err := d.Publish(context.Background(), TestEvent{Value: "test"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !received.Load() {
		t.Error("expected event to be received immediately in sync mode")
	}
}

func TestDispatcher_Publish_SyncModeMultipleListeners(t *testing.T) {
	t.Parallel()
	d := New(WithSyncMode())
	defer d.Close()

	var count1, count2 atomic.Int32
	fn1 := func(ctx context.Context, event TestEvent) error {
		count1.Add(1)
		return nil
	}
	fn2 := func(ctx context.Context, event TestEvent) error {
		count2.Add(1)
		return nil
	}

	d.Subscribe(&TestEvent{}, fn1)
	d.Subscribe(&TestEvent{}, fn2)
	d.Publish(context.Background(), TestEvent{Value: "test"})

	if count1.Load() != 1 {
		t.Errorf("expected listener1 count=1, got %d", count1.Load())
	}
	if count2.Load() != 1 {
		t.Errorf("expected listener2 count=1, got %d", count2.Load())
	}
}

func TestDispatcher_Publish_NilEvent(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	err := d.Publish(context.Background(), nil)
	if err != nil {
		t.Errorf("expected no error for nil event, got %v", err)
	}
}

func TestDispatcher_Publish_NoListeners(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	err := d.Publish(context.Background(), TestEvent{})
	if err != nil {
		t.Errorf("expected no error when no listeners, got %v", err)
	}
}

func TestDispatcher_Publish_ClosedBus(t *testing.T) {
	t.Parallel()
	d := New()
	d.Close()

	err := d.Publish(context.Background(), TestEvent{})
	if !errors.Is(err, ErrPublishOnClosedBus) {
		t.Errorf("expected ErrPublishOnClosedBus, got %v", err)
	}
}

func TestDispatcher_Publish_ContextCancelled(t *testing.T) {
	t.Parallel()
	d := New(WithWorkerCount(1))
	defer d.Close()

	fn := func(ctx context.Context, event TestEvent) error {
		return nil
	}
	d.Subscribe(&TestEvent{}, fn)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := d.Publish(ctx, TestEvent{})
	if err == nil {
		t.Error("expected context error")
	}
}

func TestDispatcher_Publish_ChannelBlockTimeout(t *testing.T) {
	t.Parallel()
	d := New(WithWorkerCount(1))
	defer d.Close()

	blockChan := make(chan struct{})
	fn := func(ctx context.Context, event TestEvent) error {
		<-blockChan
		return nil
	}
	d.Subscribe(&TestEvent{}, fn)

	for i := 0; i < 11; i++ {
		d.Publish(context.Background(), TestEvent{})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := d.Publish(ctx, TestEvent{})
	close(blockChan)

	if err == nil {
		t.Error("expected error due to context or channel block")
	}
}

func TestDispatcher_Close_MultipleCalls(t *testing.T) {
	t.Parallel()
	d := New()

	err1 := d.Close()
	if err1 != nil {
		t.Errorf("first close: expected no error, got %v", err1)
	}

	err2 := d.Close()
	if err2 != nil {
		t.Errorf("second close: expected no error, got %v", err2)
	}
}

func TestDispatcher_ProcessEvent_WithPanic(t *testing.T) {
	t.Parallel()
	ph := &testPanicHandler{calls: make([]panicCall, 0)}
	d := New(WithPanicHandler(ph))
	defer d.Close()

	fn := func(ctx context.Context, event TestEvent) error {
		panic("test panic")
	}

	d.Subscribe(&TestEvent{}, fn)
	d.Publish(context.Background(), TestEvent{})

	waitForCondition(t, 100*time.Millisecond, func() bool {
		return ph.callCount() > 0
	})
}

func TestDispatcher_ProcessEvent_WithError(t *testing.T) {
	t.Parallel()
	eh := &testErrorHandler{calls: make([]errorCall, 0)}
	d := New(WithErrorHandler(eh))
	defer d.Close()

	testErr := errors.New("test error")
	fn := func(ctx context.Context, event TestEvent) error {
		return testErr
	}

	d.Subscribe(&TestEvent{}, fn)
	d.Publish(context.Background(), TestEvent{})

	waitForCondition(t, 100*time.Millisecond, func() bool {
		return eh.callCount() > 0
	})
}

func TestDispatcher_ProcessEvent_SyncModeWithPanic(t *testing.T) {
	t.Parallel()
	ph := &testPanicHandler{calls: make([]panicCall, 0)}
	d := New(WithSyncMode(), WithPanicHandler(ph))
	defer d.Close()

	fn := func(ctx context.Context, event TestEvent) error {
		panic("sync panic")
	}

	d.Subscribe(&TestEvent{}, fn)
	d.Publish(context.Background(), TestEvent{})

	if ph.callCount() == 0 {
		t.Error("expected panic handler to be called in sync mode")
	}
}

func TestDispatcher_ProcessEvent_SyncModeWithError(t *testing.T) {
	t.Parallel()
	eh := &testErrorHandler{calls: make([]errorCall, 0)}
	d := New(WithSyncMode(), WithErrorHandler(eh))
	defer d.Close()

	testErr := errors.New("sync error")
	fn := func(ctx context.Context, event TestEvent) error {
		return testErr
	}

	d.Subscribe(&TestEvent{}, fn)
	d.Publish(context.Background(), TestEvent{})

	if eh.callCount() == 0 {
		t.Error("expected error handler to be called in sync mode")
	}
}
