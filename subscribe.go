package events

import (
	"context"
	"reflect"
	"time"
)

type Subscription interface {
	Unsubscribe()
}

type subscription struct {
	eventType  reflect.Type
	next       Next // итоговая цепочка
	dispatcher *Dispatcher
}

func (s *subscription) Unsubscribe() {
	s.dispatcher.unsubscribe(s)
}

// handlerAdapter — адаптер из Handler[E] в Next.
type handlerAdapter[E Event] struct {
	handler Handler[E]
}

func (a *handlerAdapter[E]) Handle(ctx context.Context, event Event) error {
	typed, ok := event.(E)
	if !ok {
		return nil
	}
	return a.handler.Handle(ctx, typed)
}

// retryNext — обёртка retry вокруг Next.
type retryNext struct {
	policy RetryPolicy
	next   Next
}

func (r *retryNext) Handle(ctx context.Context, event Event) error {
	return retry(ctx, r.policy, func(ctx context.Context) error {
		return r.next.Handle(ctx, event)
	})
}

// timeoutNext — обёртка timeout вокруг Next.
type timeoutNext struct {
	timeout time.Duration
	next    Next
}

func (t *timeoutNext) Handle(ctx context.Context, event Event) error {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.next.Handle(ctx, event)
}

// Subscribe регистрирует типизированный обработчик.
func Subscribe[E Event](d *Dispatcher, handler Handler[E], opts ...SubscribeOption) Subscription {
	if handler == nil {
		panic(ErrNilHandler)
	}

	eventType := reflect.TypeFor[E]()

	// Мержим опции.
	cfg := defaultSubscribeConfig()
	for _, opt := range d.config.defaultSubOpts {
		opt(&cfg)
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Базовый обработчик.
	var base Next = &handlerAdapter[E]{handler: handler}

	// Retry.
	if cfg.retry != nil {
		base = &retryNext{policy: *cfg.retry, next: base}
	}

	// Timeout (оборачивает retry — таймаут на всю попытку с ретраями).
	if cfg.timeout > 0 {
		base = &timeoutNext{timeout: cfg.timeout, next: base}
	}

	// Middleware: global → per-subscriber → handler.
	allMiddleware := make([]Middleware, 0, len(d.config.middleware)+len(cfg.middleware))
	allMiddleware = append(allMiddleware, d.config.middleware...)
	allMiddleware = append(allMiddleware, cfg.middleware...)

	chain := buildChain(allMiddleware, base)

	sub := &subscription{
		eventType:  eventType,
		next:       chain,
		dispatcher: d,
	}

	d.subscribe(sub)

	return sub
}
