package events

import "context"

// Next — следующий элемент в цепочке обработки.
type Next interface {
	Handle(ctx context.Context, event Event) error
}

// Middleware оборачивает следующий элемент цепочки.
type Middleware interface {
	Wrap(next Next) Next
}

// buildChain собирает цепочку middleware в единый Next.
// Порядок: middlewares[0] → middlewares[1] → ... → final.
func buildChain(middlewares []Middleware, final Next) Next {
	chain := final
	for i := len(middlewares) - 1; i >= 0; i-- {
		chain = middlewares[i].Wrap(chain)
	}
	return chain
}
