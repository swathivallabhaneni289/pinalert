package push

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPayloadEncode(t *testing.T) {
	t.Run("fixed envelope", func(t *testing.T) {
		p := Payload{V: PayloadVersion, Kind: "critical", Tag: "report-123", Title: "Water rising", Body: "About 3 km from your home area. Not confirmed yet.", URL: "/?report=123"}
		raw, err := p.Encode()
		if err != nil {
			t.Fatalf("Encode error = %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("encoded payload is not JSON: %v", err)
		}
		want := map[string]any{"v": float64(1), "kind": "critical", "tag": "report-123", "title": "Water rising", "body": "About 3 km from your home area. Not confirmed yet.", "url": "/?report=123"}
		if len(got) != len(want) {
			t.Fatalf("encoded keys = %v, want exactly %v", got, want)
		}
		for k, v := range want {
			if got[k] != v {
				t.Fatalf("key %q = %v, want %v", k, got[k], v)
			}
		}
	})
	t.Run("just under the limit", func(t *testing.T) {
		p := Payload{V: PayloadVersion, Kind: "critical", Tag: "t", Title: "T", Body: strings.Repeat("a", 900), URL: "/"}
		raw, err := p.Encode()
		if err != nil {
			t.Fatalf("Encode error = %v", err)
		}
		if len(raw) > MaxPayloadBytes {
			t.Fatalf("encoded length %d over limit", len(raw))
		}
	})
	t.Run("over the limit", func(t *testing.T) {
		p := Payload{V: PayloadVersion, Kind: "critical", Tag: "t", Title: "T", Body: strings.Repeat("a", MaxPayloadBytes), URL: "/"}
		raw, err := p.Encode()
		if !errors.Is(err, ErrPayloadTooLarge) {
			t.Fatalf("Encode error = %v, want ErrPayloadTooLarge", err)
		}
		if raw != nil {
			t.Fatalf("Encode returned bytes alongside the error")
		}
	})
	t.Run("version constant", func(t *testing.T) {
		if PayloadVersion != 1 || MaxPayloadBytes != 1024 {
			t.Fatalf("constants changed: %d %d", PayloadVersion, MaxPayloadBytes)
		}
	})
}

func TestValidTopic(t *testing.T) {
	cases := []struct {
		name  string
		topic string
		want  bool
	}{
		{"report id", "r123", true},
		{"one char", "a", true},
		{"alphabet", "AZaz09-_", true},
		{"exactly 32", strings.Repeat("a", 32), true},
		{"empty", "", false},
		{"33 chars", strings.Repeat("a", 33), false},
		{"space", "r 1", false},
		{"plus", "r+1", false},
		{"slash", "r/1", false},
		{"equals padding", "r1==", false},
		{"non ascii", "ré", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidTopic(tc.topic); got != tc.want {
				t.Fatalf("ValidTopic(%q) = %v, want %v", tc.topic, got, tc.want)
			}
		})
	}
}
