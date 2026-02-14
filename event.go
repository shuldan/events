package events

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Event — контракт доменного события.
type Event interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() string
}

// BaseEvent — базовая структура для встраивания в конкретные события.
// Методы реализованы на значении, чтобы работать и с *T, и с T.
type BaseEvent struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Timestamp time.Time `json:"timestamp"`
	AggrID    string    `json:"aggregate_id"`
}

func NewBaseEvent(name string, aggregateID string) BaseEvent {
	return BaseEvent{
		ID:        generateID(),
		Name:      name,
		Timestamp: time.Now().UTC(),
		AggrID:    aggregateID,
	}
}

func (e BaseEvent) EventName() string     { return e.Name }
func (e BaseEvent) OccurredAt() time.Time { return e.Timestamp }
func (e BaseEvent) AggregateID() string   { return e.AggrID }

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}
