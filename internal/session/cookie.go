// Package session issues and verifies Pinalert's anonymous, tamper-resistant
// visitor identity. There is no login: the cookie itself, HMAC-signed
// server-side, is the whole trust boundary — see 01-RESEARCH.md's "Anonymous
// session cookie (HMAC-signed)" and the Security Domain's V3/V6 rows.
package session

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

// cookieName is the single cookie this package ever reads or sets.
const cookieName = "pinalert_session"

// oneYear is the cookie MaxAge: there is no login flow to re-establish
// identity if the cookie expires, so it is deliberately long-lived.
const oneYear = 365 * 24 * time.Hour

// idBytes is the number of raw random bytes drawn from crypto/rand per
// session id, before base64url encoding.
const idBytes = 16

// ctxKey is an unexported struct type (never a string) so a value stored by
// this package can never collide with a context key set by unrelated code.
type ctxKey struct{}

// Manager issues and verifies HMAC-signed anonymous session ids. It holds no
// per-request state and is safe for concurrent use.
type Manager struct {
	secret []byte
	secure bool
}

// NewManager constructs a Manager. It rejects an empty secret rather than
// silently signing with a zero-length key, which would make every signature
// trivially forgeable.
func NewManager(secret []byte, secure bool) (*Manager, error) {
	if len(secret) == 0 {
		return nil, errors.New("session: secret must not be empty")
	}
	return &Manager{secret: secret, secure: secure}, nil
}

// NewID draws idBytes from crypto/rand — the cryptographically secure
// generator, never math/rand, whose output would be predictable from
// observed values and let an attacker guess future ids — and returns a
// base64url-encoded session id.
func (m *Manager) NewID() string {
	raw := make([]byte, idBytes)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand.Read only fails if the OS entropy source is
		// unavailable, which is an unrecoverable environment fault, not a
		// condition calling code can meaningfully handle per-request.
		panic("session: crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// Sign returns id with an HMAC-SHA256 signature appended, separated by ".".
func (m *Manager) Sign(id string) string {
	return id + "." + m.mac(id)
}

// Verify splits signed on the first "." separator, recomputes the MAC over
// the id portion, and compares it against the supplied signature using
// hmac.Equal so the comparison takes the same time regardless of how many
// leading bytes match — a byte-by-byte comparison would leak, through
// response timing, how much of a guessed signature was correct.
func (m *Manager) Verify(signed string) (id string, ok bool) {
	parts := strings.SplitN(signed, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	expected := m.mac(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return "", false
	}
	return parts[0], true
}

func (m *Manager) mac(id string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// Middleware reads the pinalert_session cookie, verifies it, and on any
// failure — absent, malformed, bad signature — issues a fresh id, calls
// persist, and sets the cookie. The resolved id is always available to
// downstream handlers via FromContext, whether it was reused or freshly
// issued.
func (m *Manager) Middleware(persist func(ctx context.Context, id string) error) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, needsCookie := m.resolveID(r)

			if needsCookie {
				if err := persist(r.Context(), id); err != nil {
					// Persistence failure must not block an emergency
					// reporter from being served; log and continue issuing
					// the cookie rather than failing the request.
					log.Printf("session: failed to persist new session %s: %v", id, err)
				}
				http.SetCookie(w, &http.Cookie{
					Name:     cookieName,
					Value:    m.Sign(id),
					Path:     "/",
					MaxAge:   int(oneYear.Seconds()),
					HttpOnly: true,
					Secure:   m.secure,
					SameSite: http.SameSiteLaxMode,
				})
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolveID returns the session id to use for this request and whether a
// new cookie needs to be issued (true for absent, malformed, or
// signature-invalid cookies; false when an existing valid cookie was
// reused).
func (m *Manager) resolveID(r *http.Request) (id string, needsCookie bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return m.NewID(), true
	}
	verifiedID, ok := m.Verify(cookie.Value)
	if !ok {
		return m.NewID(), true
	}
	return verifiedID, false
}

// FromContext returns the session id stored by Middleware, if any.
func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok
}

// ClearCookie writes a cookie that deletes the browser's pinalert_session
// cookie — D-09's explicit log-out action (internal/api/handlers.Logout).
// Every one of Name, Path, HttpOnly, Secure and SameSite must exactly match
// what Middleware issues: a browser treats a cookie differing in Path or
// the Secure flag as a DIFFERENT cookie and quietly keeps the original,
// which is how a logout can appear to work and not (threat T-01-85).
// A negative MaxAge tells the browser to delete it immediately. This
// clears only the browser's copy of the cookie — sessions.account_id is left
// deliberately untouched (DEC-K): that row's binding is what still
// connects reports filed from this session to the account, which is
// exactly what the profile page reads, so unbinding it here would silently
// erase a person's own history from their own profile. Nothing about
// signing, verification, or MaxAge for a still-live session changes; this
// extends the Phase 1 mechanism (D-08/IDENT-05) rather than replacing it.
func (m *Manager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}
