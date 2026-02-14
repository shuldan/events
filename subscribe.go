package events

import (
	"context"
	"reflect"
)

type Subscription interface {
	Unsubscribe()
}

type subscription struct {
	dispatcher *Dispatcher
	listener   *listener
}

func (s *subscription) Unsubscribe() {
	s.dispatcher.removeListener(s.listener)
}

func Subscribe[T Event](d *Dispatcher, handler Handler[T], opts ...SubscribeOption) Subscription {
	return subscribe[T](d, handler.Handle, opts...)
}

func SubscribeFunc[T Event](d *Dispatcher, fn func(context.Context, T) error, opts ...SubscribeOption) Subscription {
	return subscribe[T](d, fn, opts...)
}

func subscribe[T Event](d *Dispatcher, fn func(context.Context, T) error, opts ...SubscribeOption) Subscription {
	eventType := reflect.TypeOf((*T)(nil)).Elem()
	if eventType.Kind() == reflect.Ptr {
		eventType = eventType.Elem()
	}

	subOpts := defaultSubscribeOptions()
	for _, opt := range opts {
		opt(subOpts)
	}

	wrappedHandler := func(ctx context.Context, event Event) error {
		typed, ok := event.(T)
		if !ok {
			return ErrEventTypeMismatch
		}
		return fn(ctx, typed)
	}

	finalHandler := buildChain(wrappedHandler, d.opts.middlewares)

	l := &listener{
		id:        generateID(),
		eventType: eventType,
		handler:   finalHandler,
		retry:     subOpts.retry,
	}

	d.addListener(eventType, l)

	return &subscription{dispatcher: d, listener: l}
}

func (d *Dispatcher) SubscribeAll(handler HandleFunc, opts ...SubscribeOption) Subscription {
	subOpts := defaultSubscribeOptions()
	for _, opt := range opts {
		opt(subOpts)
	}

	finalHandler := buildChain(handler, d.opts.middlewares)

	l := &listener{
		id:      generateID(),
		handler: finalHandler,
		retry:   subOpts.retry,
	}

	d.addGlobalListener(l)

	return &subscription{dispatcher: d, listener: l}
}
