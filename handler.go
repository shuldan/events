package events

import "context"

// Handler — обработчик события типа E.
type Handler[E Event] interface {
	Handle(ctx context.Context, event E) error
}
