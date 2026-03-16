package middleware

import (
	"context"
	"errors"
	"testing"
)

func TestMetricsMiddleware_Success(t *testing.T) {
	t.Parallel()

	recorder := NewInMemoryRecorder()
	mw := NewMetrics(recorder)

	next := &successNext{}
	wrapped := mw.Wrap(next)

	err := wrapped.Handle(context.Background(), &testEvent{Value: "ok"})

	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	metrics := recorder.Metrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Err != nil {
		t.Errorf("expected nil error in metric, got %v", metrics[0].Err)
	}
	if metrics[0].Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestMetricsMiddleware_Error(t *testing.T) {
	t.Parallel()

	recorder := NewInMemoryRecorder()
	mw := NewMetrics(recorder)

	testErr := errors.New("fail")
	next := &errorNext{err: testErr}
	wrapped := mw.Wrap(next)

	err := wrapped.Handle(context.Background(), &testEvent{})

	if !errors.Is(err, testErr) {
		t.Errorf("expected %v, got %v", testErr, err)
	}

	metrics := recorder.Metrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if !errors.Is(metrics[0].Err, testErr) {
		t.Errorf("expected %v in metric, got %v", testErr, metrics[0].Err)
	}
}

func TestMetricsMiddleware_EventType(t *testing.T) {
	t.Parallel()

	recorder := NewInMemoryRecorder()
	mw := NewMetrics(recorder)

	wrapped := mw.Wrap(&successNext{})
	wrapped.Handle(context.Background(), &testEvent{})

	metrics := recorder.Metrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].EventType != "*middleware.testEvent" {
		t.Errorf("expected *middleware.testEvent, got %q", metrics[0].EventType)
	}
}

func TestInMemoryRecorder_MultipleRecords(t *testing.T) {
	t.Parallel()

	recorder := NewInMemoryRecorder()

	recorder.RecordEventHandled("typeA", 0, nil)
	recorder.RecordEventHandled("typeB", 0, errors.New("x"))

	metrics := recorder.Metrics()
	if len(metrics) != 2 {
		t.Fatalf("expected 2, got %d", len(metrics))
	}
	if metrics[0].EventType != "typeA" {
		t.Errorf("expected typeA, got %q", metrics[0].EventType)
	}
	if metrics[1].EventType != "typeB" {
		t.Errorf("expected typeB, got %q", metrics[1].EventType)
	}
}

func TestInMemoryRecorder_ReturnsCopy(t *testing.T) {
	t.Parallel()

	recorder := NewInMemoryRecorder()
	recorder.RecordEventHandled("typeA", 0, nil)

	m1 := recorder.Metrics()
	m2 := recorder.Metrics()

	m1[0].EventType = "modified"
	if m2[0].EventType == "modified" {
		t.Error("expected Metrics() to return a copy")
	}
}
