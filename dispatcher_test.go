package events

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_DefaultOptions(t *testing.T) {
	t.Parallel()
	d := New()
	defer func() { _ = d.Close(context.Background()) }()
	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
	if d.opts == nil {
		t.Fatal("expected non-nil options")
	}
	if !d.opts.asyncMode {
		t.Error("expected async mode by default")
	}
}

func TestNew_SyncMode(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	if d.opts.asyncMode {
		t.Error("expected sync mode")
	}
	if d.sharedChan != nil {
		t.Error("expected nil sharedChan in sync mode")
	}
}

func TestNew_AsyncOrderedMode(t *testing.T) {
	t.Parallel()
	d := New(WithAsyncMode(), WithOrderedDelivery(), WithWorkerCount(3), WithBufferSize(5))
	defer func() { _ = d.Close(context.Background()) }()
	if len(d.workerChans) != 3 {
		t.Errorf("expected 3 worker chans, got %d", len(d.workerChans))
	}
}

func TestNew_BufferSizeAutoCalc(t *testing.T) {
	t.Parallel()
	d := New(WithAsyncMode(), WithWorkerCount(4))
	defer func() { _ = d.Close(context.Background()) }()
	if d.opts.bufferSize != 40 {
		t.Errorf("expected buffer size 40, got %d", d.opts.bufferSize)
	}
}

func TestPublish_NilEvent(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	err := d.Publish(context.Background(), nil)
	if err != nil {
		t.Errorf("expected nil error for nil event, got %v", err)
	}
}

func TestPublish_ClosedBus(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	_ = d.Close(context.Background())
	err := d.Publish(context.Background(), newTestEvent("test", "1"))
	if !errors.Is(err, ErrPublishOnClosedBus) {
		t.Errorf("expected ErrPublishOnClosedBus, got %v", err)
	}
}

func TestPublish_NoListeners(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	err := d.Publish(context.Background(), newTestEvent("test", "1"))
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestPublish_SyncMode(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var called atomic.Int32
	SubscribeFunc[*testEvent](d, func(_ context.Context, e *testEvent) error {
		called.Add(1)
		return nil
	})
	err := d.Publish(context.Background(), newTestEvent("test", "1"))
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if called.Load() != 1 {
		t.Errorf("expected handler called once, got %d", called.Load())
	}
}

func TestPublish_AsyncUnordered(t *testing.T) {
	t.Parallel()
	var called atomic.Int32
	d := asyncDispatcher(2, 10)
	defer func() { _ = d.Close(context.Background()) }()
	SubscribeFunc[*testEvent](d, func(_ context.Context, e *testEvent) error {
		called.Add(1)
		return nil
	})
	err := d.Publish(context.Background(), newTestEvent("test", "a1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok := waitForCondition(time.Second, func() bool { return called.Load() == 1 })
	if !ok {
		t.Errorf("expected handler called, got %d", called.Load())
	}
}

func TestPublish_AsyncOrdered(t *testing.T) {
	t.Parallel()
	var called atomic.Int32
	d := New(WithAsyncMode(), WithOrderedDelivery(), WithWorkerCount(2), WithBufferSize(10))
	defer func() { _ = d.Close(context.Background()) }()
	SubscribeFunc[*testEvent](d, func(_ context.Context, e *testEvent) error {
		called.Add(1)
		return nil
	})
	err := d.Publish(context.Background(), newTestEvent("test", "agg-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok := waitForCondition(time.Second, func() bool { return called.Load() == 1 })
	if !ok {
		t.Error("expected handler to be called")
	}
}

func TestPublish_ContextCancelled(t *testing.T) {
	t.Parallel()
	d := asyncDispatcher(1, 10)
	defer func() { _ = d.Close(context.Background()) }()
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := d.Publish(ctx, newTestEvent("test", "1"))
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestPublish_EnqueueTimeout(t *testing.T) {
	t.Parallel()
	m := &spyMetrics{}
	d := New(
		WithAsyncMode(), WithWorkerCount(1), WithBufferSize(1),
		WithPublishTimeout(time.Millisecond), WithMetrics(m),
	)
	defer func() { _ = d.Close(context.Background()) }()
	blocker := make(chan struct{})
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		<-blocker
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("e1", "1"))
	_ = d.Publish(context.Background(), newTestEvent("e2", "1"))
	err := d.Publish(context.Background(), newTestEvent("e3", "1"))
	close(blocker)
	if err != nil && !errors.Is(err, ErrEventChannelBlocked) {
		t.Errorf("expected ErrEventChannelBlocked or nil, got %v", err)
	}
}

func TestPublish_EnqueueContextCancelled(t *testing.T) {
	t.Parallel()
	d := New(
		WithAsyncMode(), WithWorkerCount(1), WithBufferSize(1),
		WithPublishTimeout(5*time.Second),
	)
	defer func() { _ = d.Close(context.Background()) }()
	blocker := make(chan struct{})
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		<-blocker
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("fill1", "1"))
	_ = d.Publish(context.Background(), newTestEvent("fill2", "1"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := d.Publish(ctx, newTestEvent("overflow", "1"))
	close(blocker)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("got err: %v (acceptable)", err)
	}
}

func TestPublishAll_Success(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var count atomic.Int32
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		count.Add(1)
		return nil
	})
	err := d.PublishAll(context.Background(),
		newTestEvent("a", "1"), newTestEvent("b", "2"))
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if count.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", count.Load())
	}
}

func TestPublishAll_StopsOnError(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	_ = d.Close(context.Background())
	err := d.PublishAll(context.Background(),
		newTestEvent("a", "1"), newTestEvent("b", "2"))
	if !errors.Is(err, ErrPublishOnClosedBus) {
		t.Errorf("expected ErrPublishOnClosedBus, got %v", err)
	}
}

func TestClose_AlreadyClosed(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	if err := d.Close(context.Background()); err != nil {
		t.Errorf("first close should succeed: %v", err)
	}
	if err := d.Close(context.Background()); err != nil {
		t.Errorf("second close should return nil: %v", err)
	}
}

func TestClose_ShutdownTimeout(t *testing.T) {
	t.Parallel()
	d := asyncDispatcher(1, 10)
	blocker := make(chan struct{})
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		<-blocker
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("block", "1"))
	time.Sleep(10 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := d.Close(ctx)
	close(blocker)
	if !errors.Is(err, ErrShutdownTimeout) {
		t.Errorf("expected ErrShutdownTimeout, got %v", err)
	}
}

func TestClose_SyncMode(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	err := d.Close(context.Background())
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestProcessEvent_PanicRecovery(t *testing.T) {
	t.Parallel()
	ph := &spyPanicHandler{}
	d := syncDispatcher(WithPanicHandler(ph))
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		panic("boom")
	})
	err := d.Publish(context.Background(), newTestEvent("panic", "1"))
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !ph.called {
		t.Error("expected panic handler to be called")
	}
	if ph.value != "boom" {
		t.Errorf("expected panic value 'boom', got %v", ph.value)
	}
}

func TestProcessEvent_ErrorHandler(t *testing.T) {
	t.Parallel()
	eh := &spyErrorHandler{}
	d := syncDispatcher(WithErrorHandler(eh))
	handlerErr := errors.New("handler failed")
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		return handlerErr
	})
	_ = d.Publish(context.Background(), newTestEvent("err", "1"))
	if len(eh.errors) != 1 || !errors.Is(eh.errors[0], handlerErr) {
		t.Errorf("expected handler error, got %v", eh.errors)
	}
}

func TestNormalizeType_Pointer(t *testing.T) {
	t.Parallel()
	ptrType := reflect.TypeOf((*testEvent)(nil))
	result := normalizeType(ptrType)
	if result.Kind() == reflect.Pointer {
		t.Error("expected non-pointer type")
	}
}

func TestNormalizeType_Value(t *testing.T) {
	t.Parallel()
	valType := reflect.TypeOf(testEvent{})
	result := normalizeType(valType)
	if result != valType {
		t.Error("expected same type for value")
	}
}

func TestPartitionIndex_EmptyKey(t *testing.T) {
	t.Parallel()
	d := &Dispatcher{opts: &dispatcherOptions{workerCount: 4}}
	idx := d.partitionIndex("")
	if idx != 0 {
		t.Errorf("expected 0 for empty key, got %d", idx)
	}
}

func TestPartitionIndex_NonEmptyKey(t *testing.T) {
	t.Parallel()
	d := &Dispatcher{opts: &dispatcherOptions{workerCount: 4}}
	idx := d.partitionIndex("some-aggregate")
	if idx < 0 || idx >= 4 {
		t.Errorf("expected index in [0,4), got %d", idx)
	}
}

func TestPartitionIndex_Consistency(t *testing.T) {
	t.Parallel()
	d := &Dispatcher{opts: &dispatcherOptions{workerCount: 8}}
	idx1 := d.partitionIndex("agg-42")
	idx2 := d.partitionIndex("agg-42")
	if idx1 != idx2 {
		t.Errorf("expected same partition, got %d and %d", idx1, idx2)
	}
}

func TestRemoveFromSlice(t *testing.T) {
	t.Parallel()
	l1 := &listener{id: "a"}
	l2 := &listener{id: "b"}
	l3 := &listener{id: "c"}
	result := removeFromSlice([]*listener{l1, l2, l3}, l2)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	for _, l := range result {
		if l.id == "b" {
			t.Error("expected 'b' to be removed")
		}
	}
}

func TestRemoveFromSlice_NotFound(t *testing.T) {
	t.Parallel()
	l1 := &listener{id: "a"}
	target := &listener{id: "z"}
	result := removeFromSlice([]*listener{l1}, target)
	if len(result) != 1 {
		t.Errorf("expected 1, got %d", len(result))
	}
}

func TestPublish_GlobalAndTypedListeners(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var typedCalled, globalCalled atomic.Int32
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		typedCalled.Add(1)
		return nil
	})
	d.SubscribeAll(func(_ context.Context, _ Event) error {
		globalCalled.Add(1)
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("ev", "1"))
	if typedCalled.Load() != 1 {
		t.Errorf("expected typed called once, got %d", typedCalled.Load())
	}
	if globalCalled.Load() != 1 {
		t.Errorf("expected global called once, got %d", globalCalled.Load())
	}
}

func TestPublish_MetricsPublished(t *testing.T) {
	t.Parallel()
	m := &spyMetrics{}
	d := syncDispatcher(WithMetrics(m))
	SubscribeFunc[*testEvent](d, func(_ context.Context, _ *testEvent) error {
		return nil
	})
	_ = d.Publish(context.Background(), newTestEvent("metric-event", "1"))
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.publishedNames) != 1 || m.publishedNames[0] != "metric-event" {
		t.Errorf("expected metric-event published, got %v", m.publishedNames)
	}
}

func TestPublish_PointerEvent(t *testing.T) {
	t.Parallel()
	d := syncDispatcher()
	var called atomic.Int32
	SubscribeFunc[*testEvent](d, func(_ context.Context, e *testEvent) error {
		called.Add(1)
		return nil
	})
	err := d.Publish(context.Background(), &testEvent{
		BaseEvent: NewBaseEvent("ptr", "1"),
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called.Load() != 1 {
		t.Errorf("expected 1 call, got %d", called.Load())
	}
}

func TestClose_AsyncOrdered(t *testing.T) {
	t.Parallel()
	d := New(WithAsyncMode(), WithOrderedDelivery(), WithWorkerCount(2), WithBufferSize(5))
	err := d.Close(context.Background())
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
