package kafka

import (
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/shuldan/events"
)

const (
	headerEventID     = "event_id"
	headerEventType   = "event_type"
	headerContentType = "content_type"
	headerTimestamp   = "timestamp"
	headerPrefix      = "x-"
)

// envelopeToMessage converts an Envelope to a kafka.Message.
func envelopeToMessage(topic string, env events.Envelope) kafka.Message {
	headers := []kafka.Header{
		{Key: headerEventID, Value: []byte(env.ID)},
		{Key: headerEventType, Value: []byte(env.Type)},
	}

	if env.ContentType != "" {
		headers = append(headers, kafka.Header{
			Key:   headerContentType,
			Value: []byte(env.ContentType),
		})
	}

	if !env.Timestamp.IsZero() {
		headers = append(headers, kafka.Header{
			Key:   headerTimestamp,
			Value: []byte(env.Timestamp.Format(time.RFC3339Nano)),
		})
	}

	for k, v := range env.Metadata {
		headers = append(headers, kafka.Header{
			Key:   headerPrefix + k,
			Value: []byte(v),
		})
	}

	var key []byte
	if env.Key != "" {
		key = []byte(env.Key)
	}

	return kafka.Message{
		Topic:   topic,
		Key:     key,
		Headers: headers,
		Value:   env.Payload,
	}
}

// messageToEnvelope converts a kafka.Message to an Envelope.
func messageToEnvelope(msg kafka.Message) events.Envelope {
	env := events.Envelope{
		Payload: msg.Value,
	}

	if msg.Key != nil {
		env.Key = string(msg.Key)
	}

	var metadata map[string]string

	for _, h := range msg.Headers {
		switch h.Key {
		case headerEventID:
			env.ID = string(h.Value)
		case headerEventType:
			env.Type = string(h.Value)
		case headerContentType:
			env.ContentType = string(h.Value)
		case headerTimestamp:
			if t, err := time.Parse(time.RFC3339Nano, string(h.Value)); err == nil {
				env.Timestamp = t
			}
		default:
			if len(h.Key) > len(headerPrefix) && h.Key[:len(headerPrefix)] == headerPrefix {
				if metadata == nil {
					metadata = make(map[string]string)
				}
				metadata[h.Key[len(headerPrefix):]] = string(h.Value)
			}
		}
	}

	if metadata != nil {
		env.Metadata = metadata
	}

	return env
}
