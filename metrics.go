package events

import "time"

type MetricsCollector interface {
	EventPublished(eventName string)
	EventHandled(eventName string, duration time.Duration, err error)
	EventDropped(eventName string, reason string)
	QueueDepth(depth int)
}

type noopMetrics struct{}

func (n *noopMetrics) EventPublished(string)                     {}
func (n *noopMetrics) EventHandled(string, time.Duration, error) {}
func (n *noopMetrics) EventDropped(string, string)               {}
func (n *noopMetrics) QueueDepth(int)                            {}
