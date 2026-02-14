package events

import (
	"sync"
	"sync/atomic"
	"time"
)

type testEvent struct {
	BaseEvent
	Data string
}

type testEvent2 struct {
	BaseEvent
}

func newTestEvent(name, aggrID string) *testEvent {
	return &testEvent{
		BaseEvent: NewBaseEvent(name, aggrID),
	}
}

func newTestEvent2(name, aggrID string) *testEvent2 {
	return &testEvent2{
		BaseEvent: NewBaseEvent(name, aggrID),
	}
}

type spyMetrics struct {
	mu             sync.Mutex
	publishedNames []string
	handledNames   []string
	droppedNames   []string
	droppedReasons []string
	depths         []int
	handledErrs    []error
}

func (s *spyMetrics) EventPublished(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publishedNames = append(s.publishedNames, name)
}

func (s *spyMetrics) EventHandled(name string, _ time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handledNames = append(s.handledNames, name)
	s.handledErrs = append(s.handledErrs, err)
}

func (s *spyMetrics) EventDropped(name string, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.droppedNames = append(s.droppedNames, name)
	s.droppedReasons = append(s.droppedReasons, reason)
}

func (s *spyMetrics) QueueDepth(depth int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.depths = append(s.depths, depth)
}

type spyPanicHandler struct {
	mu     sync.Mutex
	called bool
	value  any
}

func (s *spyPanicHandler) Handle(_ Event, v any, _ []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called = true
	s.value = v
}

type spyErrorHandler struct {
	mu     sync.Mutex
	errors []error
}

func (s *spyErrorHandler) Handle(_ Event, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, err)
}

func waitForCondition(timeout time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

var _ Event = (*testEvent)(nil)
var _ Event = (*testEvent2)(nil)
var _ MetricsCollector = (*spyMetrics)(nil)
var _ PanicHandler = (*spyPanicHandler)(nil)
var _ ErrorHandler = (*spyErrorHandler)(nil)

func syncDispatcher(opts ...Option) *Dispatcher {
	allOpts := []Option{WithSyncMode()}
	allOpts = append(allOpts, opts...)
	return New(allOpts...)
}

func asyncDispatcher(workers, buf int, opts ...Option) *Dispatcher {
	allOpts := []Option{WithAsyncMode(), WithWorkerCount(workers), WithBufferSize(buf)}
	allOpts = append(allOpts, opts...)
	return New(allOpts...)
}

var _ atomic.Int32
