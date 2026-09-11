package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/api/handlers"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
)

// accountCtxKey is an unexported struct type (never a string) so a value
// stored by requireVerifiedAccount can never collide with a context key set
// by unrelated code — mirrors internal/session.ctxKey's pattern.
type accountCtxKey struct{}

// Account is the identity requireVerifiedAccount resolves for a verified
// session: the account row joined through sessions.account_id, never a
// value the client can assert directly (threat T-01-71).
type Account struct {
	ID    int64
	Email string
}

// AccountFromContext returns the verified account requireVerifiedAccount
// stored in the request context, if any. Plan 01.1-06's profile handler and
// account header read the caller's account this way rather than repeating
// the session-to-account lookup.
func AccountFromContext(ctx context.Context) (Account, bool) {
	acc, ok := ctx.Value(accountCtxKey{}).(Account)
	return acc, ok
}

// unverifiedErrorResponse is written for every gated /api/ request from an
// unverified session — the same ErrorResponse envelope every other error
// this API returns uses, so a browser's fetch layer never has to special-case
// the access gate's refusal.
var unverifiedErrorResponse = handlers.ErrorResponse{
	Error: handlers.ErrorDetail{Field: "auth", Message: "Verify your email to continue."},
}

// requireVerifiedAccount enforces D-05 ("login is required to view
// anything, not merely to act") for every route it wraps. It resolves the
// caller's account per request from sessions.account_id via the signed
// session cookie (session.FromContext) — never from anything the client
// asserts directly (threat T-01-71), continuing Phase 1's
// server-computed-fields pattern (T-01-07).
//
// A session with no bound account (a nil queries handle, a missing session
// id, or a not-found lookup result) is treated as unverified, never as an
// error: internal/session/cookie.go deliberately
// logs and continues when its persist callback fails, so a browser can hold
// a valid signed cookie with no sessions row at all — that state must be
// indistinguishable from "never verified" and must never surface as a 500
// (RESEARCH.md Pattern 2). Any other database error is logged server-side
// and also treated as unverified: the gate fails closed on an
// infrastructure fault rather than opening it (threat T-01-72).
//
// On the unverified path, requireVerifiedAccount branches by surface: an
// /api/ request gets a 401 with the standard ErrorResponse envelope and no
// data; every other request gets a 302 to /login before any handler runs,
// so no app-shell bytes are ever produced (threat T-01-73). The redirect
// carries Cache-Control: no-store so an intermediary cannot cache "this
// visitor is unverified" for a browser that has since verified (threat
// T-01-75).
func requireVerifiedAccount(q *sqlcgen.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID, ok := session.FromContext(r.Context())
			if ok && q != nil {
				row, err := q.GetAccountBySessionID(r.Context(), sessionID)
				switch {
				case err == nil:
					ctx := context.WithValue(r.Context(), accountCtxKey{}, Account{ID: row.ID, Email: row.Email})
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				case errors.Is(err, pgx.ErrNoRows):
					// No account bound to this session yet — unverified,
					// not an error. Fall through to the unverified path.
				default:
					log.Printf("api: requireVerifiedAccount: looking up account for session: %v", err)
				}
			}

			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(unverifiedErrorResponse); err != nil {
					log.Printf("api: requireVerifiedAccount: writing 401 body: %v", err)
				}
				return
			}

			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/login", http.StatusFound)
		})
	}
}
