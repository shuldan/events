package events

type PanicHandler interface {
	Handle(event any, listener any, panicValue any, stack []byte)
}

type ErrorHandler interface {
	Handle(event any, listener any, err error)
}

type Option func(*options)

type options struct {
	panicHandler PanicHandler
	errorHandler ErrorHandler
	asyncMode    bool
	workerCount  int
}

func WithPanicHandler(h PanicHandler) Option {
	return func(o *options) {
		o.panicHandler = h
	}
}

func WithErrorHandler(h ErrorHandler) Option {
	return func(o *options) {
		o.errorHandler = h
	}
}

func WithAsyncMode() Option {
	return func(o *options) {
		o.asyncMode = true
	}
}

func WithSyncMode() Option {
	return func(o *options) {
		o.asyncMode = false
	}
}

func WithWorkerCount(count int) Option {
	return func(o *options) {
		if count < 1 {
			count = 1
		}
		o.workerCount = count
	}
}
