package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shuldan/events"
)

type fakeWriter struct {
	mu       sync.Mutex
	msgs     []kafkago.Message
	writeErr error
	closeErr error
	closed   bool
}

func (w *fakeWriter) WriteMessages(_ context.Context, msgs ...kafkago.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writeErr != nil {
		return w.writeErr
	}
	w.msgs = append(w.msgs, msgs...)
	return nil
}

func (w *fakeWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return w.closeErr
}

func (w *fakeWriter) messageCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.msgs)
}

type fakeReader struct {
	mu         sync.Mutex
	messages   []kafkago.Message
	index      int
	fetchErr   error
	commitErr  error
	closed     bool
	fetchDelay time.Duration
	onCommit   func()
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	if r.fetchDelay > 0 {
		select {
		case <-time.After(r.fetchDelay):
		case <-ctx.Done():
			return kafkago.Message{}, ctx.Err()
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.fetchErr != nil {
		err := r.fetchErr
		r.fetchErr = nil
		return kafkago.Message{}, err
	}

	if r.index >= len(r.messages) {
		r.mu.Unlock()
		<-ctx.Done()
		r.mu.Lock()
		return kafkago.Message{}, ctx.Err()
	}

	msg := r.messages[r.index]
	r.index++
	return msg, nil
}

func (r *fakeReader) CommitMessages(_ context.Context, _ ...kafkago.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.onCommit != nil {
		r.onCommit()
	}
	return r.commitErr
}

func (r *fakeReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *fakeReader) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type captureHandler struct {
	mu   sync.Mutex
	envs []events.Envelope
	err  error
}

func (h *captureHandler) Handle(_ context.Context, env events.Envelope) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.envs = append(h.envs, env)
	return h.err
}

func (h *captureHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.envs)
}

func (h *captureHandler) envelopes() []events.Envelope {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]events.Envelope, len(h.envs))
	copy(out, h.envs)
	return out
}

func baseCfg() Config {
	return Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-events",
	}
}

func baseCfgWithGroup() Config {
	cfg := baseCfg()
	cfg.ConsumerGroup = "test-group"
	return cfg
}

func newTestTransport(cfg Config, w messageWriter, rf func() messageReader) *Transport {
	cfg.withDefaults()
	t := &Transport{
		cfg:    cfg,
		writer: w,
	}
	if rf != nil {
		t.newReader = rf
	}
	return t
}

func makeTestMessage(id, typ string) kafkago.Message {
	return kafkago.Message{
		Value: []byte("payload"),
		Headers: []kafkago.Header{
			{Key: headerEventID, Value: []byte(id)},
			{Key: headerEventType, Value: []byte(typ)},
		},
	}
}

func noopCreator(_ Config) error { return nil }
func noopChecker(_ Config) error { return nil }

func TestNewTransport_ValidationError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  Config
		err  error
	}{
		{"missing brokers", Config{Topic: "t"}, ErrMissingBrokers},
		{"missing topic", Config{Brokers: []string{"b"}}, ErrMissingTopic},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := newTransport(tc.cfg, noopCreator, noopChecker, nil, nil)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected %v, got %v", tc.err, err)
			}
		})
	}
}

func TestNewTransport_AutoCreateSuccess(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()
	cfg.AutoCreateTopics = true
	w := &fakeWriter{}

	called := false
	creator := func(_ Config) error { called = true; return nil }

	tr, err := newTransport(cfg, creator, noopChecker, w, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
	if !called {
		t.Error("expected creator called")
	}
}

func TestNewTransport_AutoCreateError(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()
	cfg.AutoCreateTopics = true

	creator := func(_ Config) error { return errors.New("create failed") }
	_, err := newTransport(cfg, creator, noopChecker, nil, nil)
	if err == nil || err.Error() != "ensure topic: create failed" {
		t.Errorf("expected 'ensure topic: create failed', got %v", err)
	}
}

func TestNewTransport_CheckSuccess(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()
	w := &fakeWriter{}

	called := false
	checker := func(_ Config) error { called = true; return nil }

	tr, err := newTransport(cfg, noopCreator, checker, w, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil")
	}
	if !called {
		t.Error("expected checker called")
	}
}

func TestNewTransport_CheckError(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()

	checker := func(c Config) error {
		return fmt.Errorf("%w: %s", ErrTopicNotFound, c.Topic)
	}
	_, err := newTransport(cfg, noopCreator, checker, nil, nil)
	if !errors.Is(err, ErrTopicNotFound) {
		t.Errorf("expected ErrTopicNotFound, got %v", err)
	}
}

func TestNewTransport_DefaultsApplied(t *testing.T) {
	t.Parallel()
	cfg := Config{Brokers: []string{"b"}, Topic: "t"}
	w := &fakeWriter{}

	tr, err := newTransport(cfg, noopCreator, noopChecker, w, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tr.cfg.NumPartitions != 1 {
		t.Errorf("expected 1, got %d", tr.cfg.NumPartitions)
	}
	if tr.cfg.ReplicationFactor != 1 {
		t.Errorf("expected 1, got %d", tr.cfg.ReplicationFactor)
	}
	if tr.cfg.MaxBytes != 1e6 {
		t.Errorf("expected 1000000, got %d", tr.cfg.MaxBytes)
	}
	if tr.cfg.WriteTimeout != 10*time.Second {
		t.Errorf("expected 10s, got %v", tr.cfg.WriteTimeout)
	}
}

func TestNewTransport_InjectedWriterUsed(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()
	w := &fakeWriter{}

	tr, err := newTransport(cfg, noopCreator, noopChecker, w, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tr.writer != w {
		t.Error("expected injected writer")
	}
}

func TestNewTransport_InjectedReaderFactory(t *testing.T) {
	t.Parallel()
	cfg := baseCfgWithGroup()
	w := &fakeWriter{}
	r := &fakeReader{}
	rf := func() messageReader { return r }

	tr, err := newTransport(cfg, noopCreator, noopChecker, w, rf)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	got := tr.newReader()
	if got != r {
		t.Error("expected injected reader factory")
	}
}

func TestNewTransport_NilWriterCreatesDefault(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()

	tr, err := newTransport(cfg, noopCreator, noopChecker, nil, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tr.writer == nil {
		t.Error("expected non-nil default writer")
	}
}

func TestNewTransport_NilReaderFactoryCreatesDefault(t *testing.T) {
	t.Parallel()
	cfg := baseCfgWithGroup()
	w := &fakeWriter{}

	tr, err := newTransport(cfg, noopCreator, noopChecker, w, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tr.newReader == nil {
		t.Error("expected non-nil default reader factory")
	}
}

func TestTransport_Publish_Success(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)

	err := tr.Publish(context.Background(), events.Envelope{ID: "e1", Type: "T"})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if w.messageCount() != 1 {
		t.Errorf("expected 1, got %d", w.messageCount())
	}
}

func TestTransport_Publish_WriteError(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{writeErr: errors.New("broker down")}
	tr := newTestTransport(baseCfg(), w, nil)

	err := tr.Publish(context.Background(), events.Envelope{})
	if err == nil || err.Error() != "broker down" {
		t.Errorf("expected 'broker down', got %v", err)
	}
}

func TestTransport_Publish_WhenClosed(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)
	_ = tr.Close(context.Background())

	err := tr.Publish(context.Background(), events.Envelope{})
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestTransport_Publish_Multiple(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)

	for i := 0; i < 5; i++ {
		if err := tr.Publish(context.Background(), events.Envelope{ID: "e", Type: "T"}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	if w.messageCount() != 5 {
		t.Errorf("expected 5, got %d", w.messageCount())
	}
}

func TestTransport_Subscribe_WhenClosed(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfgWithGroup(), w, nil)
	_ = tr.Close(context.Background())

	err := tr.Subscribe(context.Background(), nil)
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestTransport_Subscribe_MissingConsumerGroup(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)

	err := tr.Subscribe(context.Background(), nil)
	if !errors.Is(err, ErrMissingConsumerGroup) {
		t.Errorf("expected ErrMissingConsumerGroup, got %v", err)
	}
}

func TestTransport_Subscribe_DeliversMessages(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	r := &fakeReader{messages: []kafkago.Message{
		makeTestMessage("e1", "TypeA"),
		makeTestMessage("e2", "TypeB"),
	}}
	tr := newTestTransport(baseCfgWithGroup(), w, func() messageReader { return r })
	handler := &captureHandler{}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Subscribe(ctx, handler) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not return")
	}

	if handler.count() != 2 {
		t.Errorf("expected 2, got %d", handler.count())
	}
	envs := handler.envelopes()
	if envs[0].ID != "e1" || envs[1].ID != "e2" {
		t.Errorf("unexpected IDs: %s, %s", envs[0].ID, envs[1].ID)
	}
}

func TestTransport_Subscribe_HandlerError_CommitsAnyway(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	r := &fakeReader{messages: []kafkago.Message{makeTestMessage("e1", "T")}}
	tr := newTestTransport(baseCfgWithGroup(), w, func() messageReader { return r })
	handler := &captureHandler{err: errors.New("fail")}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Subscribe(ctx, handler) }()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if handler.count() != 1 {
		t.Errorf("expected 1, got %d", handler.count())
	}
}

func TestTransport_Subscribe_FetchError_Retries(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	r := &fakeReader{
		fetchErr: errors.New("temporary"),
		messages: []kafkago.Message{makeTestMessage("e1", "T")},
	}
	tr := newTestTransport(baseCfgWithGroup(), w, func() messageReader { return r })
	handler := &captureHandler{}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Subscribe(ctx, handler) }()

	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	if handler.count() != 1 {
		t.Errorf("expected 1 after retry, got %d", handler.count())
	}
}

func TestTransport_Subscribe_CommitError_ContinuesIfCtxOk(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	r := &fakeReader{
		messages:  []kafkago.Message{makeTestMessage("e1", "T")},
		commitErr: errors.New("commit fail"),
	}
	tr := newTestTransport(baseCfgWithGroup(), w, func() messageReader { return r })
	handler := &captureHandler{}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Subscribe(ctx, handler) }()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if handler.count() != 1 {
		t.Errorf("expected 1, got %d", handler.count())
	}
}

func TestTransport_Subscribe_CommitError_ReturnsIfCtxDone(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	ctx, cancel := context.WithCancel(context.Background())

	r := &fakeReader{
		messages:  []kafkago.Message{makeTestMessage("e1", "T")},
		commitErr: errors.New("commit fail"),
		onCommit:  func() { cancel() },
	}
	tr := newTestTransport(baseCfgWithGroup(), w, func() messageReader { return r })
	handler := &captureHandler{}

	done := make(chan error, 1)
	go func() { done <- tr.Subscribe(ctx, handler) }()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not return")
	}
}

func TestTransport_Subscribe_ClosesReader(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	r := &fakeReader{}
	tr := newTestTransport(baseCfgWithGroup(), w, func() messageReader { return r })
	handler := &captureHandler{}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Subscribe(ctx, handler) }()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if !r.isClosed() {
		t.Error("expected reader closed")
	}
}

func TestTransport_Subscribe_ContextCancelDuringFetch(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	r := &fakeReader{fetchDelay: 5 * time.Second}
	tr := newTestTransport(baseCfgWithGroup(), w, func() messageReader { return r })
	handler := &captureHandler{}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.Subscribe(ctx, handler) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not return")
	}
}

func TestTransport_Close_Success(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)

	err := tr.Close(context.Background())
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		t.Error("expected writer closed")
	}
}

func TestTransport_Close_WriterError(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{closeErr: errors.New("close failed")}
	tr := newTestTransport(baseCfg(), w, nil)

	err := tr.Close(context.Background())
	if err == nil || err.Error() != "close failed" {
		t.Errorf("expected 'close failed', got %v", err)
	}
}

func TestTransport_Close_AlreadyClosed(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)
	_ = tr.Close(context.Background())

	err := tr.Close(context.Background())
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestTransport_Close_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)

	var wg sync.WaitGroup
	results := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = tr.Close(context.Background())
		}(i)
	}
	wg.Wait()

	success := 0
	for _, err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Errorf("expected 1 success, got %d", success)
	}
}

func TestTransport_Publish_ConcurrentWithClose(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	tr := newTestTransport(baseCfg(), w, nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = tr.Publish(context.Background(), events.Envelope{ID: "e", Type: "T"})
	}()
	go func() {
		defer wg.Done()
		_ = tr.Close(context.Background())
	}()
	wg.Wait()
}
