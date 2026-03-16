package events

import "time"

// SubscribeOption — опция подписки.
type SubscribeOption func(*subscribeConfig)

type subscribeConfig struct {
	retry      *RetryPolicy
	timeout    time.Duration
	middleware []Middleware
}

func defaultSubscribeConfig() subscribeConfig {
	return subscribeConfig{}
}

// WithRetry задаёт политику повторных попыток.
func WithRetry(policy RetryPolicy) SubscribeOption {
	return func(c *subscribeConfig) {
		c.retry = &policy
	}
}

// WithTimeout задаёт таймаут обработки события.
func WithTimeout(d time.Duration) SubscribeOption {
	return func(c *subscribeConfig) {
		c.timeout = d
	}
}

// WithSubscribeMiddleware добавляет middleware для конкретной подписки.
func WithSubscribeMiddleware(mw ...Middleware) SubscribeOption {
	return func(c *subscribeConfig) {
		c.middleware = append(c.middleware, mw...)
	}
}
