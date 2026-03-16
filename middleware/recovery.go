package middleware

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/shuldan/events"
)

type recoveryMiddleware struct{}

func NewRecovery() events.Middleware {
	return &recoveryMiddleware{}
}

type recoveryNext struct {
	next events.Next
}

func (m *recoveryMiddleware) Wrap(next events.Next) events.Next {
	return &recoveryNext{next: next}
}

func (n *recoveryNext) Handle(ctx context.Context, event events.Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("events: panic recovered: %v\n%s", r, debug.Stack())
		}
	}()
	return n.next.Handle(ctx, event)
}
