package events

import (
	"errors"
	"testing"
)

func TestDefaultPanicHandler_Handle(t *testing.T) {
	t.Parallel()
	handler := newDefaultPanicHandler()
	if handler == nil {
		t.Fatal("expected non-nil panic handler")
	}

	handler.Handle(
		TestEvent{Value: "test"},
		"listener",
		"panic value",
		[]byte("stack trace"),
	)
}

func TestDefaultPanicHandler_HandleWithNilValues(t *testing.T) {
	t.Parallel()
	handler := newDefaultPanicHandler()

	handler.Handle(nil, nil, nil, nil)
}

func TestDefaultPanicHandler_HandleWithVariousTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		event      any
		listener   any
		panicValue any
		stack      []byte
	}{
		{"all_nil", nil, nil, nil, nil},
		{"string_panic", TestEvent{}, "listener", "panic", []byte("trace")},
		{"error_panic", TestEvent{}, func() {}, errors.New("error"), []byte{}},
		{"int_panic", AnotherEvent{}, struct{}{}, 42, []byte("stack")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newDefaultPanicHandler()
			handler.Handle(tt.event, tt.listener, tt.panicValue, tt.stack)
		})
	}
}

func TestDefaultErrorHandler_Handle(t *testing.T) {
	t.Parallel()
	handler := newDefaultErrorHandler()
	if handler == nil {
		t.Fatal("expected non-nil error handler")
	}

	testErr := errors.New("test error")
	handler.Handle(TestEvent{Value: "test"}, "listener", testErr)
}

func TestDefaultErrorHandler_HandleWithNilError(t *testing.T) {
	t.Parallel()
	handler := newDefaultErrorHandler()

	handler.Handle(TestEvent{}, "listener", nil)
}

func TestDefaultErrorHandler_HandleWithVariousErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		event    any
		listener any
		err      error
	}{
		{"nil_error", TestEvent{}, "listener", nil},
		{"simple_error", TestEvent{}, func() {}, errors.New("simple")},
		{"wrapped_error", AnotherEvent{}, struct{}{}, ErrInvalidListener},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newDefaultErrorHandler()
			handler.Handle(tt.event, tt.listener, tt.err)
		})
	}
}
