package codec

import (
	"testing"
)

type testEvent struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestJSONCodec_Encode_Success(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	event := &testEvent{Name: "test", Value: 42}

	data, err := c.Encode(event)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty data")
	}

	expected := `{"name":"test","value":42}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestJSONCodec_Encode_NilEvent(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	data, err := c.Encode(nil)

	if err != nil {
		t.Fatalf("expected nil error for nil event, got %v", err)
	}
	if string(data) != "null" {
		t.Errorf("expected null, got %s", string(data))
	}
}

func TestJSONCodec_Encode_InvalidValue(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	_, err := c.Encode(make(chan int))

	if err == nil {
		t.Fatal("expected error for channel type")
	}
}

func TestJSONCodec_Decode_Success(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	data := []byte(`{"name":"decoded","value":99}`)

	var target testEvent
	err := c.Decode(data, &target)

	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if target.Name != "decoded" {
		t.Errorf("expected %q, got %q", "decoded", target.Name)
	}
	if target.Value != 99 {
		t.Errorf("expected 99, got %d", target.Value)
	}
}

func TestJSONCodec_Decode_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	var target testEvent
	err := c.Decode([]byte(`{invalid`), &target)

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestJSONCodec_Decode_EmptyData(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	var target testEvent
	err := c.Decode([]byte{}, &target)

	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestJSONCodec_ContentType(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	ct := c.ContentType()

	if ct != "application/json" {
		t.Errorf("expected %q, got %q", "application/json", ct)
	}
}

func TestJSONCodec_RoundTrip(t *testing.T) {
	t.Parallel()

	c := NewJSON()
	original := &testEvent{Name: "round", Value: 123}

	data, err := c.Encode(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded testEvent
	err = c.Decode(data, &decoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("name: expected %q, got %q", original.Name, decoded.Name)
	}
	if decoded.Value != original.Value {
		t.Errorf("value: expected %d, got %d", original.Value, decoded.Value)
	}
}
