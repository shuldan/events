package kafka

import "errors"

var (
	// ErrTopicNotFound is returned when a required topic does not exist and auto-creation is disabled.
	ErrTopicNotFound = errors.New("topic not found and auto-creation is disabled")

	// ErrMissingBrokers is returned when no brokers are configured.
	ErrMissingBrokers = errors.New("brokers are required")

	// ErrMissingTopic is returned when topic is not configured.
	ErrMissingTopic = errors.New("topic is required")

	// ErrMissingConsumerGroup is returned when consumer group is not configured but Subscribe is called.
	ErrMissingConsumerGroup = errors.New("consumer group is required for Subscribe")

	// ErrTransportClosed is returned when the transport is already closed.
	ErrTransportClosed = errors.New("transport is closed")
)
