package memory

import (
	"context"
	"sync"

	"github.com/shuldan/events"
)

// Transport — in-memory транспорт для тестов.
// Доставляет события напрямую подписчикам внутри процесса.
type Transport struct {
	mu      sync.RWMutex
	handler events.TransportHandler
	closed  bool
	done    chan struct{}
}

// New создаёт in-memory транспорт.
func New() *Transport {
	return &Transport{
		done: make(chan struct{}),
	}
}

func (t *Transport) Publish(ctx context.Context, envelope events.Envelope) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return nil
	}

	if t.handler == nil {
		return nil
	}

	return t.handler.Handle(ctx, envelope)
}

// Subscribe блокируется до отмены контекста.
func (t *Transport) Subscribe(ctx context.Context, handler events.TransportHandler) error {
	t.mu.Lock()
	t.handler = handler
	t.mu.Unlock()

	// Блокируемся до отмены контекста или закрытия.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.done:
		return nil
	}
}

func (t *Transport) Close(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.closed {
		t.closed = true
		close(t.done)
	}

	return nil
}
