package api

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// WelcomePath is where a not onboarded account is sent. It is a constant, so no
// part of a request can build the redirect target (no open redirect, T-34-01).
const WelcomePath = "/welcome"

// onboarded reports whether the account agreed to the current terms version and
// finished the first-time flow. An older stored version counts as not agreed,
// because agreement is to a text and the text has changed (D-27).
func onboarded(a account.Account) bool {
	return a.TermsVersion == service.CurrentTermsVersion && a.FirstRunDone
}

// noAccountOnce makes the wiring-mistake log line appear once, not per request.
var noAccountOnce sync.Once

// requireOnboarded sends an account that has not agreed to the current terms
// or not finished the first-time flow to WelcomePath, for page requests only.
// It runs after requireVerifiedAccount, so the account is already in the
// context.
//
// When enabled is false it returns next unchanged: the flag is off by default
// (Deps.OnboardingGate), so every existing harness behaves as before.
//
// Three kinds of request always pass through:
//   - Any path under /api/. The offline queue flush and every fetch of the
//     page must never meet a new 4xx or a redirect, so a person who is mid
//     flow, or whose queued report is sent after a terms change, loses nothing
//     (orchestrator decision 5, T-34-02).
//   - The exact WelcomePath, so the redirect cannot loop.
//   - Any method other than GET and HEAD, so a POST such as logout still works.
//
// Existing accounts, the owner's included, have no consent stored and are not
// backfilled: they see the first-time flow once (orchestrator decision 5).
func requireOnboarded(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if (r.Method != http.MethodGet && r.Method != http.MethodHead) ||
				strings.HasPrefix(r.URL.Path, "/api/") ||
				r.URL.Path == WelcomePath {
				next.ServeHTTP(w, r)
				return
			}
			acc, ok := account.FromContext(r.Context())
			if !ok {
				// A wiring mistake: requireVerifiedAccount already refused every
				// unverified caller, so pass through rather than strand anyone.
				noAccountOnce.Do(func() {
					log.Printf("api: requireOnboarded: no account in the request context; mount it after requireVerifiedAccount")
				})
				next.ServeHTTP(w, r)
				return
			}
			if onboarded(acc) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, WelcomePath, http.StatusFound)
		})
	}
}
