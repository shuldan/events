package events

import (
	"context"
	"reflect"
)

type listener struct {
	listenerFunc func(context.Context, any) error
	eventType    reflect.Type
}

func (l *listener) handleEvent(ctx context.Context, event any) error {
	eventValue := reflect.ValueOf(event)
	if !eventValue.Type().AssignableTo(l.eventType) {
		return ErrInvalidEventType
	}

	return l.listenerFunc(ctx, event)
}

func adapterFromFunction(fn any) (*listener, error) {
	fnType := reflect.TypeOf(fn)
	if fnType.NumIn() != 2 || fnType.NumOut() != 1 {
		return nil, ErrInvalidListenerFunction
	}

	ctxType, eventType := fnType.In(0), fnType.In(1)
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()

	if !ctxType.Implements(contextType) {
		return nil, ErrInvalidListenerFunction
	}
	if fnType.Out(0) != errorType {
		return nil, ErrInvalidListenerFunction
	}

	return &listener{
		eventType: eventType,
		listenerFunc: func(ctx context.Context, event any) error {
			results := reflect.ValueOf(fn).Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(event),
			})
			if err := results[0].Interface(); err != nil {
				return err.(error)
			}
			return nil
		},
	}, nil
}

func listenerFromMethod(receiver reflect.Value, method reflect.Method) (*listener, error) {
	fnType := method.Type
	if fnType.NumIn() != 3 {
		return nil, ErrInvalidListenerMethod
	}

	ctxType := fnType.In(1)
	eventType := fnType.In(2)
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()

	if !ctxType.Implements(contextType) {
		return nil, ErrInvalidListenerMethod
	}
	if fnType.NumOut() != 1 || fnType.Out(0) != errorType {
		return nil, ErrInvalidListenerMethod
	}

	return &listener{
		eventType: eventType,
		listenerFunc: func(ctx context.Context, event any) error {
			results := method.Func.Call([]reflect.Value{
				receiver,
				reflect.ValueOf(ctx),
				reflect.ValueOf(event),
			})
			if err := results[0].Interface(); err != nil {
				return err.(error)
			}
			return nil
		},
	}, nil
}

func newListener(listener any) (*listener, error) {
	listenerVal := reflect.ValueOf(listener)
	if !listenerVal.IsValid() {
		return nil, ErrInvalidListener
	}

	if listenerVal.Kind() == reflect.Func {
		return adapterFromFunction(listener)
	}

	if method, ok := listenerVal.Type().MethodByName("Handle"); ok {
		return listenerFromMethod(listenerVal, method)
	}

	return nil, ErrInvalidListener
}
