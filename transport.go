package events

import (
	"context"
	"time"
)

// Envelope — обёртка события для передачи через транспорт.
type Envelope struct {
	ID          string
	Type        string
	Key         string
	Payload     []byte
	ContentType string
	Metadata    map[string]string
	Timestamp   time.Time
}

// TransportHandler обрабатывает входящие сообщения из транспорта.
type TransportHandler interface {
	Handle(ctx context.Context, envelope Envelope) error
}

// Transport — абстракция внешнего транспорта.
type Transport interface {
	Publish(ctx context.Context, envelope Envelope) error
	Subscribe(ctx context.Context, handler TransportHandler) error
	Close(ctx context.Context) error
}
