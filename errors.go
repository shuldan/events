package events

import "errors"

var (
	ErrBusClosed           = errors.New("events: bus is closed")
	ErrPublishOnClosedBus  = errors.New("events: cannot publish on closed bus")
	ErrEventChannelBlocked = errors.New("events: event channel blocked, publish timeout exceeded")
	ErrEventTypeMismatch   = errors.New("events: event type mismatch in handler")
	ErrShutdownTimeout     = errors.New("events: graceful shutdown timed out")
)
