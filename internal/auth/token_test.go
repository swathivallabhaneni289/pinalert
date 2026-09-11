package auth_test

import (
	"encoding/base64"
	"regexp"
	"testing"
	"time"

	"pinalert/internal/auth"
)

var hexRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestTokenGenerateShape(t *testing.T) {
	raw, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("raw token is not valid base64url: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("raw token decodes to %d bytes, want 32", len(decoded))
	}

	if hash != auth.HashToken(raw) {
		t.Fatalf("hash %q does not equal HashToken(raw) %q", hash, auth.HashToken(raw))
	}
}

func TestTokenGenerateUnique(t *testing.T) {
	raw1, hash1, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	raw2, hash2, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if raw1 == raw2 {
		t.Fatalf("two consecutive GenerateToken calls returned the same raw value")
	}
	if hash1 == hash2 {
		t.Fatalf("two consecutive GenerateToken calls returned the same hash")
	}
}

func TestTokenHashDeterministic(t *testing.T) {
	hash1 := auth.HashToken("some-raw-token-value")
	hash2 := auth.HashToken("some-raw-token-value")

	if hash1 != hash2 {
		t.Fatalf("HashToken is not deterministic: %q != %q", hash1, hash2)
	}
	if !hexRE.MatchString(hash1) {
		t.Fatalf("HashToken output %q is not 64 lowercase hex characters", hash1)
	}
}

func TestTokenTTL(t *testing.T) {
	if auth.TokenTTL != 5*time.Minute {
		t.Fatalf("TokenTTL = %v, want 5m", auth.TokenTTL)
	}
}
