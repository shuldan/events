package events

import (
	"runtime"
	"time"
)

type Option func(*dispatcherOptions)

type dispatcherOptions struct {
	asyncMode      bool
	workerCount    int
	bufferSize     int
	publishTimeout time.Duration
	ordered        bool

	middlewares  []Middleware
	panicHandler PanicHandler
	errorHandler ErrorHandler
	metrics      MetricsCollector
}

func defaultOptions() *dispatcherOptions {
	return &dispatcherOptions{
		asyncMode:      true,
		workerCount:    runtime.NumCPU(),
		bufferSize:     0, // будет вычислен, если не задан
		publishTimeout: 5 * time.Second,
		ordered:        false,
		panicHandler:   &defaultPanicHandler{},
		errorHandler:   &defaultErrorHandler{},
		metrics:        &noopMetrics{},
	}
}

func WithAsyncMode() Option {
	return func(o *dispatcherOptions) { o.asyncMode = true }
}

func WithSyncMode() Option {
	return func(o *dispatcherOptions) { o.asyncMode = false }
}

func WithWorkerCount(n int) Option {
	return func(o *dispatcherOptions) {
		if n < 1 {
			n = 1
		}
		o.workerCount = n
	}
}

func WithBufferSize(size int) Option {
	return func(o *dispatcherOptions) {
		if size < 0 {
			size = 0
		}
		o.bufferSize = size
	}
}

func WithPublishTimeout(d time.Duration) Option {
	return func(o *dispatcherOptions) { o.publishTimeout = d }
}

// WithOrderedDelivery гарантирует, что события одного агрегата
// обрабатываются последовательно (роутинг по AggregateID).
func WithOrderedDelivery() Option {
	return func(o *dispatcherOptions) { o.ordered = true }
}

func WithMiddleware(mw ...Middleware) Option {
	return func(o *dispatcherOptions) { o.middlewares = append(o.middlewares, mw...) }
}

func WithPanicHandler(h PanicHandler) Option {
	return func(o *dispatcherOptions) { o.panicHandler = h }
}

func WithErrorHandler(h ErrorHandler) Option {
	return func(o *dispatcherOptions) { o.errorHandler = h }
}

func WithMetrics(m MetricsCollector) Option {
	return func(o *dispatcherOptions) { o.metrics = m }
}
