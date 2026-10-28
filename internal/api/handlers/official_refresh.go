package handlers

// The external cron's door into the official feed (plan 05-12, D-15, research G1
// layer 1). A free host sleeps when nobody visits, and D-11's phone alerts must
// go out with nobody on the site, so a GitHub Actions schedule calls
// POST /internal/official/refresh every 10 minutes. The call itself wakes the
// host, and the handler runs the refresh while the caller waits.
//
// The route is invisible until it is configured. With no token set, and for every
// method but POST, it answers http.NotFound, the same bytes as an unknown route,
// so a stranger learns nothing (and it is registered with r.Handle, so chi never
// answers 405). A POST must carry "Authorization: Bearer <token>"; the token is
// compared in constant time and a wrong or missing one is a 401. The handler
// reads no request data at all. Both feeds run through the refresher's
// RefreshIfDue, so each feed's minimum gap and its single flight protect NDMA
// whatever the caller does, and the whole run is bounded by RefreshDeadline.
//
// The answer is a small summary of closed words and counts: no URL, no feed text,
// no token. Logs hold the feed name, the counts and the class word, never the
// token or any feed text.
//
// This endpoint carries no swag annotation: it is an operator door, not part of
// the public /api.

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"pinalert/internal/official"
)

const (
	// RefreshDeadline bounds one refresh call. Render's free host lets a request
	// run about a minute.
	RefreshDeadline = 55 * time.Second
	// RefreshTokenMinLen is the shortest OFFICIAL_REFRESH_TOKEN the server accepts.
	RefreshTokenMinLen = 32
)

// bearerPrefix is the exact scheme word the Authorization header must start with.
const bearerPrefix = "Bearer "

// OfficialRefresher runs one pass over every registered official feed, each
// through its minimum gap. *official.Service satisfies it.
type OfficialRefresher interface {
	RefreshAll(ctx context.Context) []official.Result
}

// RefreshSummary is the answer of the refresh endpoint.
type RefreshSummary struct {
	Feeds []RefreshFeedSummary `json:"feeds"`
}

// RefreshFeedSummary is what one feed did. Error is empty or one closed word:
// timeout, status, parse, anomaly or network.
type RefreshFeedSummary struct {
	Feed    string `json:"feed"`
	Skipped bool   `json:"skipped"`
	Items   int    `json:"items"`
	Stored  int    `json:"stored"`
	Error   string `json:"error"`
}

// ValidateRefreshToken checks the configured token at startup. Empty is allowed
// (the route stays a 404); anything shorter than RefreshTokenMinLen is refused
// with a message that never holds the value.
func ValidateRefreshToken(token string) error {
	if token != "" && len(token) < RefreshTokenMinLen {
		return errors.New("OFFICIAL_REFRESH_TOKEN must be at least 32 characters")
	}
	return nil
}

// OfficialRefresh is the handler of POST /internal/official/refresh. token is the
// configured secret; empty keeps the route a 404.
func OfficialRefresh(svc OfficialRefresher, token string) http.Handler {
	return officialRefresh(svc, token, RefreshDeadline)
}

// officialRefresh is OfficialRefresh with the deadline injected, for the tests.
func officialRefresh(svc OfficialRefresher, token string, deadline time.Duration) http.Handler {
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if !refreshAuthorized(r.Header.Get("Authorization"), want) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if svc == nil {
			http.Error(w, "the official feed is not available", http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), deadline)
		defer cancel()
		results := svc.RefreshAll(ctx)

		sum := RefreshSummary{Feeds: make([]RefreshFeedSummary, 0, len(results))}
		for _, res := range results {
			f := RefreshFeedSummary{
				Feed:    string(res.Feed),
				Skipped: res.Skipped,
				Items:   res.ItemsSeen,
				Stored:  res.Stored,
				Error:   refreshWord(res.ErrorClass),
			}
			sum.Feeds = append(sum.Feeds, f)
			log.Printf("official refresh: feed=%s skipped=%t items=%d stored=%d class=%s",
				f.Feed, f.Skipped, f.Items, f.Stored, f.Error)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(sum); err != nil {
			log.Printf("official refresh: writing the answer failed")
		}
	})
}

// refreshAuthorized reports whether the header is exactly "Bearer <token>". Both
// sides are hashed first, so the compare is over equal length byte slices and the
// token's length does not show in the time it takes.
func refreshAuthorized(header string, want [sha256.Size]byte) bool {
	if !strings.HasPrefix(header, bearerPrefix) {
		return false
	}
	got := sha256.Sum256([]byte(header[len(bearerPrefix):]))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// refreshWord keeps the closed set of error words: anything else, which only a
// bug could produce, is reported as network.
func refreshWord(class string) string {
	switch class {
	case "", "timeout", "status", "parse", "anomaly", "network":
		return class
	}
	return "network"
}
