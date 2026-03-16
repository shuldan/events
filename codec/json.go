package codec

import (
	"encoding/json"
	"fmt"

	"github.com/shuldan/events"
)

type jsonCodec struct{}

// NewJSON создаёт JSON-кодек.
func NewJSON() events.Codec {
	return &jsonCodec{}
}

func (c *jsonCodec) Encode(event events.Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("json codec: encode failed: %w", err)
	}
	return data, nil
}

func (c *jsonCodec) Decode(data []byte, target events.Event) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("json codec: decode failed: %w", err)
	}
	return nil
}

func (c *jsonCodec) ContentType() string {
	return "application/json"
}
