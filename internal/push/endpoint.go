// Package push is the transport for phone notifications: the endpoint
// allowlist, the payload envelope, the guarded HTTP client and the webpush-go
// wrapper. It holds no database access, no visibility logic and no wording.
//
// A push endpoint is a capability URL chosen by the browser's push service:
// whoever holds it can send to that device, and the server will make an
// outbound HTTPS request to whatever address the client hands over. So this
// package refuses anything that is not a known push service, which keeps the
// server from ever being pointed at an internal address (SSRF). The allowlist
// hosts other than Apple's are an assumption from widely known service names
// (research A2) to be confirmed from the endpoint_host values seen at the
// owner's device check.
package push

import (
	"encoding/base64"
	"errors"
	"net/netip"
	"net/url"
	"strings"
)

var (
	// ErrUnsupportedService means the endpoint is well formed but its host is
	// not a push service this server talks to.
	ErrUnsupportedService = errors.New("push: this browser's notification service is not supported")
	// ErrBadEndpoint means the endpoint is malformed or breaks a rule such as
	// https only, no user info, port 443 only, no IP literal or a length cap.
	ErrBadEndpoint = errors.New("push: malformed subscription endpoint")
	// ErrBadKeys means the browser's p256dh or auth key has the wrong shape.
	ErrBadKeys = errors.New("push: malformed subscription keys")
)

const (
	// defaultMaxEndpointLen is the longest endpoint accepted.
	defaultMaxEndpointLen = 2048
	// p256dhLen is the byte length of an uncompressed P-256 public key.
	p256dhLen = 65
	// authLen is the byte length of the Web Push auth secret.
	authLen = 16
)

// EndpointPolicy is the allowlist for subscription endpoints. A host in Hosts
// matches exactly; a host in Suffixes matches itself or any host ending in a
// dot plus the suffix, so evil-notify.windows.com never matches
// notify.windows.com.
type EndpointPolicy struct {
	Hosts    []string
	Suffixes []string
	MaxLen   int
}

// DefaultEndpointPolicy is the list of known push services: Google, Mozilla,
// Apple and Windows.
func DefaultEndpointPolicy() EndpointPolicy {
	return EndpointPolicy{
		Hosts:    []string{"fcm.googleapis.com", "updates.push.services.mozilla.com"},
		Suffixes: []string{"push.apple.com", "notify.windows.com"},
		MaxLen:   defaultMaxEndpointLen,
	}
}

// Check validates an endpoint and returns its lower case host. Errors carry
// no part of the endpoint, so a caller can log them freely.
func (p EndpointPolicy) Check(endpoint string) (string, error) {
	maxLen := p.MaxLen
	if maxLen <= 0 {
		maxLen = defaultMaxEndpointLen
	}
	if endpoint == "" || len(endpoint) > maxLen {
		return "", ErrBadEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", ErrBadEndpoint
	}
	if !strings.EqualFold(u.Scheme, "https") || u.User != nil || u.Opaque != "" {
		return "", ErrBadEndpoint
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", ErrBadEndpoint
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return "", ErrBadEndpoint
	}
	if port := u.Port(); port != "" && port != "443" {
		return "", ErrBadEndpoint
	}
	if !p.allows(host) {
		return "", ErrUnsupportedService
	}
	return host, nil
}

func (p EndpointPolicy) allows(host string) bool {
	for _, h := range p.Hosts {
		if host == strings.ToLower(h) {
			return true
		}
	}
	for _, s := range p.Suffixes {
		if onDotBoundary(host, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// onDotBoundary reports whether host is suffix itself or ends in "." plus
// suffix.
func onDotBoundary(host, suffix string) bool {
	return host == suffix || strings.HasSuffix(host, "."+suffix)
}

// IsAppleHost reports whether host is push.apple.com or sits under it. Apple
// endpoints ignore the notification tag and the site cannot silence them, so
// the service layer treats them differently.
func IsAppleHost(host string) bool {
	return onDotBoundary(strings.ToLower(host), "push.apple.com")
}

// CheckKeys checks the shape of the browser's keys: p256dh decodes to 65
// bytes starting 0x04 and auth decodes to 16 bytes. Errors never echo a key.
func CheckKeys(p256dh, auth string) error {
	pub, ok := decodeBase64(p256dh)
	if !ok || len(pub) != p256dhLen || pub[0] != 0x04 {
		return ErrBadKeys
	}
	secret, ok := decodeBase64(auth)
	if !ok || len(secret) != authLen {
		return ErrBadKeys
	}
	return nil
}

// decodeBase64 accepts the encodings webpush-go itself accepts: url safe or
// standard, with or without padding.
func decodeBase64(s string) ([]byte, bool) {
	if s == "" {
		return nil, false
	}
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding, base64.URLEncoding, base64.StdEncoding, base64.RawStdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, true
		}
	}
	return nil, false
}
