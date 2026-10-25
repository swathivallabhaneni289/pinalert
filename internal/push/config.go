package push

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

// Environment variable names for the VAPID identity.
const (
	EnvPublicKey  = "VAPID_PUBLIC_KEY"
	EnvPrivateKey = "VAPID_PRIVATE_KEY"
	EnvSubject    = "VAPID_SUBJECT"
	// EnvContactEmail and EnvBaseURL are the fallbacks for the subject, in
	// that order; BASE_URL only counts when it is https.
	EnvContactEmail = "CONTACT_EMAIL"
	EnvBaseURL      = "BASE_URL"
)

// Config is the VAPID identity the server signs its sends with. PublicKey and
// PrivateKey are base64url strings from go run ./cmd/vapidkeys; Subject is a
// bare email address or an https URL (webpush-go adds "mailto:" itself).
//
// The key pair is made once with go run ./cmd/vapidkeys and kept in the
// host's secret settings, never in the repo. Changing it breaks every stored
// subscription (Apple answers VapidPkHashMismatch), so rotate only with a plan
// to ask every device again. Keys are never generated at startup, in
// development too.
type Config struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

// LoadConfig reads the VAPID identity from the environment through getenv.
//
//   - Both keys unset: ok is false and the error is nil. Push is off; the
//     caller logs one warning and the app stays usable.
//   - Only one key set, a malformed key, a pair that does not belong together
//     or a bad subject: an error, which stops startup. No error text holds a
//     key value.
func LoadConfig(getenv func(string) string) (Config, bool, error) {
	pub := strings.TrimSpace(getenv(EnvPublicKey))
	priv := strings.TrimSpace(getenv(EnvPrivateKey))
	if pub == "" && priv == "" {
		return Config{}, false, nil
	}
	if pub == "" {
		return Config{}, false, fmt.Errorf("push: %s is set but %s is not; set both or neither", EnvPrivateKey, EnvPublicKey)
	}
	if priv == "" {
		return Config{}, false, fmt.Errorf("push: %s is set but %s is not; set both or neither", EnvPublicKey, EnvPrivateKey)
	}

	pubBytes, ok := decodeVAPIDKey(pub)
	if !ok || len(pubBytes) != p256dhLen || pubBytes[0] != 0x04 {
		return Config{}, false, fmt.Errorf("push: %s is not a base64url uncompressed P-256 public key (65 bytes)", EnvPublicKey)
	}
	privBytes, ok := decodeVAPIDKey(priv)
	if !ok || len(privBytes) != 32 {
		return Config{}, false, fmt.Errorf("push: %s is not a base64url P-256 private key (32 bytes)", EnvPrivateKey)
	}
	key, err := ecdh.P256().NewPrivateKey(privBytes)
	if err != nil {
		return Config{}, false, fmt.Errorf("push: %s is not a valid P-256 private key", EnvPrivateKey)
	}
	if !bytes.Equal(key.PublicKey().Bytes(), pubBytes) {
		return Config{}, false, fmt.Errorf("push: %s and %s do not belong together", EnvPublicKey, EnvPrivateKey)
	}

	subject, err := loadSubject(getenv)
	if err != nil {
		return Config{}, false, err
	}
	return Config{PublicKey: pub, PrivateKey: priv, Subject: subject}, true, nil
}

// decodeVAPIDKey accepts the two forms webpush-go itself decodes: base64url
// with or without padding.
func decodeVAPIDKey(s string) ([]byte, bool) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, true
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, true
	}
	return nil, false
}

// loadSubject picks VAPID_SUBJECT, then CONTACT_EMAIL, then an https
// BASE_URL, and validates the one it picks.
func loadSubject(getenv func(string) string) (string, error) {
	if v := strings.TrimSpace(getenv(EnvSubject)); v != "" {
		subject, err := normalizeSubject(v)
		if err != nil {
			return "", fmt.Errorf("push: %s must be an email address or an https URL", EnvSubject)
		}
		return subject, nil
	}
	if v := strings.TrimSpace(getenv(EnvContactEmail)); v != "" {
		subject, err := normalizeSubject(v)
		if err != nil {
			return "", fmt.Errorf("push: %s must be an email address or an https URL", EnvContactEmail)
		}
		return subject, nil
	}
	if v := strings.TrimSpace(getenv(EnvBaseURL)); strings.HasPrefix(strings.ToLower(v), "https://") {
		if subject, err := normalizeSubject(v); err == nil {
			return subject, nil
		}
	}
	return "", fmt.Errorf("push: %s must be set (or %s, or an https %s): push needs a contact for the push services", EnvSubject, EnvContactEmail, EnvBaseURL)
}

// normalizeSubject returns a bare email address (any "mailto:" prefix is
// dropped, because webpush-go adds its own) or an https URL with its scheme
// lower cased. webpush-go checks for "https:" case sensitively and prepends
// "mailto:" to anything else, so "HTTPS://..." must not reach it as given
// (review S-02, 2026-10-25).
func normalizeSubject(s string) (string, error) {
	if strings.HasPrefix(strings.ToLower(s), "https:") {
		u, err := url.Parse(s)
		if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil {
			return "", errors.New("bad https subject")
		}
		return "https:" + s[len("https:"):], nil
	}
	if len(s) >= len("mailto:") && strings.EqualFold(s[:len("mailto:")], "mailto:") {
		s = strings.TrimSpace(s[len("mailto:"):])
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || addr.Name != "" {
		return "", errors.New("bad address subject")
	}
	return s, nil
}
