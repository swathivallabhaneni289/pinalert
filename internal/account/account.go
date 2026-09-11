// Package account holds the verified-caller identity type and its request
// context accessors.
//
// This is a deliberately dependency-free leaf package (mirrors
// internal/session's and internal/ratelimit's own single-purpose,
// zero-internal-dependency shape). It exists because internal/api's
// requireVerifiedAccount middleware (gate.go) resolves the caller's
// account and internal/api/handlers.Page (plan 01.1-06) needs to read it
// back out of the request context — but internal/api already imports
// internal/api/handlers (router.go, gate.go both reach for
// handlers.LoginGate/handlers.Page/handlers.ErrorResponse etc.), so
// internal/api/handlers cannot import internal/api back without an import
// cycle. Both packages import this one instead; neither imports the other
// for this purpose. internal/api keeps thin Account/AccountFromContext
// re-exports for its own same-package callers (gate.go, gate_test.go), but
// internal/api/handlers and any future handler package must import
// pinalert/internal/account directly.
package account

import "context"

// Account is the identity requireVerifiedAccount resolves for a verified
// session: the account row joined through sessions.account_id, never a
// value the client can assert directly (threat T-01-71).
type Account struct {
	ID    int64
	Email string
}

// ctxKey is an unexported struct type (never a string) so a value stored by
// WithAccount can never collide with a context key set by unrelated code —
// mirrors internal/session.ctxKey's pattern.
type ctxKey struct{}

// WithAccount returns a copy of ctx carrying a. internal/api's
// requireVerifiedAccount middleware is the one production caller; tests
// that need a gated request without standing up the whole router
// (internal/api/handlers/page_test.go) are the other.
func WithAccount(ctx context.Context, a Account) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// FromContext returns the verified account WithAccount stored in the
// request context, if any.
func FromContext(ctx context.Context) (Account, bool) {
	acc, ok := ctx.Value(ctxKey{}).(Account)
	return acc, ok
}
