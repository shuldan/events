package events

import "context"

type HandleFunc func(ctx context.Context, event Event) error

type Handler[T Event] interface {
	Handle(ctx context.Context, event T) error
}

type HandlerFunc[T Event] func(ctx context.Context, event T) error

func (f HandlerFunc[T]) Handle(ctx context.Context, event T) error {
	return f(ctx, event)
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
	PublishAll(ctx context.Context, events ...Event) error
}

type Subscriber interface {
	SubscribeAll(handler HandleFunc, opts ...SubscribeOption) Subscription
}

type EventBus interface {
	Publisher
	Subscriber
	Close(ctx context.Context) error
}
