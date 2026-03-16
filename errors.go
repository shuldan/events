package events

import "errors"

var (
	// ErrDispatcherClosed — публикация в закрытый dispatcher.
	ErrDispatcherClosed = errors.New("events: dispatcher is closed")

	// ErrNilHandler — передан nil обработчик.
	ErrNilHandler = errors.New("events: handler is nil")
)
