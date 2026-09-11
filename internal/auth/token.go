// Package auth mints and hashes the single-use magic-link tokens Phase 1.1's
// mandatory email verification depends on. The raw token returned by
// GenerateToken is the secret that gets emailed to the visitor; only its
// SHA-256 hash is ever written to internal/store's magic_link_tokens table
// (see 01.1-01-PLAN.md threat T-01-52) — a database read never yields a
// usable link.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// tokenBytes is the number of raw random bytes drawn from crypto/rand per
// token, before base64url encoding — 256 bits, scaled up from
// internal/session/cookie.go's 16-byte session id (DEC-B, 01.1-01-PLAN.md).
const tokenBytes = 32

// TokenTTL is how long a magic-link token remains valid after issuance
// (D-02, 01.1-CONTEXT.md).
const TokenTTL = 5 * time.Minute

// GenerateToken draws tokenBytes from crypto/rand — the cryptographically
// secure generator, never math/rand, whose output would be predictable from
// observed values and let an attacker guess future tokens — and returns the
// base64url-encoded raw token alongside the hex-encoded SHA-256 hash of that
// raw value. Unlike session.Manager.NewID, the error from rand.Read is
// returned rather than panicking: this runs inside a request that can fail
// cleanly with a 500, not at process startup.
func GenerateToken() (raw string, hash string, err error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken returns the hex-encoded SHA-256 hash of raw. It is deterministic
// so the verify path (plan 01.1-02) can hash an incoming token and look it up
// by the same value that GenerateToken persisted.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
