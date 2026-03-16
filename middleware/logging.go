package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shuldan/events"
)

type Logger interface {
	Info(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

type loggingMiddleware struct {
	logger Logger
}

// NewLogging создаёт middleware для логирования обработки событий.
func NewLogging(logger Logger) events.Middleware {
	return &loggingMiddleware{logger: logger}
}

type loggingNext struct {
	logger Logger
	next   events.Next
}

func (m *loggingMiddleware) Wrap(next events.Next) events.Next {
	return &loggingNext{logger: m.logger, next: next}
}

func (n *loggingNext) Handle(ctx context.Context, event events.Event) error {
	eventType := fmt.Sprintf("%T", event)

	n.logger.Info("handling event",
		"event_type", eventType,
	)

	start := time.Now()
	err := n.next.Handle(ctx, event)
	duration := time.Since(start)

	if err != nil {
		n.logger.Error("event handling failed",
			"event_type", eventType,
			"duration", duration,
			"error", err,
		)
	} else {
		n.logger.Info("event handled",
			"event_type", eventType,
			"duration", duration,
		)
	}

	return err
}

type slogAdapter struct {
	logger *slog.Logger
}

func NewSlogAdapter() Logger {
	return &slogAdapter{}
}

func (a *slogAdapter) Info(msg string, keysAndValues ...any)  { a.logger.Info(msg, keysAndValues...) }
func (a *slogAdapter) Error(msg string, keysAndValues ...any) { a.logger.Error(msg, keysAndValues...) }
