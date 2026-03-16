package events

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestPublish_WithTransport(t *testing.T) {
	t.Parallel()

	tr := &fakeTransport{publishCh: make(chan Envelope, 10)}
	c := &fakeCodec{}

	d := New(WithTransport(tr), WithCodec(c))
	defer d.Close(context.Background())

	time.Sleep(50 * time.Millisecond)

	d.Publish(context.Background(), &plainTestEvent{Value: "hello"})

	select {
	case env := <-tr.publishCh:
		if env.Type != reflect.TypeFor[*plainTestEvent]().String() {
			t.Errorf("unexpected type %q", env.Type)
		}
		if env.ContentType != "application/fake" {
			t.Errorf("unexpected content type %q", env.ContentType)
		}
		if env.ID == "" {
			t.Error("expected non-empty ID")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for transport publish")
	}
}

func TestPublish_WithTransport_KeyedEvent(t *testing.T) {
	t.Parallel()

	tr := &fakeTransport{publishCh: make(chan Envelope, 10)}
	c := &fakeCodec{}

	d := New(WithTransport(tr), WithCodec(c))
	defer d.Close(context.Background())

	time.Sleep(50 * time.Millisecond)

	d.Publish(context.Background(), &keyedTestEvent{key: "order-1", Value: "v"})

	select {
	case env := <-tr.publishCh:
		if env.Key != "order-1" {
			t.Errorf("expected key %q, got %q", "order-1", env.Key)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestPublish_TransportNilCodec_NoPublish(t *testing.T) {
	t.Parallel()

	tr := &fakeTransport{publishCh: make(chan Envelope, 10)}

	d := New(WithTransport(tr))
	defer d.Close(context.Background())

	d.Publish(context.Background(), &plainTestEvent{})

	select {
	case <-tr.publishCh:
		t.Error("should not publish without codec")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPublish_TransportEncodeError(t *testing.T) {
	t.Parallel()

	tr := &fakeTransport{publishCh: make(chan Envelope, 10)}
	c := &fakeCodec{encodeErr: errTest}

	d := New(WithTransport(tr), WithCodec(c))
	defer d.Close(context.Background())

	err := d.Publish(context.Background(), &plainTestEvent{})
	if err == nil {
		t.Fatal("expected encode error")
	}
}

func TestPublish_TransportPublishError(t *testing.T) {
	t.Parallel()

	tr := &fakeTransport{
		publishCh:  make(chan Envelope, 10),
		publishErr: errTest,
	}
	c := &fakeCodec{}

	d := New(WithTransport(tr), WithCodec(c))
	defer d.Close(context.Background())

	err := d.Publish(context.Background(), &plainTestEvent{})
	if err == nil {
		t.Fatal("expected transport publish error")
	}
}

func TestInboundRouter_NoCodec(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	router := &inboundRouter{dispatcher: d}
	err := router.Handle(context.Background(), Envelope{})

	if err == nil {
		t.Fatal("expected error when codec is nil")
	}
}

func TestInboundRouter_UnknownType(t *testing.T) {
	t.Parallel()

	d := New(WithCodec(&fakeCodec{}))
	defer d.Close(context.Background())

	router := &inboundRouter{dispatcher: d}
	err := router.Handle(context.Background(), Envelope{Type: "unknown.Type"})

	if err != nil {
		t.Errorf("expected nil for unknown type, got %v", err)
	}
}

func TestLookupType_Found(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	Subscribe(d, newCountHandler[*plainTestEvent]())

	typ, ok := d.lookupType(reflect.TypeFor[*plainTestEvent]().String())
	if !ok {
		t.Fatal("expected to find type")
	}
	if typ != reflect.TypeFor[*plainTestEvent]() {
		t.Errorf("expected %v, got %v", reflect.TypeFor[*plainTestEvent](), typ)
	}
}

func TestLookupType_NotFound(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	_, ok := d.lookupType("nonexistent.Type")
	if ok {
		t.Error("expected not found")
	}
}

func TestClose_WithTransport_Error(t *testing.T) {
	t.Parallel()

	tr := &fakeTransport{
		publishCh: make(chan Envelope, 10),
		closeErr:  errTest,
	}

	d := New(WithTransport(tr))
	err := d.Close(context.Background())

	if !errors.Is(err, errTest) {
		t.Errorf("expected errTest, got %v", err)
	}
}

type fakeTransport struct {
	publishCh  chan Envelope
	publishErr error
	closeErr   error
	handler    TransportHandler
}

func (t *fakeTransport) Publish(_ context.Context, e Envelope) error {
	if t.publishErr != nil {
		return t.publishErr
	}
	t.publishCh <- e
	return nil
}

func (t *fakeTransport) Subscribe(ctx context.Context, h TransportHandler) error {
	t.handler = h
	<-ctx.Done()
	return ctx.Err()
}

func (t *fakeTransport) Close(_ context.Context) error {
	return t.closeErr
}

type fakeCodec struct {
	encodeErr error
	decodeErr error
}

func (c *fakeCodec) Encode(_ Event) ([]byte, error) {
	if c.encodeErr != nil {
		return nil, c.encodeErr
	}
	return []byte(`{}`), nil
}

func (c *fakeCodec) Decode(_ []byte, _ Event) error {
	return c.decodeErr
}

func (c *fakeCodec) ContentType() string {
	return "application/fake"
}
