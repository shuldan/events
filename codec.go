package events

// Codec сериализует/десериализует события.
type Codec interface {
	Encode(event Event) ([]byte, error)
	Decode(data []byte, target Event) error
	ContentType() string
}
