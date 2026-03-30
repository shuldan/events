package kafka

import (
	"reflect"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/shuldan/events"
)

func makeFullEnvelope() events.Envelope {
	return events.Envelope{
		ID:          "evt-123",
		Type:        "OrderCreated",
		Key:         "order-456",
		Payload:     []byte(`{"id":"456"}`),
		ContentType: "application/json",
		Metadata:    map[string]string{"trace": "abc"},
		Timestamp:   time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}
}

func makeMinimalEnvelope() events.Envelope {
	return events.Envelope{
		ID:      "evt-min",
		Type:    "Ping",
		Payload: []byte("ping"),
	}
}

func findHeader(headers []kafka.Header, key string) (string, bool) {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value), true
		}
	}
	return "", false
}

func TestEnvelopeToMessage_FullEnvelope(t *testing.T) {
	t.Parallel()
	env := makeFullEnvelope()
	msg := envelopeToMessage("test-topic", env)

	if msg.Topic != "test-topic" {
		t.Errorf("expected topic test-topic, got %s", msg.Topic)
	}
	if string(msg.Key) != "order-456" {
		t.Errorf("expected key order-456, got %s", string(msg.Key))
	}
	if string(msg.Value) != `{"id":"456"}` {
		t.Errorf("unexpected value: %s", string(msg.Value))
	}

	checks := map[string]string{
		headerEventID:     "evt-123",
		headerEventType:   "OrderCreated",
		headerContentType: "application/json",
		"x-trace":         "abc",
	}
	for key, want := range checks {
		got, ok := findHeader(msg.Headers, key)
		if !ok {
			t.Errorf("missing header %s", key)
		} else if got != want {
			t.Errorf("header %s: expected %s, got %s", key, want, got)
		}
	}

	ts, ok := findHeader(msg.Headers, headerTimestamp)
	if !ok {
		t.Error("missing timestamp header")
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Errorf("timestamp parse error: %v", err)
	}
	if !parsed.Equal(env.Timestamp) {
		t.Errorf("expected timestamp %v, got %v", env.Timestamp, parsed)
	}
}

func TestEnvelopeToMessage_MinimalEnvelope(t *testing.T) {
	t.Parallel()
	env := makeMinimalEnvelope()
	msg := envelopeToMessage("topic", env)

	if msg.Key != nil {
		t.Errorf("expected nil key, got %v", msg.Key)
	}

	_, hasContentType := findHeader(msg.Headers, headerContentType)
	if hasContentType {
		t.Error("expected no content_type header for minimal envelope")
	}

	_, hasTimestamp := findHeader(msg.Headers, headerTimestamp)
	if hasTimestamp {
		t.Error("expected no timestamp header for minimal envelope")
	}
}

func TestEnvelopeToMessage_EmptyMetadata(t *testing.T) {
	t.Parallel()
	env := events.Envelope{ID: "e1", Type: "T", Payload: []byte("p")}
	msg := envelopeToMessage("t", env)

	for _, h := range msg.Headers {
		if len(h.Key) > 2 && h.Key[:2] == "x-" {
			t.Errorf("unexpected metadata header: %s", h.Key)
		}
	}
}

func TestMessageToEnvelope_FullMessage(t *testing.T) {
	t.Parallel()
	original := makeFullEnvelope()
	msg := envelopeToMessage("topic", original)
	restored := messageToEnvelope(msg)

	if restored.ID != original.ID {
		t.Errorf("ID: expected %s, got %s", original.ID, restored.ID)
	}
	if restored.Type != original.Type {
		t.Errorf("Type: expected %s, got %s", original.Type, restored.Type)
	}
	if restored.Key != original.Key {
		t.Errorf("Key: expected %s, got %s", original.Key, restored.Key)
	}
	if restored.ContentType != original.ContentType {
		t.Errorf("ContentType: expected %s, got %s", original.ContentType, restored.ContentType)
	}
	if !restored.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp: expected %v, got %v", original.Timestamp, restored.Timestamp)
	}
	if string(restored.Payload) != string(original.Payload) {
		t.Errorf("Payload: expected %s, got %s", original.Payload, restored.Payload)
	}
	if !reflect.DeepEqual(restored.Metadata, original.Metadata) {
		t.Errorf("Metadata: expected %v, got %v", original.Metadata, restored.Metadata)
	}
}

func TestMessageToEnvelope_MinimalMessage(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{
		Value: []byte("data"),
		Headers: []kafka.Header{
			{Key: headerEventID, Value: []byte("id1")},
			{Key: headerEventType, Value: []byte("Evt")},
		},
	}
	env := messageToEnvelope(msg)

	if env.ID != "id1" {
		t.Errorf("expected ID id1, got %s", env.ID)
	}
	if env.Type != "Evt" {
		t.Errorf("expected Type Evt, got %s", env.Type)
	}
	if env.Key != "" {
		t.Errorf("expected empty key, got %s", env.Key)
	}
	if env.ContentType != "" {
		t.Errorf("expected empty content type, got %s", env.ContentType)
	}
	if env.Metadata != nil {
		t.Errorf("expected nil metadata, got %v", env.Metadata)
	}
	if !env.Timestamp.IsZero() {
		t.Errorf("expected zero timestamp, got %v", env.Timestamp)
	}
}

func TestMessageToEnvelope_InvalidTimestamp(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{
		Value: []byte("data"),
		Headers: []kafka.Header{
			{Key: headerEventID, Value: []byte("id1")},
			{Key: headerEventType, Value: []byte("Evt")},
			{Key: headerTimestamp, Value: []byte("not-a-timestamp")},
		},
	}
	env := messageToEnvelope(msg)

	if !env.Timestamp.IsZero() {
		t.Errorf("expected zero timestamp for invalid input, got %v", env.Timestamp)
	}
}

func TestMessageToEnvelope_UnknownHeadersIgnored(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{
		Value: []byte("data"),
		Headers: []kafka.Header{
			{Key: headerEventID, Value: []byte("id1")},
			{Key: headerEventType, Value: []byte("Evt")},
			{Key: "unknown-header", Value: []byte("val")},
		},
	}
	env := messageToEnvelope(msg)

	if env.Metadata != nil {
		t.Errorf("expected nil metadata for non-prefixed headers, got %v", env.Metadata)
	}
}

func TestMessageToEnvelope_MultipleMetadata(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{
		Value: []byte("data"),
		Headers: []kafka.Header{
			{Key: headerEventID, Value: []byte("id1")},
			{Key: headerEventType, Value: []byte("Evt")},
			{Key: "x-foo", Value: []byte("bar")},
			{Key: "x-baz", Value: []byte("qux")},
		},
	}
	env := messageToEnvelope(msg)

	if len(env.Metadata) != 2 {
		t.Errorf("expected 2 metadata entries, got %d", len(env.Metadata))
	}
	if env.Metadata["foo"] != "bar" {
		t.Errorf("expected metadata foo=bar, got %s", env.Metadata["foo"])
	}
	if env.Metadata["baz"] != "qux" {
		t.Errorf("expected metadata baz=qux, got %s", env.Metadata["baz"])
	}
}

func TestRoundtrip_EnvelopeToMessageAndBack(t *testing.T) {
	t.Parallel()
	original := makeFullEnvelope()
	msg := envelopeToMessage("my-topic", original)
	restored := messageToEnvelope(msg)

	if restored.ID != original.ID ||
		restored.Type != original.Type ||
		restored.Key != original.Key ||
		restored.ContentType != original.ContentType ||
		string(restored.Payload) != string(original.Payload) {
		t.Error("roundtrip failed: fields mismatch")
	}
}

func TestMessageToEnvelope_NilKey(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{
		Key:   nil,
		Value: []byte("data"),
		Headers: []kafka.Header{
			{Key: headerEventID, Value: []byte("id1")},
		},
	}
	env := messageToEnvelope(msg)

	if env.Key != "" {
		t.Errorf("expected empty key for nil msg key, got %s", env.Key)
	}
}

func TestMessageToEnvelope_EmptyKey(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{
		Key:   []byte(""),
		Value: []byte("data"),
		Headers: []kafka.Header{
			{Key: headerEventID, Value: []byte("id1")},
		},
	}
	env := messageToEnvelope(msg)

	if env.Key != "" {
		t.Errorf("expected empty key for empty msg key, got %q", env.Key)
	}
}

func TestEnvelopeToMessage_ZeroTimestamp(t *testing.T) {
	t.Parallel()
	env := events.Envelope{
		ID:   "id",
		Type: "T",
	}
	msg := envelopeToMessage("t", env)

	_, found := findHeader(msg.Headers, headerTimestamp)
	if found {
		t.Error("expected no timestamp header for zero timestamp")
	}
}

func TestMessageToEnvelope_PrefixExactLength(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{
		Value: []byte("data"),
		Headers: []kafka.Header{
			{Key: headerEventID, Value: []byte("id1")},
			{Key: "x-", Value: []byte("val")},
		},
	}
	env := messageToEnvelope(msg)

	if env.Metadata != nil {
		t.Errorf("expected nil metadata for header exactly 'x-', got %v", env.Metadata)
	}
}
