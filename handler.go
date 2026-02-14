package events

import "log/slog"

type PanicHandler interface {
	Handle(event Event, panicValue any, stack []byte)
}

type ErrorHandler interface {
	Handle(event Event, err error)
}

type defaultPanicHandler struct{}

func (d *defaultPanicHandler) Handle(event Event, panicValue any, stack []byte) {
	slog.Error("event bus panic",
		"event", event.EventName(),
		"aggregate_id", event.AggregateID(),
		"panic", panicValue,
		"stack", string(stack),
	)
}

type defaultErrorHandler struct{}

func (d *defaultErrorHandler) Handle(event Event, err error) {
	slog.Error("event bus handler error",
		"event", event.EventName(),
		"aggregate_id", event.AggregateID(),
		"error", err,
	)
}
