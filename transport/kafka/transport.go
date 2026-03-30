package kafka

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shuldan/events"
)

type topicChecker func(cfg Config) error
type topicCreator func(cfg Config) error

type Transport struct {
	cfg    Config
	writer messageWriter

	newReader func() messageReader

	mu     sync.Mutex
	closed bool
}

func New(cfg Config) (*Transport, error) {
	return newTransport(cfg, ensureTopic, checkTopicExists, nil, nil)
}

func newTransport(
	cfg Config,
	create topicCreator,
	check topicChecker,
	w messageWriter,
	rf func() messageReader,
) (*Transport, error) {
	cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.AutoCreateTopics {
		if err := create(cfg); err != nil {
			return nil, fmt.Errorf("ensure topic: %w", err)
		}
	} else {
		if err := check(cfg); err != nil {
			return nil, err
		}
	}

	if w == nil {
		w = &kafkago.Writer{
			Addr:         kafkago.TCP(cfg.Brokers...),
			Topic:        cfg.Topic,
			Balancer:     &kafkago.Hash{},
			WriteTimeout: cfg.WriteTimeout,
			RequiredAcks: kafkago.RequireAll,
			Async:        false,
		}
	}

	t := &Transport{
		cfg:    cfg,
		writer: w,
	}

	if rf != nil {
		t.newReader = rf
	} else {
		t.newReader = func() messageReader {
			return kafkago.NewReader(kafkago.ReaderConfig{
				Brokers:        cfg.Brokers,
				Topic:          cfg.Topic,
				GroupID:        cfg.ConsumerGroup,
				MaxBytes:       cfg.MaxBytes,
				CommitInterval: cfg.CommitInterval,
				StartOffset:    kafkago.LastOffset,
			})
		}
	}

	return t, nil
}

func (t *Transport) Publish(ctx context.Context, env events.Envelope) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrTransportClosed
	}
	t.mu.Unlock()

	msg := envelopeToMessage(t.cfg.Topic, env)
	return t.writer.WriteMessages(ctx, msg)
}

func (t *Transport) Subscribe(ctx context.Context, handler events.TransportHandler) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrTransportClosed
	}
	if t.cfg.ConsumerGroup == "" {
		t.mu.Unlock()
		return ErrMissingConsumerGroup
	}
	t.mu.Unlock()

	reader := t.newReader()
	defer func() { _ = reader.Close() }()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		env := messageToEnvelope(msg)

		if err := handler.Handle(ctx, env); err != nil {
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
}

func (t *Transport) Close(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTransportClosed
	}
	t.closed = true

	return t.writer.Close()
}

func ensureTopic(cfg Config) error {
	conn, err := kafkago.Dial("tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get controller: %w", err)
	}

	controllerConn, err := kafkago.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("dial controller: %w", err)
	}
	defer func() { _ = controllerConn.Close() }()

	return controllerConn.CreateTopics(kafkago.TopicConfig{
		Topic:             cfg.Topic,
		NumPartitions:     cfg.NumPartitions,
		ReplicationFactor: cfg.ReplicationFactor,
	})
}

func checkTopicExists(cfg Config) error {
	conn, err := kafkago.Dial("tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer func() { _ = conn.Close() }()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return fmt.Errorf("read partitions: %w", err)
	}

	for _, p := range partitions {
		if p.Topic == cfg.Topic {
			return nil
		}
	}

	return fmt.Errorf("%w: %s", ErrTopicNotFound, cfg.Topic)
}
