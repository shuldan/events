package events

import "errors"

var (
	ErrInvalidListener         = errors.New("listener must be func(context.ParentContext, T) error or have Handle(context.ParentContext, T) error method")
	ErrInvalidListenerFunction = errors.New("listener function must have signature func(context.ParentContext, T) error")
	ErrInvalidListenerMethod   = errors.New("handle method must have signature Handle(context.ParentContext, T) error")
	ErrInvalidEventType        = errors.New("eventType must be a pointer to struct")
	ErrBusClosed               = errors.New("cannot subscribe: event Dispatcher is closed")
	ErrPublishOnClosedBus      = errors.New("cannot publish: event Dispatcher is closed")
	ErrEventChannelBlocked     = errors.New("event channel blocked")
)
