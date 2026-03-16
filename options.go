package events

import "context"

// Option — опция конфигурации Dispatcher.
type Option func(*dispatcherConfig)

type dispatcherConfig struct {
	async          bool
	workerPoolSize int
	errorHandler   ErrorHandler
	middleware     []Middleware
	transport      Transport
	codec          Codec
	defaultSubOpts []SubscribeOption
}

// ErrorHandler вызывается при ошибке обработки (после исчерпания retry).
type ErrorHandler func(ctx context.Context, event Event, err error)

func defaultConfig() dispatcherConfig {
	return dispatcherConfig{
		workerPoolSize: 1,
	}
}

// WithAsyncMode включает асинхронную обработку событий.
func WithAsyncMode() Option {
	return func(c *dispatcherConfig) {
		c.async = true
	}
}

// WithWorkerPool задаёт размер пула воркеров (только для async-режима).
func WithWorkerPool(size int) Option {
	return func(c *dispatcherConfig) {
		if size > 0 {
			c.workerPoolSize = size
		}
	}
}

// WithErrorHandler задаёт обработчик ошибок.
func WithErrorHandler(fn ErrorHandler) Option {
	return func(c *dispatcherConfig) {
		c.errorHandler = fn
	}
}

// WithMiddleware добавляет глобальный middleware.
func WithMiddleware(mw ...Middleware) Option {
	return func(c *dispatcherConfig) {
		c.middleware = append(c.middleware, mw...)
	}
}

// WithTransport задаёт внешний транспорт.
func WithTransport(t Transport) Option {
	return func(c *dispatcherConfig) {
		c.transport = t
	}
}

// WithCodec задаёт кодек сериализации.
func WithCodec(codec Codec) Option {
	return func(c *dispatcherConfig) {
		c.codec = codec
	}
}

// WithDefaultSubscribeOptions задаёт дефолтные опции для подписчиков.
func WithDefaultSubscribeOptions(opts ...SubscribeOption) Option {
	return func(c *dispatcherConfig) {
		c.defaultSubOpts = append(c.defaultSubOpts, opts...)
	}
}
