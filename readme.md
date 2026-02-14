# `events` — Типобезопасный диспетчер доменных событий для Go

[![Go CI](https://github.com/shuldan/events/workflows/Go%20CI/badge.svg)](https://github.com/shuldan/events/actions)
[![codecov](https://codecov.io/gh/shuldan/events/branch/main/graph/badge.svg)](https://codecov.io/gh/shuldan/events)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Пакет `events` предоставляет высокопроизводительную шину доменных событий для Go-приложений, построенных по принципам DDD. Использует дженерики для типобезопасности на этапе компиляции, поддерживает синхронную и асинхронную доставку, middleware, retry-политики и упорядоченную обработку.

---

## Основные возможности

- **Типобезопасность через дженерики** — ошибки несоответствия типов обнаруживаются на этапе компиляции, а не в рантайме.
- **Структурные и функциональные обработчики** — слушателем может быть структура с методом `Handle` или обычная функция.
- **Middleware** — цепочки сквозной логики: логирование, трассировка, метрики, транзакции.
- **Retry с exponential backoff** — настраиваемая политика повторных попыток для каждой подписки.
- **Упорядоченная доставка** — события одного агрегата обрабатываются последовательно через партиционирование по `AggregateID`.
- **Wildcard-подписки** — глобальные обработчики, получающие все события (аудит, логирование).
- **Batch-публикация** — отправка нескольких событий за один вызов.
- **Отписка** — каждая подписка возвращает объект `Subscription` для управления жизненным циклом.
- **Graceful shutdown** — корректное завершение с поддержкой таймаута через контекст.
- **Observability** — интерфейс `MetricsCollector` для сбора метрик обработки.
- **Тестируемость** — интерфейсы `Publisher`, `Subscriber`, `EventBus` для подмены в тестах.

---

## Установка

Требуется **Go 1.24+**.

```sh
go get github.com/shuldan/events
```

---

## Быстрый старт

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/shuldan/events"
)

// Определяем доменное событие.
type OrderCreated struct {
    events.BaseEvent
    OrderID string
    UserID  string
    Amount  float64
}

func main() {
    // Создаём шину в синхронном режиме.
    bus := events.New(events.WithSyncMode())
    defer bus.Close(context.Background())

    // Подписываемся функцией.
    events.SubscribeFunc(bus, func(ctx context.Context, e OrderCreated) error {
        fmt.Printf("Order %s created for user %s, amount: %.2f\n",
            e.OrderID, e.UserID, e.Amount)
        return nil
    })

    // Публикуем событие.
    err := bus.Publish(context.Background(), OrderCreated{
        BaseEvent: events.NewBaseEvent("order.created", "order-1"),
        OrderID:   "order-1",
        UserID:    "user-42",
        Amount:    199.90,
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

---

## Определение событий

Каждое событие реализует интерфейс `Event`. Для удобства предусмотрена базовая структура `BaseEvent`, которую достаточно встроить.

```go
type Event interface {
    EventName() string
    OccurredAt() time.Time
    AggregateID() string
}
```

### Пример доменных событий

```go
type OrderShipped struct {
    events.BaseEvent
    OrderID    string
    TrackingNo string
    ShippedAt  time.Time
}

type PaymentReceived struct {
    events.BaseEvent
    PaymentID string
    OrderID   string
    Amount    float64
    Currency  string
}
```

`NewBaseEvent` автоматически генерирует уникальный ID и устанавливает время:

```go
evt := OrderShipped{
    BaseEvent:  events.NewBaseEvent("order.shipped", "order-123"),
    OrderID:    "order-123",
    TrackingNo: "TRACK-456",
    ShippedAt:  time.Now(),
}

evt.EventName()    // "order.shipped"
evt.AggregateID()  // "order-123"
evt.OccurredAt()   // time.Time (момент создания)
```

---

## Обработчики событий

### Структурный обработчик

Основной способ для обработчиков с зависимостями. Любая структура с методом `Handle(context.Context, T) error` автоматически реализует интерфейс `Handler[T]`.

```go
type ShippingNotificationListener struct {
    mailer     MailService
    templateID string
}

func NewShippingNotificationListener(
    mailer MailService,
    templateID string,
) *ShippingNotificationListener {
    return &ShippingNotificationListener{
        mailer:     mailer,
        templateID: templateID,
    }
}

func (l *ShippingNotificationListener) Handle(ctx context.Context, e OrderShipped) error {
    return l.mailer.Send(ctx, l.templateID, map[string]string{
        "order_id":    e.OrderID,
        "tracking_no": e.TrackingNo,
    })
}
```

Подписка:

```go
listener := NewShippingNotificationListener(mailer, "shipping-tpl")
sub := events.Subscribe(bus, listener)
```

### Функциональный обработчик

Для простых случаев, не требующих зависимостей:

```go
sub := events.SubscribeFunc(bus, func(ctx context.Context, e PaymentReceived) error {
    slog.InfoContext(ctx, "payment received",
        "payment_id", e.PaymentID,
        "amount", e.Amount,
    )
    return nil
})
```

### Wildcard-обработчик

Получает все события независимо от типа. Удобен для аудита и сквозного логирования:

```go
bus.SubscribeAll(func(ctx context.Context, e events.Event) error {
    slog.InfoContext(ctx, "event occurred",
        "name", e.EventName(),
        "aggregate_id", e.AggregateID(),
        "occurred_at", e.OccurredAt(),
    )
    return nil
})
```

---

## Отписка

Каждая функция подписки возвращает объект `Subscription`:

```go
sub := events.Subscribe(bus, listener)

// Обработчик больше не будет вызываться.
sub.Unsubscribe()
```

---

## Публикация событий

### Одиночная публикация

```go
err := bus.Publish(ctx, OrderShipped{
    BaseEvent:  events.NewBaseEvent("order.shipped", "order-123"),
    OrderID:    "order-123",
    TrackingNo: "TRACK-789",
    ShippedAt:  time.Now(),
})
```

### Batch-публикация

Публикация нескольких событий за один вызов:

```go
err := bus.PublishAll(ctx,
    OrderShipped{
        BaseEvent: events.NewBaseEvent("order.shipped", "order-1"),
        OrderID:   "order-1",
    },
    PaymentReceived{
        BaseEvent: events.NewBaseEvent("payment.received", "order-1"),
        PaymentID: "pay-1",
        OrderID:   "order-1",
        Amount:    99.90,
    },
)
```

---

## Конфигурация

### Создание диспетчера

```go
bus := events.New(
    events.WithAsyncMode(),           // асинхронная обработка (по умолчанию)
    events.WithWorkerCount(8),        // количество воркеров (по умолчанию runtime.NumCPU())
    events.WithBufferSize(100),       // размер буфера канала (по умолчанию workerCount * 10)
    events.WithPublishTimeout(3*time.Second), // таймаут отправки в канал (по умолчанию 5s)
    events.WithOrderedDelivery(),     // упорядоченная доставка по AggregateID
    events.WithMiddleware(mw1, mw2), // глобальные middleware
    events.WithPanicHandler(ph),      // кастомный обработчик паник
    events.WithErrorHandler(eh),      // кастомный обработчик ошибок
    events.WithMetrics(collector),    // сборщик метрик
)
```

### Режимы доставки

**Синхронный** — события обрабатываются немедленно в горутине вызывающего `Publish`. Гарантирует порядок. Воркеры не создаются:

```go
bus := events.New(events.WithSyncMode())
```

**Асинхронный** — события помещаются в буферизованный канал и обрабатываются пулом воркеров:

```go
bus := events.New(events.WithAsyncMode(), events.WithWorkerCount(4))
```

### Упорядоченная доставка

При включённом `WithOrderedDelivery()` события с одинаковым `AggregateID` всегда попадают на один и тот же воркер. Это гарантирует последовательную обработку событий внутри агрегата:

```go
bus := events.New(
    events.WithAsyncMode(),
    events.WithOrderedDelivery(),
    events.WithWorkerCount(8),
)
```

---

## Middleware

Middleware позволяет добавлять сквозную логику вокруг обработки событий. Применяются в порядке добавления: первый — внешний (выполняется первым до обработчика и последним после).

```go
type Middleware func(next HandleFunc) HandleFunc
```

### Пример: логирование

```go
func LoggingMiddleware() events.Middleware {
    return func(next events.HandleFunc) events.HandleFunc {
        return func(ctx context.Context, event events.Event) error {
            slog.InfoContext(ctx, "event handling started",
                "event", event.EventName(),
                "aggregate_id", event.AggregateID(),
            )

            start := time.Now()
            err := next(ctx, event)
            duration := time.Since(start)

            if err != nil {
                slog.ErrorContext(ctx, "event handling failed",
                    "event", event.EventName(),
                    "duration", duration,
                    "error", err,
                )
            } else {
                slog.InfoContext(ctx, "event handling completed",
                    "event", event.EventName(),
                    "duration", duration,
                )
            }

            return err
        }
    }
}
```

### Пример: трассировка

```go
func TracingMiddleware(tracer trace.Tracer) events.Middleware {
    return func(next events.HandleFunc) events.HandleFunc {
        return func(ctx context.Context, event events.Event) error {
            ctx, span := tracer.Start(ctx, "event:"+event.EventName(),
                trace.WithAttributes(
                    attribute.String("event.aggregate_id", event.AggregateID()),
                ),
            )
            defer span.End()

            err := next(ctx, event)
            if err != nil {
                span.RecordError(err)
                span.SetStatus(codes.Error, err.Error())
            }
            return err
        }
    }
}
```

### Применение

```go
bus := events.New(
    events.WithMiddleware(
        LoggingMiddleware(),
        TracingMiddleware(tracer),
    ),
)
```

---

## Retry-политика

Политика повторных попыток задаётся при подписке и применяется индивидуально к каждому обработчику:

```go
events.Subscribe(bus, listener, events.WithRetry(events.RetryPolicy{
    MaxRetries:   5,              // максимальное число повторов
    InitialDelay: 100 * time.Millisecond, // начальная задержка
    MaxDelay:     5 * time.Second,        // максимальная задержка
    Multiplier:   2.0,                    // множитель (exponential backoff)
}))
```

Логика:
1. Обработчик вызывается.
2. Если возвращает ошибку — пауза `InitialDelay`, повторный вызов.
3. Каждая следующая пауза умножается на `Multiplier`, но не превышает `MaxDelay`.
4. После исчерпания попыток ошибка передаётся в `ErrorHandler`.
5. Если контекст отменён — retry прекращается немедленно.

---

## Обработка ошибок и паник

### Ошибки обработчиков

После исчерпания retry-попыток (или при отсутствии retry) ошибка передаётся в `ErrorHandler`:

```go
type ErrorHandler interface {
    Handle(event Event, err error)
}
```

```go
type alertingErrorHandler struct {
    alerter AlertService
}

func (h *alertingErrorHandler) Handle(event events.Event, err error) {
    slog.Error("event handler failed",
        "event", event.EventName(),
        "aggregate_id", event.AggregateID(),
        "error", err,
    )
    h.alerter.Send(fmt.Sprintf("Event %s failed: %v", event.EventName(), err))
}

bus := events.New(events.WithErrorHandler(&alertingErrorHandler{alerter: alerter}))
```

### Паники

Паники в обработчиках перехватываются автоматически и передаются в `PanicHandler`:

```go
type PanicHandler interface {
    Handle(event Event, panicValue any, stack []byte)
}
```

---

## Observability

Интерфейс `MetricsCollector` позволяет собирать метрики обработки:

```go
type MetricsCollector interface {
    EventPublished(eventName string)
    EventHandled(eventName string, duration time.Duration, err error)
    EventDropped(eventName string, reason string)
    QueueDepth(depth int)
}
```

### Пример: интеграция с Prometheus

```go
type prometheusMetrics struct {
    published  *prometheus.CounterVec
    handled    *prometheus.HistogramVec
    errors     *prometheus.CounterVec
    dropped    *prometheus.CounterVec
    queueDepth prometheus.Gauge
}

func (m *prometheusMetrics) EventPublished(name string) {
    m.published.WithLabelValues(name).Inc()
}

func (m *prometheusMetrics) EventHandled(name string, duration time.Duration, err error) {
    m.handled.WithLabelValues(name).Observe(duration.Seconds())
    if err != nil {
        m.errors.WithLabelValues(name).Inc()
    }
}

func (m *prometheusMetrics) EventDropped(name string, reason string) {
    m.dropped.WithLabelValues(name, reason).Inc()
}

func (m *prometheusMetrics) QueueDepth(depth int) {
    m.queueDepth.Set(float64(depth))
}

bus := events.New(events.WithMetrics(&prometheusMetrics{...}))
```

---

## Graceful shutdown

Метод `Close` принимает контекст для ограничения времени ожидания:

```go
// Ожидание до 10 секунд завершения всех обработчиков.
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := bus.Close(ctx); err != nil {
    slog.Error("shutdown timed out, some events may be lost", "error", err)
}
```

При вызове `Close`:
1. Новые вызовы `Publish` возвращают `ErrPublishOnClosedBus`.
2. Каналы закрываются, воркеры дообрабатывают оставшиеся задачи.
3. Если контекст истекает раньше — возвращается `ErrShutdownTimeout`.

---

## Тестирование

Интерфейсы `Publisher`, `Subscriber` и `EventBus` позволяют подменять шину в тестах:

```go
type mockPublisher struct {
    published []events.Event
}

func (m *mockPublisher) Publish(_ context.Context, event events.Event) error {
    m.published = append(m.published, event)
    return nil
}

func (m *mockPublisher) PublishAll(_ context.Context, evts ...events.Event) error {
    m.published = append(m.published, evts...)
    return nil
}

func TestShipOrder(t *testing.T) {
    mock := &mockPublisher{}
    emitter := NewOrderEventEmitter(mock)

    emitter.EmitShipped(context.Background(), "order-1", "TRACK-123")

    if len(mock.published) != 1 {
        t.Fatalf("expected 1 event, got %d", len(mock.published))
    }
    if mock.published[0].EventName() != "order.shipped" {
        t.Fatalf("unexpected event: %s", mock.published[0].EventName())
    }
}
```

Для интеграционных тестов удобно использовать синхронный режим:

```go
func TestIntegration(t *testing.T) {
    bus := events.New(events.WithSyncMode())
    defer bus.Close(context.Background())

    var handled bool
    events.SubscribeFunc(bus, func(ctx context.Context, e OrderShipped) error {
        handled = true
        return nil
    })

    _ = bus.Publish(context.Background(), OrderShipped{
        BaseEvent: events.NewBaseEvent("order.shipped", "order-1"),
        OrderID:   "order-1",
    })

    if !handled {
        t.Fatal("event was not handled")
    }
}
```

---

## Справочник API

### Создание и управление

| Функция / Метод | Описание |
|---|---|
| `events.New(opts ...Option) *Dispatcher` | Создаёт диспетчер с заданными опциями |
| `bus.Close(ctx context.Context) error` | Graceful shutdown с таймаутом через контекст |

### Подписка

| Функция / Метод | Описание |
|---|---|
| `events.Subscribe[T](bus, handler, opts...) Subscription` | Структурный обработчик (`Handler[T]`) |
| `events.SubscribeFunc[T](bus, fn, opts...) Subscription` | Функциональный обработчик |
| `bus.SubscribeAll(handler, opts...) Subscription` | Wildcard — все события |
| `sub.Unsubscribe()` | Отмена подписки |

### Публикация

| Метод | Описание |
|---|---|
| `bus.Publish(ctx, event) error` | Публикация одного события |
| `bus.PublishAll(ctx, events...) error` | Публикация нескольких событий |

### Опции диспетчера

| Опция | Описание | По умолчанию |
|---|---|---|
| `WithAsyncMode()` | Асинхронная обработка через пул воркеров | Включено |
| `WithSyncMode()` | Синхронная обработка в горутине вызывающего | — |
| `WithWorkerCount(n)` | Количество воркеров | `runtime.NumCPU()` |
| `WithBufferSize(n)` | Размер буфера канала событий | `workerCount * 10` |
| `WithPublishTimeout(d)` | Таймаут отправки события в канал | `5s` |
| `WithOrderedDelivery()` | Упорядоченная доставка по `AggregateID` | Выключено |
| `WithMiddleware(mw...)` | Глобальные middleware | — |
| `WithPanicHandler(h)` | Обработчик паник | Логирование в `slog` |
| `WithErrorHandler(h)` | Обработчик ошибок | Логирование в `slog` |
| `WithMetrics(m)` | Сборщик метрик | No-op |

### Опции подписки

| Опция | Описание |
|---|---|
| `WithRetry(RetryPolicy{...})` | Retry с exponential backoff для подписки |

---

## Работа с проектом

### Установка инструментов

```sh
make install-tools
```

### Полная локальная проверка

```sh
make all
```

Выполняет: проверку форматирования, линтинг, security-сканирование, запуск тестов.

### CI

```sh
make ci
```

---

## Лицензия

Распространяется под лицензией [MIT](LICENSE).

---

## Вклад в проект

PR и issue приветствуются. Перед отправкой убедитесь, что `make all` проходит без ошибок.

---

> **Автор**: MSeytumerov
> **Репозиторий**: `github.com/shuldan/events`
> **Go**: `1.24+`
