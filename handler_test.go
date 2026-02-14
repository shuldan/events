package events

import (
	"testing"
)

func TestDefaultPanicHandler_Handle(t *testing.T) {
	t.Parallel()
	h := &defaultPanicHandler{}
	evt := newTestEvent("panic-event", "agg-1")
	h.Handle(evt, "test panic", []byte("fake stack"))
}

func TestDefaultErrorHandler_Handle(t *testing.T) {
	t.Parallel()
	h := &defaultErrorHandler{}
	evt := newTestEvent("error-event", "agg-1")
	h.Handle(evt, errTest)
}

var errTest = errForTest("test error")

type errForTest string

func (e errForTest) Error() string { return string(e) }
