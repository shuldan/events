package events

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Dispatcher — шина событий.
type Dispatcher struct {
	config dispatcherConfig

	mu          sync.RWMutex
	subscribers map[reflect.Type][]*subscription
	closed      bool

	// Async.
	tasks    chan task
	wg       sync.WaitGroup
	stopOnce sync.Once
	stop     chan struct{}

	// Keyed ordering.
	keyedMu   sync.Mutex
	keyedChan map[string]chan task

	// Transport inbound.
	transportCancel context.CancelFunc
	transportWg     sync.WaitGroup
}

type task struct {
	ctx   context.Context
	event Event
	sub   *subscription
}

// New создаёт Dispatcher с заданными опциями.
func New(opts ...Option) *Dispatcher {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	d := &Dispatcher{
		config:      cfg,
		subscribers: make(map[reflect.Type][]*subscription),
		stop:        make(chan struct{}),
		keyedChan:   make(map[string]chan task),
	}

	if cfg.async {
		d.tasks = make(chan task, cfg.workerPoolSize*64)
		d.startWorkers()
	}

	if cfg.transport != nil {
		d.startTransportConsumer()
	}

	return d
}

// startWorkers запускает пул воркеров для обработки неупорядоченных событий.
func (d *Dispatcher) startWorkers() {
	for i := 0; i < d.config.workerPoolSize; i++ {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for t := range d.tasks {
				d.executeTask(t)
			}
		}()
	}
}

// startTransportConsumer запускает приём событий из транспорта.
func (d *Dispatcher) startTransportConsumer() {
	ctx, cancel := context.WithCancel(context.Background())
	d.transportCancel = cancel

	d.transportWg.Add(1)
	go func() {
		defer d.transportWg.Done()
		_ = d.config.transport.Subscribe(ctx, &inboundRouter{dispatcher: d})
	}()
}

// inboundRouter обрабатывает входящие сообщения из транспорта.
type inboundRouter struct {
	dispatcher *Dispatcher
}

func (r *inboundRouter) Handle(ctx context.Context, envelope Envelope) error {
	if r.dispatcher.config.codec == nil {
		return errors.New("events: codec is required for transport")
	}

	// Получаем зарегистрированный тип события.
	eventType, ok := r.dispatcher.lookupType(envelope.Type)
	if !ok {
		return nil // Нет подписчиков — пропускаем.
	}

	// Создаём экземпляр и десериализуем.
	event := reflect.New(eventType).Interface().(Event)
	if err := r.dispatcher.config.codec.Decode(envelope.Payload, event); err != nil {
		return fmt.Errorf("events: decode failed: %w", err)
	}

	return r.dispatcher.Publish(ctx, event)
}

// lookupType ищет зарегистрированный тип по строковому имени.
func (d *Dispatcher) lookupType(name string) (reflect.Type, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for t := range d.subscribers {
		if t.String() == name {
			return t, true
		}
	}
	return nil, false
}

// subscribe добавляет подписку.
func (d *Dispatcher) subscribe(sub *subscription) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers[sub.eventType] = append(d.subscribers[sub.eventType], sub)
}

// unsubscribe удаляет подписку.
func (d *Dispatcher) unsubscribe(sub *subscription) {
	d.mu.Lock()
	defer d.mu.Unlock()

	subs := d.subscribers[sub.eventType]
	for i, s := range subs {
		if s == sub {
			d.subscribers[sub.eventType] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

// Publish отправляет событие всем подписчикам.
func (d *Dispatcher) Publish(ctx context.Context, event Event) error {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return ErrDispatcherClosed
	}

	eventType := reflect.TypeOf(event)
	subs := make([]*subscription, len(d.subscribers[eventType]))
	copy(subs, d.subscribers[eventType])
	d.mu.RUnlock()

	// Публикуем в транспорт (если есть).
	if err := d.publishToTransport(ctx, event); err != nil {
		return fmt.Errorf("events: transport publish failed: %w", err)
	}

	if len(subs) == 0 {
		return nil
	}

	if d.config.async {
		return d.publishAsync(ctx, event, subs)
	}

	return d.publishSync(ctx, event, subs)
}

// PublishAll отправляет несколько событий.
func (d *Dispatcher) PublishAll(ctx context.Context, events ...Event) error {
	var errs []error

	for _, event := range events {
		if err := d.Publish(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// publishSync — синхронная доставка: все обработчики, все ошибки.
func (d *Dispatcher) publishSync(ctx context.Context, event Event, subs []*subscription) error {
	var errs []error

	for _, sub := range subs {
		if err := sub.next.Handle(ctx, event); err != nil {
			if d.config.errorHandler != nil {
				d.config.errorHandler(ctx, event, err)
			}
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// publishAsync — асинхронная доставка с учётом ordering key.
func (d *Dispatcher) publishAsync(ctx context.Context, event Event, subs []*subscription) error {
	key := eventKey(event)

	for _, sub := range subs {
		t := task{ctx: ctx, event: event, sub: sub}

		if key != "" {
			d.dispatchKeyed(key, t)
		} else {
			select {
			case d.tasks <- t:
			case <-d.stop:
				return ErrDispatcherClosed
			}
		}
	}

	return nil
}

// dispatchKeyed направляет задачу в очередь ключа.
func (d *Dispatcher) dispatchKeyed(key string, t task) {
	d.keyedMu.Lock()

	ch, exists := d.keyedChan[key]
	if !exists {
		ch = make(chan task, 64)
		d.keyedChan[key] = ch

		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for kt := range ch {
				d.executeTask(kt)
			}
			// Горутина завершается, удаляем канал.
			d.keyedMu.Lock()
			delete(d.keyedChan, key)
			d.keyedMu.Unlock()
		}()
	}

	d.keyedMu.Unlock()

	ch <- t
}

// executeTask выполняет задачу и обрабатывает ошибку.
func (d *Dispatcher) executeTask(t task) {
	err := t.sub.next.Handle(t.ctx, t.event)
	if err != nil && d.config.errorHandler != nil {
		d.config.errorHandler(t.ctx, t.event, err)
	}
}

// publishToTransport публикует событие во внешний транспорт.
func (d *Dispatcher) publishToTransport(ctx context.Context, event Event) error {
	if d.config.transport == nil || d.config.codec == nil {
		return nil
	}

	payload, err := d.config.codec.Encode(event)
	if err != nil {
		return fmt.Errorf("events: encode failed: %w", err)
	}

	envelope := Envelope{
		ID:          uuid.New().String(),
		Type:        reflect.TypeOf(event).String(),
		Key:         eventKey(event),
		Payload:     payload,
		ContentType: d.config.codec.ContentType(),
		Timestamp:   time.Now(),
	}

	return d.config.transport.Publish(ctx, envelope)
}

// eventKey извлекает ключ из события, если реализован KeyedEvent.
func eventKey(event Event) string {
	if keyed, ok := event.(KeyedEvent); ok {
		return keyed.EventKey()
	}
	return ""
}

// Close завершает работу Dispatcher.
// Дожидается завершения всех in-flight обработчиков.
func (d *Dispatcher) Close(ctx context.Context) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	d.mu.Unlock()

	var errs []error

	// Останавливаем приём из транспорта.
	if d.transportCancel != nil {
		d.transportCancel()
		d.transportWg.Wait()
	}

	// Останавливаем воркеры.
	if d.config.async {
		d.stopOnce.Do(func() {
			close(d.stop)
			close(d.tasks)

			// Закрываем keyed каналы.
			d.keyedMu.Lock()
			for _, ch := range d.keyedChan {
				close(ch)
			}
			d.keyedMu.Unlock()
		})

		// Ждём завершения с учётом контекста.
		done := make(chan struct{})
		go func() {
			d.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		}
	}

	// Закрываем транспорт.
	if d.config.transport != nil {
		if err := d.config.transport.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
