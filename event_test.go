package events

import (
	"testing"
	"time"
)

func TestNewBaseEvent(t *testing.T) {
	t.Parallel()
	before := time.Now().UTC()
	be := NewBaseEvent("test.created", "agg-123")
	after := time.Now().UTC()

	if be.Name != "test.created" {
		t.Errorf("expected name 'test.created', got %q", be.Name)
	}
	if be.AggrID != "agg-123" {
		t.Errorf("expected aggregate id 'agg-123', got %q", be.AggrID)
	}
	if be.ID == "" {
		t.Error("expected non-empty ID")
	}
	if be.Timestamp.Before(before) || be.Timestamp.After(after) {
		t.Errorf("timestamp %v not in range [%v, %v]", be.Timestamp, before, after)
	}
}

func TestBaseEvent_EventName(t *testing.T) {
	t.Parallel()
	be := BaseEvent{Name: "order.placed"}
	if be.EventName() != "order.placed" {
		t.Errorf("expected 'order.placed', got %q", be.EventName())
	}
}

func TestBaseEvent_OccurredAt(t *testing.T) {
	t.Parallel()
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	be := BaseEvent{Timestamp: ts}
	if !be.OccurredAt().Equal(ts) {
		t.Errorf("expected %v, got %v", ts, be.OccurredAt())
	}
}

func TestBaseEvent_AggregateID(t *testing.T) {
	t.Parallel()
	be := BaseEvent{AggrID: "agg-42"}
	if be.AggregateID() != "agg-42" {
		t.Errorf("expected 'agg-42', got %q", be.AggregateID())
	}
}

func TestBaseEvent_EmptyFields(t *testing.T) {
	t.Parallel()
	be := BaseEvent{}
	if be.EventName() != "" {
		t.Errorf("expected empty name, got %q", be.EventName())
	}
	if be.AggregateID() != "" {
		t.Errorf("expected empty aggregate id, got %q", be.AggregateID())
	}
	if !be.OccurredAt().IsZero() {
		t.Errorf("expected zero time, got %v", be.OccurredAt())
	}
}

func TestGenerateID_Unique(t *testing.T) {
	t.Parallel()
	ids := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := generateID()
		if id == "" {
			t.Fatal("generated empty ID")
		}
		if _, exists := ids[id]; exists {
			t.Fatalf("duplicate ID: %s", id)
		}
		ids[id] = struct{}{}
	}
}

func TestGenerateID_Format(t *testing.T) {
	t.Parallel()
	id := generateID()
	if len(id) == 0 {
		t.Fatal("empty ID")
	}
	dashCount := 0
	for _, c := range id {
		if c == '-' {
			dashCount++
		}
	}
	if dashCount != 4 {
		t.Errorf("expected 4 dashes in ID %q, got %d", id, dashCount)
	}
}

func TestBaseEvent_ImplementsEvent(t *testing.T) {
	t.Parallel()
	var _ Event = BaseEvent{}
	var _ Event = &BaseEvent{}
}
