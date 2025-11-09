package events

import (
	"context"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

type eventTask struct {
	ctx      context.Context
	event    any
	listener *listener
}

type Dispatcher struct {
	mu        sync.RWMutex
	listeners map[reflect.Type][]*listener
	closed    bool
	wg        sync.WaitGroup
	eventChan chan eventTask
	opts      *options
}

func New(opts ...Option) *Dispatcher {
	dispatcher := &Dispatcher{
		listeners: make(map[reflect.Type][]*listener),
		opts: &options{
			panicHandler: newDefaultPanicHandler(),
			errorHandler: newDefaultErrorHandler(),
			asyncMode:    true,
			workerCount:  runtime.NumCPU(),
		},
	}

	for _, opt := range opts {
		opt(dispatcher.opts)
	}

	dispatcher.startWorkers()

	return dispatcher
}

func (d *Dispatcher) Subscribe(eventTypeArg any, listener any) error {
	eventTypeOf := reflect.TypeOf(eventTypeArg)
	if eventTypeOf == nil {
		return ErrInvalidEventType
	}
	if eventTypeOf.Kind() != reflect.Ptr || eventTypeOf.Elem().Kind() != reflect.Struct {
		return ErrInvalidEventType
	}
	eventType := eventTypeOf.Elem()

	l, err := newListener(listener)
	if err != nil {
		return err
	}

	if l.eventType != eventType {
		return ErrInvalidListener
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return ErrBusClosed
	}

	d.listeners[eventType] = append(d.listeners[eventType], l)
	return nil
}

func (d *Dispatcher) Publish(ctx context.Context, event any) error {
	if event == nil {
		return nil
	}

	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return ErrPublishOnClosedBus
	}

	eventType := reflect.TypeOf(event)
	listeners, ok := d.listeners[eventType]
	d.mu.RUnlock()

	if !ok || len(listeners) == 0 {
		return nil
	}

	if d.opts.asyncMode {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, l := range listeners {
			select {
			case d.eventChan <- eventTask{ctx: ctx, event: event, listener: l}:
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return ErrEventChannelBlocked
			}
		}
	} else {
		for _, l := range listeners {
			d.processEvent(eventTask{ctx: ctx, event: event, listener: l})
		}
	}

	return nil
}

func (d *Dispatcher) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	d.mu.Unlock()

	if d.eventChan != nil {
		close(d.eventChan)
	}

	d.wg.Wait()
	return nil
}

func (d *Dispatcher) startWorkers() {
	d.eventChan = make(chan eventTask, d.opts.workerCount*10)
	for i := 0; i < d.opts.workerCount; i++ {
		d.wg.Add(1)
		go d.worker()
	}
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for task := range d.eventChan {
		d.processEvent(task)
	}
}

func (d *Dispatcher) processEvent(task eventTask) {
	defer func() {
		if r := recover(); r != nil {
			d.opts.panicHandler.Handle(task.event, task.listener, r, debug.Stack())
		}
	}()

	if err := task.listener.handleEvent(task.ctx, task.event); err != nil {
		d.opts.errorHandler.Handle(task.event, task.listener, err)
	}
}
