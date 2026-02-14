package events

import (
	"context"
	"hash/fnv"
	"reflect"
	"runtime/debug"
	"sync"
	"time"
)

type eventTask struct {
	ctx      context.Context
	event    Event
	listener *listener
}

type Dispatcher struct {
	mu              sync.RWMutex
	listeners       map[reflect.Type][]*listener
	globalListeners []*listener
	closed          bool
	wg              sync.WaitGroup
	opts            *dispatcherOptions
	sharedChan      chan eventTask
	workerChans     []chan eventTask
}

func New(opts ...Option) *Dispatcher {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.bufferSize == 0 {
		o.bufferSize = o.workerCount * 10
	}

	d := &Dispatcher{
		listeners: make(map[reflect.Type][]*listener),
		opts:      o,
	}

	if o.asyncMode {
		d.startWorkers()
	}

	return d
}

func (d *Dispatcher) Publish(ctx context.Context, event Event) error {
	if event == nil {
		return nil
	}

	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return ErrPublishOnClosedBus
	}

	eventType := normalizeType(reflect.TypeOf(event))
	targeted := d.listeners[eventType]
	global := d.globalListeners
	all := make([]*listener, 0, len(targeted)+len(global))
	all = append(all, targeted...)
	all = append(all, global...)
	d.mu.RUnlock()

	if len(all) == 0 {
		return nil
	}

	d.opts.metrics.EventPublished(event.EventName())

	if !d.opts.asyncMode {
		for _, l := range all {
			d.processEvent(eventTask{ctx: ctx, event: event, listener: l})
		}
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	for _, l := range all {
		task := eventTask{ctx: ctx, event: event, listener: l}
		if err := d.enqueue(ctx, task, event); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) PublishAll(ctx context.Context, events ...Event) error {
	for _, event := range events {
		if err := d.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) Close(ctx context.Context) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	d.mu.Unlock()

	if d.opts.asyncMode {
		if d.opts.ordered {
			for _, ch := range d.workerChans {
				close(ch)
			}
		} else {
			close(d.sharedChan)
		}
	}

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ErrShutdownTimeout
	}
}

func (d *Dispatcher) addListener(eventType reflect.Type, l *listener) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listeners[eventType] = append(d.listeners[eventType], l)
}

func (d *Dispatcher) addGlobalListener(l *listener) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.globalListeners = append(d.globalListeners, l)
}

func (d *Dispatcher) removeListener(target *listener) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if target.eventType != nil {
		d.listeners[target.eventType] = removeFromSlice(d.listeners[target.eventType], target)
	} else {
		d.globalListeners = removeFromSlice(d.globalListeners, target)
	}
}

func removeFromSlice(slice []*listener, target *listener) []*listener {
	result := make([]*listener, 0, len(slice))
	for _, l := range slice {
		if l.id != target.id {
			result = append(result, l)
		}
	}
	return result
}

func (d *Dispatcher) startWorkers() {
	if d.opts.ordered {
		d.workerChans = make([]chan eventTask, d.opts.workerCount)
		for i := 0; i < d.opts.workerCount; i++ {
			d.workerChans[i] = make(chan eventTask, d.opts.bufferSize)
			d.wg.Add(1)
			go d.worker(d.workerChans[i])
		}
	} else {
		d.sharedChan = make(chan eventTask, d.opts.bufferSize)
		for i := 0; i < d.opts.workerCount; i++ {
			d.wg.Add(1)
			go d.worker(d.sharedChan)
		}
	}
}

func (d *Dispatcher) worker(ch <-chan eventTask) {
	defer d.wg.Done()
	for task := range ch {
		d.processEvent(task)
	}
}

func (d *Dispatcher) enqueue(ctx context.Context, task eventTask, event Event) error {
	var ch chan eventTask

	if d.opts.ordered {
		idx := d.partitionIndex(event.AggregateID())
		ch = d.workerChans[idx]
	} else {
		ch = d.sharedChan
	}

	d.opts.metrics.QueueDepth(len(ch))

	select {
	case ch <- task:
		return nil
	case <-ctx.Done():
		d.opts.metrics.EventDropped(event.EventName(), "context_cancelled")
		return ctx.Err()
	case <-time.After(d.opts.publishTimeout):
		d.opts.metrics.EventDropped(event.EventName(), "timeout")
		return ErrEventChannelBlocked
	}
}

func (d *Dispatcher) partitionIndex(key string) int {
	if key == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32()) % d.opts.workerCount
}

func (d *Dispatcher) processEvent(task eventTask) {
	defer func() {
		if r := recover(); r != nil {
			d.opts.panicHandler.Handle(task.event, r, debug.Stack())
		}
	}()

	start := time.Now()
	err := task.listener.execute(task.ctx, task.event)
	duration := time.Since(start)

	d.opts.metrics.EventHandled(task.event.EventName(), duration, err)

	if err != nil {
		d.opts.errorHandler.Handle(task.event, err)
	}
}

func normalizeType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}
