package auth

import (
	"bytes"
	"regexp"
	"testing"
)

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

func TestNewCodeShape(t *testing.T) {
	if CodeDigits != 6 {
		t.Fatalf("CodeDigits = %d", CodeDigits)
	}
	for i := 0; i < 2000; i++ {
		c, err := NewCode()
		if err != nil {
			t.Fatalf("NewCode: %v", err)
		}
		if !sixDigits.MatchString(c) {
			t.Fatalf("code %q is not exactly 6 ASCII digits", c)
		}
	}
}

func TestNewCodeUniform(t *testing.T) {
	var seen [CodeDigits][10]int
	leadingZero := 0
	for i := 0; i < 20000; i++ {
		c, err := NewCode()
		if err != nil {
			t.Fatalf("NewCode: %v", err)
		}
		if len(c) != CodeDigits {
			t.Fatalf("code %q has length %d", c, len(c))
		}
		for pos := 0; pos < CodeDigits; pos++ {
			seen[pos][c[pos]-'0']++
		}
		if c[0] == '0' {
			leadingZero++
		}
	}
	for pos := range seen {
		for d, n := range seen[pos] {
			if n == 0 {
				t.Errorf("digit %d never appeared at position %d", d, pos)
			}
		}
	}
	// About 10 percent of codes start with 0; zero padding must not be lost.
	if leadingZero < 1000 || leadingZero > 3000 {
		t.Errorf("%d of 20000 codes start with 0, expected about 2000", leadingZero)
	}
}

func TestCodeHashKeyed(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	base := CodeHash(key, "signup", "a@example.com", "123456")

	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(base) {
		t.Fatalf("hash %q is not 64 lowercase hex characters", base)
	}
	if base != CodeHash(key, "signup", "a@example.com", "123456") {
		t.Fatal("hash is not deterministic")
	}
	otherKey := bytes.Repeat([]byte{8}, 32)
	variants := map[string]string{
		"key":     CodeHash(otherKey, "signup", "a@example.com", "123456"),
		"purpose": CodeHash(key, "reset", "a@example.com", "123456"),
		"email":   CodeHash(key, "signup", "b@example.com", "123456"),
		"code":    CodeHash(key, "signup", "a@example.com", "123457"),
	}
	for what, h := range variants {
		if h == base {
			t.Errorf("changing the %s did not change the hash", what)
		}
	}

	if !CodeHashEqual(base, CodeHash(key, "signup", "a@example.com", "123456")) {
		t.Error("equal hashes compare unequal")
	}
	if CodeHashEqual(base, variants["code"]) {
		t.Error("different hashes compare equal")
	}
	if CodeHashEqual(base, "not hex") || CodeHashEqual("zz", "zz") || CodeHashEqual("", "") {
		t.Error("bad hex must compare false")
	}
}

func TestDeriveKeyLabels(t *testing.T) {
	secret := []byte("a-session-secret-of-some-length!")
	code := DeriveKey(secret, CodeKeyLabel)
	throttle := DeriveKey(secret, ThrottleKeyLabel)

	if CodeKeyLabel != "pinalert/auth-code-key/v1" || ThrottleKeyLabel != "pinalert/auth-throttle-key/v1" {
		t.Fatalf("labels changed: %q %q", CodeKeyLabel, ThrottleKeyLabel)
	}
	if len(code) != 32 || len(throttle) != 32 {
		t.Fatalf("key lengths %d, %d, want 32", len(code), len(throttle))
	}
	if bytes.Equal(code, throttle) {
		t.Error("code and throttle keys are equal")
	}
	if bytes.Equal(code, secret) || bytes.Equal(throttle, secret) {
		t.Error("a derived key equals the raw secret")
	}
	if !bytes.Equal(code, DeriveKey(secret, CodeKeyLabel)) {
		t.Error("derivation is not deterministic")
	}
	if bytes.Equal(code, DeriveKey([]byte("another secret entirely, 32 byte"), CodeKeyLabel)) {
		t.Error("different secrets gave the same key")
	}

	// KeyedHash: label and value both matter, output is hex.
	h := KeyedHash(throttle, "email", "a@example.com")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(h) {
		t.Fatalf("KeyedHash %q is not 64 hex characters", h)
	}
	if h == KeyedHash(throttle, "device", "a@example.com") || h == KeyedHash(throttle, "email", "b@example.com") {
		t.Error("KeyedHash ignores its label or value")
	}
}
