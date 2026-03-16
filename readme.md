# `events` — Типобезопасный диспетчер доменных событий для Go

[![Go CI](https://github.com/shuldan/events/workflows/Go%20CI/badge.svg)](https://github.com/shuldan/events/actions)
[![codecov](https://codecov.io/gh/shuldan/events/branch/main/graph/badge.svg)](https://codecov.io/gh/shuldan/events)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Пакет `events` предоставляет высокопроизводительную шину доменных событий для Go-приложений, построенных по принципам DDD. Использует дженерики для типобезопасности на этапе компиляции, поддерживает синхронную и асинхронную доставку, структурные middleware, retry-политики, упорядоченную обработку по ключу и абстрактный транспорт для кросс-сервисного взаимодействия.

---

## Основные возможности

- **Типобезопасность через дженерики** — ошибки несоответствия типов обнаруживаются на этапе компиляции.
- **Структурные обработчики** — слушателем является структура, реализующая интерфейс `Handler[E]`.
- **Структурные middleware** — цепочки сквозной логики через интерфейс `Middleware` с методом `Wrap`.
- **Retry с exponential backoff** — настраиваемая политика повторных попыток глобально и для каждой подписки.
- **Упорядоченная доставка по ключу** — события с одинаковым `EventKey()` обрабатываются последовательно.
- **Batch-публикация** — отправка нескольких событий за один вызов `PublishAll`.
- **Отписка** — каждая подписка возвращает объект `Subscription` для управления жизненным циклом.
- **Graceful shutdown** — корректное завершение с поддержкой таймаута через контекст.
- **Абстрактный транспорт** — интерфейс `Transport` для кросс-сервисной доставки (Kafka, NATS, RabbitMQ).
- **Абстрактный кодек** — интерфейс `Codec` для сериализации событий (JSON, Protobuf).
- **Нулевые внешние зависимости в ядре** — корневой пакет зависит только от стандартной библиотеки.

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
    OrderID string
    UserID  string
    Amount  float64
}

// Определяем обработчик.
type OrderCreatedListener struct{}

func (l *OrderCreatedListener) Handle(ctx context.Context, e OrderCreated) error {
    fmt.Printf("Order %s created for user %s, amount: %.2f\n",
        e.OrderID, e.UserID, e.Amount)
    return nil
}

func main() {
    // Создаём диспетчер в синхронном режиме (по умолчанию).
    d := events.New()
    defer d.Close(context.Background())

    // Подписываемся.
    listener := &OrderCreatedListener{}
    events.Subscribe(d, listener)

    // Публикуем событие.
    err := d.Publish(context.Background(), OrderCreated{
        OrderID: "order-1",
        UserID:  "user-42",
        Amount:  199.90,
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

---

## Определение событий

Событием может быть любая структура — специальный интерфейс реализовывать не нужно.

```go
type OrderShipped struct {
    OrderID    string
    TrackingNo string
    ShippedAt  time.Time
}

type PaymentReceived struct {
    PaymentID string
    OrderID   string
    Amount    float64
    Currency  string
}
```

### Упорядоченные события

Для гарантии порядка обработки реализуйте интерфейс `KeyedEvent`:

```go
type KeyedEvent interface {
    EventKey() string
}
```

```go
type OrderStatusChanged struct {
    OrderID   string
    NewStatus string
}

// События одного заказа обрабатываются последовательно.
func (e OrderStatusChanged) EventKey() string { return e.OrderID }
```

---

## Обработчики событий

Обработчик — это структура, реализующая интерфейс `Handler[E]`:

```go
type Handler[E Event] interface {
    Handle(ctx context.Context, event E) error
}
```

### Пример

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
sub := events.Subscribe(d, listener)
```

---

## Отписка

Каждая функция подписки возвращает объект `Subscription`:

```go
sub := events.Subscribe(d, listener)

// Обработчик больше не будет вызываться.
sub.Unsubscribe()
```

---

## Публикация событий

### Одиночная публикация

```go
err := d.Publish(ctx, OrderShipped{
    OrderID:    "order-123",
    TrackingNo: "TRACK-789",
    ShippedAt:  time.Now(),
})
```

### Batch-публикация

```go
err := d.PublishAll(ctx,
    OrderShipped{OrderID: "order-1", TrackingNo: "TRACK-001"},
    PaymentReceived{PaymentID: "pay-1", OrderID: "order-1", Amount: 99.90},
)
```

### Поведение при ошибках

Обработчики независимы друг от друга. Событие доставляется всем подписчикам, даже если один из них вернул ошибку:

| Сценарий | Sync | Async |
|---|---|---|
| Handler A — ok, Handler B — error | Оба вызваны, `errors.Join(nil, errB)` | Оба вызваны, `ErrorHandler(errB)` |
| Handler A — error, Handler B — ok | Оба вызваны, `errors.Join(errA, nil)` | Оба вызваны, `ErrorHandler(errA)` |
| Retry исчерпан | Ошибка в `errors.Join` | `ErrorHandler` |
| Dispatcher закрыт | `ErrDispatcherClosed` | `ErrDispatcherClosed` |

---

## Конфигурация

### Создание диспетчера

```go
d := events.New(
    events.WithAsyncMode(),
    events.WithWorkerPool(8),
    events.WithErrorHandler(func(ctx context.Context, event events.Event, err error) {
        slog.Error("event handling failed", "error", err)
    }),
    events.WithMiddleware(
        middleware.NewLogging(slog.Default()),
    ),
    events.WithCodec(codec.NewJSON()),
    events.WithTransport(kafka.New(kafka.Config{
        Brokers: []string{"localhost:9092"},
        Topic:   "domain-events",
    })),
    events.WithDefaultSubscribeOptions(
        events.WithRetry(events.RetryPolicy{
            MaxRetries:   3,
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     2 * time.Second,
            Multiplier:   2.0,
        }),
        events.WithTimeout(30 * time.Second),
    ),
)
defer d.Close(context.Background())
```

### Режимы доставки

**Синхронный** (по умолчанию) — события обрабатываются в горутине вызывающего `Publish`:

```go
d := events.New()
```

**Асинхронный** — события обрабатываются пулом воркеров:

```go
d := events.New(events.WithAsyncMode(), events.WithWorkerPool(4))
```

### Упорядоченная доставка по ключу

События, реализующие `KeyedEvent`, с одинаковым ключом всегда обрабатываются последовательно. Разные ключи обрабатываются параллельно:

```go
type OrderStatusChanged struct {
    OrderID   string
    NewStatus string
}

func (e OrderStatusChanged) EventKey() string { return e.OrderID }

// Все события заказа "order-123" обрабатываются последовательно,
// события "order-456" — параллельно с ними.
```

---

## Middleware

Middleware реализуется через структурный интерфейс:

```go
type Next interface {
    Handle(ctx context.Context, event Event) error
}

type Middleware interface {
    Wrap(next Next) Next
}
```

### Порядок применения

```
Global Middleware 1
  → Global Middleware 2
    → Subscribe Middleware 1
      → [Timeout]
        → [Retry]
          → Handler.Handle()
```

### Пример: логирование

```go
type loggingMiddleware struct {
    logger *slog.Logger
}

func NewLogging(logger *slog.Logger) events.Middleware {
    return &loggingMiddleware{logger: logger}
}

func (m *loggingMiddleware) Wrap(next events.Next) events.Next {
    return &loggingNext{logger: m.logger, next: next}
}

type loggingNext struct {
    logger *slog.Logger
    next   events.Next
}

func (n *loggingNext) Handle(ctx context.Context, event events.Event) error {
    n.logger.Info("handling event", "type", fmt.Sprintf("%T", event))
    start := time.Now()

    err := n.next.Handle(ctx, event)

    if err != nil {
        n.logger.Error("event failed", "duration", time.Since(start), "error", err)
    } else {
        n.logger.Info("event handled", "duration", time.Since(start))
    }
    return err
}
```

### Готовые middleware

Пакет `events/middleware` предоставляет готовые реализации:

```go
import "github.com/shuldan/events/middleware"

d := events.New(
    events.WithMiddleware(
        middleware.NewLogging(slog.Default()),
        middleware.NewMetrics(recorder),
    ),
)

// Recovery не включён по умолчанию — подключается осознанно:
events.Subscribe(d, handler,
    events.WithSubscribeMiddleware(middleware.NewRecovery()),
)
```

### Применение

Middleware можно задать глобально и на уровне подписки. Локальные дополняют глобальные:

```go
d := events.New(
    events.WithMiddleware(globalMw),
)

events.Subscribe(d, handler,
    events.WithSubscribeMiddleware(localMw),
)
```

---

## Retry-политика

Задаётся глобально или на уровне подписки. Локальные переопределяют глобальные:

```go
// Глобальный дефолт.
d := events.New(
    events.WithDefaultSubscribeOptions(
        events.WithRetry(events.RetryPolicy{
            MaxRetries:   3,
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     2 * time.Second,
            Multiplier:   2.0,
        }),
    ),
)

// Переопределение для критичного обработчика.
events.Subscribe(d, paymentHandler,
    events.WithRetry(events.RetryPolicy{
        MaxRetries:   10,
        InitialDelay: 500 * time.Millisecond,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
    }),
    events.WithTimeout(60 * time.Second),
)
```

Логика:
1. Обработчик вызывается.
2. Если возвращает ошибку — пауза `InitialDelay`, повторный вызов.
3. Каждая следующая пауза умножается на `Multiplier`, но не превышает `MaxDelay`.
4. После исчерпания попыток ошибка передаётся в `ErrorHandler`.
5. Если контекст отменён — retry прекращается немедленно.

---

## Транспорт

Интерфейс `Transport` позволяет доставлять события между сервисами:

```go
type Transport interface {
    Publish(ctx context.Context, envelope Envelope) error
    Subscribe(ctx context.Context, handler TransportHandler) error
    Close(ctx context.Context) error
}

type TransportHandler interface {
    Handle(ctx context.Context, envelope Envelope) error
}
```

### Поток данных

```
Service A:
  Publish(Event)
      │
      ├──► local handlers
      │
      ├──► Codec.Encode(Event) → []byte
      │        │
      └────────┴──► Transport.Publish(Envelope) ──► Kafka / NATS / RabbitMQ

Service B:
  Kafka ──► Transport.Subscribe() → Envelope
                │
                ▼
            Codec.Decode([]byte) → Event
                │
                ▼
            local handlers
```

### Кодек

Интерфейс `Codec` отделяет сериализацию от транспорта:

```go
type Codec interface {
    Encode(event Event) ([]byte, error)
    Decode(data []byte, target Event) error
    ContentType() string
}
```

Готовая реализация:

```go
import "github.com/shuldan/events/codec"

d := events.New(
    events.WithCodec(codec.NewJSON()),
)
```

### Готовые транспорты

```go
import "github.com/shuldan/events/transport/memory"

// In-memory транспорт для тестов.
d := events.New(
    events.WithTransport(memory.New()),
    events.WithCodec(codec.NewJSON()),
)
```

---

## Graceful shutdown

Метод `Close` принимает контекст для ограничения времени ожидания:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := d.Close(ctx); err != nil {
    slog.Error("shutdown timed out", "error", err)
}
```

При вызове `Close`:
1. Новые вызовы `Publish` возвращают `ErrDispatcherClosed`.
2. В async-режиме дожидается завершения всех in-flight обработчиков.
3. Останавливает приём из транспорта.
4. Закрывает транспорт.
5. Если контекст истекает раньше — возвращается ошибка контекста.

---

## Структура пакета

```
events/
├── dispatcher.go          # Dispatcher, New, Publish, PublishAll, Close
├── handler.go             # Handler[E], HandleFunc
├── subscribe.go           # Subscribe[E], Subscription
├── middleware.go           # Next, Middleware, buildChain
├── transport.go           # Transport, TransportHandler, Envelope
├── codec.go               # Codec
├── event.go               # Event, KeyedEvent
├── options.go             # Option (глобальные опции Dispatcher)
├── subscribe_options.go   # SubscribeOption (опции подписки)
├── retry.go               # RetryPolicy
├── errors.go              # ErrDispatcherClosed, ErrNilHandler
│
├── middleware/             # Готовые middleware
│   ├── logging.go         # NewLogging
│   ├── recovery.go        # NewRecovery
│   └── metrics.go         # NewMetrics
│
├── codec/                 # Готовые кодеки
│   └── json.go            # NewJSON
│
└── transport/             # Готовые транспорты
    └── memory/
        └── memory.go      # New (in-memory, для тестов)
```

Тяжёлые зависимости (kafka-client, nats-client, prometheus) изолированы в подпакетах. Корневой пакет — нулевые внешние зависимости.

---

## Тестирование

Для тестов удобно использовать синхронный режим:

```go
func TestOrderCreatedHandling(t *testing.T) {
    d := events.New()
    defer d.Close(context.Background())

    var handled bool
    h := &testHandler{onHandle: func() { handled = true }}
    events.Subscribe(d, h)

    d.Publish(context.Background(), OrderCreated{OrderID: "order-1"})

    if !handled {
        t.Fatal("event was not handled")
    }
}
```

In-memory транспорт для интеграционных тестов:

```go
import "github.com/shuldan/events/transport/memory"

func TestCrossServiceDelivery(t *testing.T) {
    tr := memory.New()
    d := events.New(
        events.WithTransport(tr),
        events.WithCodec(codec.NewJSON()),
    )
    defer d.Close(context.Background())
    // ...
}
```

---

## Справочник API

### Создание и управление

| Функция / Метод | Описание |
|---|---|
| `events.New(opts ...Option) *Dispatcher` | Создаёт диспетчер |
| `d.Publish(ctx, event) error` | Публикация одного события |
| `d.PublishAll(ctx, events...) error` | Публикация нескольких событий |
| `d.Close(ctx) error` | Graceful shutdown |

### Подписка

| Функция / Метод | Описание |
|---|---|
| `events.Subscribe[E](d, handler, opts...) Subscription` | Типизированная подписка |
| `sub.Unsubscribe()` | Отмена подписки |

### Опции диспетчера

| Опция | Описание | По умолчанию |
|---|---|---|
| `WithAsyncMode()` | Асинхронная обработка | Выключено (sync) |
| `WithWorkerPool(n)` | Количество воркеров | `1` |
| `WithErrorHandler(fn)` | Обработчик ошибок | `nil` |
| `WithMiddleware(mw...)` | Глобальные middleware | — |
| `WithTransport(t)` | Внешний транспорт | `nil` |
| `WithCodec(c)` | Кодек сериализации | `nil` |
| `WithDefaultSubscribeOptions(opts...)` | Дефолтные опции подписок | — |

### Опции подписки

| Опция | Описание |
|---|---|
| `WithRetry(RetryPolicy{...})` | Retry с exponential backoff |
| `WithTimeout(d)` | Таймаут обработки события |
| `WithSubscribeMiddleware(mw...)` | Middleware для конкретной подписки |

### Интерфейсы

| Интерфейс | Метод | Назначение |
|---|---|---|
| `Handler[E]` | `Handle(ctx, E) error` | Обработчик события |
| `Middleware` | `Wrap(Next) Next` | Обёртка обработки |
| `Next` | `Handle(ctx, Event) error` | Следующий элемент цепочки |
| `Transport` | `Publish / Subscribe / Close` | Внешний транспорт |
| `TransportHandler` | `Handle(ctx, Envelope) error` | Обработчик входящих сообщений |
| `Codec` | `Encode / Decode / ContentType` | Сериализация |
| `KeyedEvent` | `EventKey() string` | Ключ для упорядоченной доставки |

---

## Лицензия

Распространяется под лицензией [MIT](LICENSE).

---

## Вклад в проект

PR и issue приветствуются.

---

> **Автор**: MSeytumerov
> **Репозиторий**: `github.com/shuldan/events`
> **Go**: `1.24+`
