package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIntegration_MultipleEventsAndListeners(t *testing.T) {
	t.Parallel()
	d := New(WithWorkerCount(2))
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

	for i := 0; i < 10; i++ {
		d.Publish(context.Background(), TestEvent{Value: "test"})
	}

	waitForCondition(t, 200*time.Millisecond, func() bool {
		return count1.Load() == 10 && count2.Load() == 10
	})
}

func TestIntegration_DifferentEventTypes(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	var testEventCount, anotherEventCount atomic.Int32

	d.Subscribe(&TestEvent{}, func(ctx context.Context, event TestEvent) error {
		testEventCount.Add(1)
		return nil
	})

	d.Subscribe(&AnotherEvent{}, func(ctx context.Context, event AnotherEvent) error {
		anotherEventCount.Add(1)
		return nil
	})

	d.Publish(context.Background(), TestEvent{})
	d.Publish(context.Background(), AnotherEvent{})
	d.Publish(context.Background(), TestEvent{})

	waitForCondition(t, 100*time.Millisecond, func() bool {
		return testEventCount.Load() == 2 && anotherEventCount.Load() == 1
	})
}

func TestIntegration_ConcurrentPublish(t *testing.T) {
	t.Parallel()
	d := New(WithWorkerCount(4))
	defer d.Close()

	var count atomic.Int32
	d.Subscribe(&TestEvent{}, func(ctx context.Context, event TestEvent) error {
		count.Add(1)
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				d.Publish(context.Background(), TestEvent{})
			}
		}()
	}
	wg.Wait()

	waitForCondition(t, 300*time.Millisecond, func() bool {
		return count.Load() == 100
	})
}

func TestIntegration_ErrorPropagation(t *testing.T) {
	t.Parallel()
	eh := &testErrorHandler{calls: make([]errorCall, 0)}
	d := New(WithErrorHandler(eh))
	defer d.Close()

	testErr := errors.New("test error")
	d.Subscribe(&TestEvent{}, func(ctx context.Context, event TestEvent) error {
		return testErr
	})

	d.Publish(context.Background(), TestEvent{})

	waitForCondition(t, 100*time.Millisecond, func() bool {
		return eh.callCount() == 1
	})
}

func TestIntegration_PanicRecovery(t *testing.T) {
	t.Parallel()
	ph := &testPanicHandler{calls: make([]panicCall, 0)}
	d := New(WithPanicHandler(ph))
	defer d.Close()

	d.Subscribe(&TestEvent{}, func(ctx context.Context, event TestEvent) error {
		panic("intentional panic")
	})

	d.Publish(context.Background(), TestEvent{})

	waitForCondition(t, 100*time.Millisecond, func() bool {
		return ph.callCount() == 1
	})
}

func TestIntegration_ContextCancellationDuringPublish(t *testing.T) {
	t.Parallel()
	d := New(WithWorkerCount(4))
	defer d.Close()

	blockChan := make(chan struct{})
	d.Subscribe(&TestEvent{}, func(ctx context.Context, event TestEvent) error {
		<-blockChan
		return nil
	})

	for i := 0; i < 5; i++ {
		d.Publish(context.Background(), TestEvent{})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := d.Publish(ctx, TestEvent{})
	close(blockChan)

	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestIntegration_SubscribeAfterPublish(t *testing.T) {
	t.Parallel()
	d := New()
	defer d.Close()

	d.Publish(context.Background(), TestEvent{})

	var count atomic.Int32
	d.Subscribe(&TestEvent{}, func(ctx context.Context, event TestEvent) error {
		count.Add(1)
		return nil
	})

	d.Publish(context.Background(), TestEvent{})

	waitForCondition(t, 100*time.Millisecond, func() bool {
		return count.Load() == 1
	})
}

func TestIntegration_CloseWhileProcessing(t *testing.T) {
	t.Parallel()
	d := New(WithWorkerCount(2))

	var started, completed atomic.Int32
	d.Subscribe(&TestEvent{}, func(ctx context.Context, event TestEvent) error {
		started.Add(1)
		time.Sleep(30 * time.Millisecond)
		completed.Add(1)
		return nil
	})

	for i := 0; i < 5; i++ {
		d.Publish(context.Background(), TestEvent{})
	}

	time.Sleep(10 * time.Millisecond)
	d.Close()

	if started.Load() == 0 {
		t.Error("expected some events to start processing")
	}
}

func TestIntegration_SyncModeOrdering(t *testing.T) {
	t.Parallel()
	d := New(WithSyncMode())
	defer d.Close()

	var results []int
	var mu sync.Mutex

	d.Subscribe(&AnotherEvent{}, func(ctx context.Context, event AnotherEvent) error {
		mu.Lock()
		results = append(results, event.ID)
		mu.Unlock()
		return nil
	})

	for i := 1; i <= 5; i++ {
		d.Publish(context.Background(), AnotherEvent{ID: i})
	}

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	for i := 0; i < 5; i++ {
		if results[i] != i+1 {
			t.Errorf("expected results[%d]=%d, got %d", i, i+1, results[i])
		}
	}
}
