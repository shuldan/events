package events

// Event — маркерный интерфейс для всех доменных событий.
type Event any

// KeyedEvent — событие с ключом для гарантии порядка.
// События с одинаковым ключом обрабатываются последовательно.
type KeyedEvent interface {
	Event
	EventKey() string
}
