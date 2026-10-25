package push

import (
	"encoding/json"
	"errors"
)

// PayloadVersion is the version written into every payload so the service
// worker can ignore a shape it does not know.
const PayloadVersion = 1

// MaxPayloadBytes bounds an encoded payload. It sits well under webpush-go's
// 3993 byte plaintext limit (every message is padded to one 4096 byte record).
const MaxPayloadBytes = 1024

// ErrPayloadTooLarge means an encoded payload went over MaxPayloadBytes. The
// builder keeps payloads small, so this is a bug signal, not a user error.
var ErrPayloadTooLarge = errors.New("push: payload over the size budget")

// Payload is the envelope the service worker reads. It carries only words the
// server wrote itself (the service layer builds them) and never a report
// description, a name, a phone number, an address or a coordinate.
type Payload struct {
	V     int    `json:"v"`
	Kind  string `json:"kind"`
	Tag   string `json:"tag"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// Encode writes the payload as JSON and refuses an encoding over
// MaxPayloadBytes.
func (p Payload) Encode() ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxPayloadBytes {
		return nil, ErrPayloadTooLarge
	}
	return raw, nil
}

// maxTopicLen is the longest Web Push Topic header value (RFC 8030).
const maxTopicLen = 32

// ValidTopic reports whether topic is 1 to 32 characters of the base64url
// alphabet, which is what push services accept in the Topic header.
func ValidTopic(topic string) bool {
	if topic == "" || len(topic) > maxTopicLen {
		return false
	}
	for i := 0; i < len(topic); i++ {
		c := topic[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}
