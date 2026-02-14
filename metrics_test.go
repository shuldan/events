package events

import (
	"testing"
	"time"
)

func TestNoopMetrics_AllMethods(t *testing.T) {
	t.Parallel()
	m := &noopMetrics{}
	m.EventPublished("test")
	m.EventHandled("test", time.Second, nil)
	m.EventDropped("test", "reason")
	m.QueueDepth(10)
}

func TestNoopMetrics_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ MetricsCollector = (*noopMetrics)(nil)
}
