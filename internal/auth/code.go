package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// CodeDigits is the length of the typed code emailed at sign up and on a
// password reset.
const CodeDigits = 6

// Key labels. One server secret (SESSION_SECRET) feeds several uses; each use
// gets its own key from DeriveKey so the raw secret is never a key and a key
// for one use cannot be replayed for another (D-02). The v1 suffix lets a later
// change rotate a use without touching the others.
const (
	CodeKeyLabel     = "pinalert/auth-code-key/v1"
	ThrottleKeyLabel = "pinalert/auth-throttle-key/v1"
)

// codeLabel is the versioned label CodeHash mixes into every code hash.
const codeLabel = "pinalert/auth-code/v1"

// NewCode returns a uniformly random 6 digit code, 000000 to 999999, zero
// padded. It uses crypto/rand with rand.Int, which rejects biased draws, never
// the predictable seeded generator and never a modulo of raw bytes (threat T-07.1-05). The error is
// returned, not panicked, because this runs inside a request.
func NewCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", CodeDigits, n.Int64()), nil
}

// KeyedHash is HMAC-SHA256 under key over label + "|" + value, hex encoded.
// The throttle uses it so the database holds no raw email address or device
// id, and the code hash is built on it. A bare hash would be unsafe here: the
// code space is only 10 to the power 6, so anyone who read a table of bare
// hashes would invert it in a moment (threat T-07.1-01). The key stays on the
// server.
func KeyedHash(key []byte, label, value string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(label + "|" + value))
	return hex.EncodeToString(m.Sum(nil))
}

// CodeHash is the stored form of a code: KeyedHash over the versioned code
// label with the purpose, the address and the code. Binding the purpose and the
// address means a code issued for one address or one use never checks out for
// another.
func CodeHash(key []byte, purpose, email, code string) string {
	return KeyedHash(key, codeLabel, purpose+"|"+email+"|"+code)
}

// CodeHashEqual compares two hex hashes in constant time with hmac.Equal. A
// value that is not valid hex, or empty, is never equal.
func CodeHashEqual(a, b string) bool {
	x, err := hex.DecodeString(a)
	if err != nil || len(x) == 0 {
		return false
	}
	y, err := hex.DecodeString(b)
	if err != nil || len(y) == 0 {
		return false
	}
	return hmac.Equal(x, y)
}

// DeriveKey returns a 32 byte key for one use: HMAC-SHA256(secret, label).
func DeriveKey(secret []byte, label string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(label))
	return m.Sum(nil)
}
