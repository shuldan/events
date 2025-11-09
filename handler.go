package events

import "log/slog"

type defaultPanicHandler struct{}

func newDefaultPanicHandler() PanicHandler {
	return &defaultPanicHandler{}
}

func (d *defaultPanicHandler) Handle(event any, listener any, panicValue any, stack []byte) {
	slog.Error(
		"event eventBus panic",
		"event", event,
		"listener", listener,
		"panic_value", panicValue,
		"stack", string(stack),
	)
}

type defaultErrorHandler struct{}

func newDefaultErrorHandler() ErrorHandler {
	return &defaultErrorHandler{}
}

func (d *defaultErrorHandler) Handle(event any, listener any, err error) {
	slog.Error(
		"event eventBus error",
		"event", event,
		"listener", listener,
		"error", err,
	)
}
