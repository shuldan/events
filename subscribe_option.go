package events

import "time"

type SubscribeOption func(*subscribeOptions)

type subscribeOptions struct {
	retry *RetryPolicy
}

func defaultSubscribeOptions() *subscribeOptions {
	return &subscribeOptions{}
}

type RetryPolicy struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func WithRetry(policy RetryPolicy) SubscribeOption {
	return func(o *subscribeOptions) {
		if policy.Multiplier <= 0 {
			policy.Multiplier = 2.0
		}
		if policy.MaxRetries < 0 {
			policy.MaxRetries = 0
		}
		o.retry = &policy
	}
}
