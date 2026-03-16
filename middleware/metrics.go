package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shuldan/events"
)

// MetricsRecorder — интерфейс для записи метрик.
type MetricsRecorder interface {
	RecordEventHandled(eventType string, duration time.Duration, err error)
}

type metricsMiddleware struct {
	recorder MetricsRecorder
}

func NewMetrics(recorder MetricsRecorder) events.Middleware {
	return &metricsMiddleware{recorder: recorder}
}

type metricsNext struct {
	recorder MetricsRecorder
	next     events.Next
}

func (m *metricsMiddleware) Wrap(next events.Next) events.Next {
	return &metricsNext{recorder: m.recorder, next: next}
}

func (n *metricsNext) Handle(ctx context.Context, event events.Event) error {
	eventType := fmt.Sprintf("%T", event)
	start := time.Now()

	err := n.next.Handle(ctx, event)

	n.recorder.RecordEventHandled(eventType, time.Since(start), err)

	return err
}

// ─── InMemoryRecorder ────────────────────────────────

type Metric struct {
	EventType string
	Duration  time.Duration
	Err       error
}

type InMemoryRecorder struct {
	mu      sync.Mutex
	metrics []Metric
}

func NewInMemoryRecorder() *InMemoryRecorder {
	return &InMemoryRecorder{}
}

func (r *InMemoryRecorder) RecordEventHandled(eventType string, duration time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = append(r.metrics, Metric{
		EventType: eventType,
		Duration:  duration,
		Err:       err,
	})
}

func (r *InMemoryRecorder) Metrics() []Metric {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Metric, len(r.metrics))
	copy(result, r.metrics)
	return result
}
