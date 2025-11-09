package events

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type TestEventHandler struct{}

func (h TestEventHandler) Handle(ctx context.Context, event TestEvent) error {
	return nil
}

type TestEventHandlerWithError struct{}

func (h TestEventHandlerWithError) Handle(ctx context.Context, event TestEvent) error {
	return errors.New("handler error")
}

type handlerMissingContext struct{}

func (h handlerMissingContext) Handle(event TestEvent) error {
	return nil
}

type handlerNoErrorReturn struct{}

func (h handlerNoErrorReturn) Handle(ctx context.Context, event TestEvent) {}

type handlerWrongContextType struct{}

func (h handlerWrongContextType) Handle(s string, event TestEvent) error {
	return nil
}

func TestNewListener_ValidFunction(t *testing.T) {
	t.Parallel()
	fn := func(ctx context.Context, event TestEvent) error {
		return nil
	}

	l, err := newListener(fn)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil listener")
	}
	if l.eventType != reflect.TypeOf(TestEvent{}) {
		t.Errorf("expected event type TestEvent, got %v", l.eventType)
	}
}

func TestNewListener_ValidMethod(t *testing.T) {
	t.Parallel()
	handler := TestEventHandler{}

	l, err := newListener(handler)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil listener")
	}
}

func TestNewListener_InvalidListener(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		listener any
	}{
		{"string", "invalid"},
		{"int", 42},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newListener(tt.listener)
			if err == nil {
				t.Error("expected error for invalid listener")
			}
		})
	}
}

func TestAdapterFromFunction_InvalidSignatures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		fn   any
	}{
		{"no_params", func() {}},
		{"one_param", func(ctx context.Context) {}},
		{"three_params", func(ctx context.Context, e TestEvent, x int) error { return nil }},
		{"no_return", func(ctx context.Context, e TestEvent) {}},
		{"two_returns", func(ctx context.Context, e TestEvent) (error, error) { return nil, nil }},
		{"wrong_first_param", func(s string, e TestEvent) error { return nil }},
		{"wrong_return_type", func(ctx context.Context, e TestEvent) int { return 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := adapterFromFunction(tt.fn)
			if !errors.Is(err, ErrInvalidListenerFunction) {
				t.Errorf("expected ErrInvalidListenerFunction, got %v", err)
			}
		})
	}
}

func TestListenerFromMethod_InvalidSignatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler any
	}{
		{"no_context", handlerMissingContext{}},
		{"no_error_return", handlerNoErrorReturn{}},
		{"wrong_context_type", handlerWrongContextType{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newListener(tt.handler)
			if err == nil {
				t.Error("expected error for invalid handler method")
			}
		})
	}
}

func TestListener_HandleEvent_Success(t *testing.T) {
	t.Parallel()
	fn := func(ctx context.Context, event TestEvent) error {
		return nil
	}

	l, _ := newListener(fn)
	err := l.handleEvent(context.Background(), TestEvent{Value: "test"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListener_HandleEvent_WithError(t *testing.T) {
	t.Parallel()
	testErr := errors.New("handler error")
	fn := func(ctx context.Context, event TestEvent) error {
		return testErr
	}

	l, _ := newListener(fn)
	err := l.handleEvent(context.Background(), TestEvent{})
	if err != testErr {
		t.Errorf("expected %v, got %v", testErr, err)
	}
}

func TestListener_HandleEvent_MethodSuccess(t *testing.T) {
	t.Parallel()
	handler := TestEventHandler{}
	l, _ := newListener(handler)

	err := l.handleEvent(context.Background(), TestEvent{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestListener_HandleEvent_MethodWithError(t *testing.T) {
	t.Parallel()
	handler := TestEventHandlerWithError{}
	l, _ := newListener(handler)

	err := l.handleEvent(context.Background(), TestEvent{})
	if err == nil {
		t.Error("expected error from handler method")
	}
}

func TestListener_HandleEvent_ReturnNilError(t *testing.T) {
	t.Parallel()
	fn := func(ctx context.Context, event TestEvent) error {
		return nil
	}

	l, _ := newListener(fn)
	err := l.handleEvent(context.Background(), TestEvent{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
