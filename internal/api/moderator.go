package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/account"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
)

// requireModerator is the gate of the moderator screen and its API (D-08,
// correction 2, T-04-06, T-04-07).
//
// It does its own account lookup and answers http.NotFound (the same bytes as
// chi's default for an unknown route) to every request that is not from a
// listed moderator: a logged out visitor, an unverified session, a verified
// account that is not on the list, and an API path alike. So the routes cannot
// be discovered. It never redirects and never answers 401.
//
// It sits outside requireVerifiedAccount on purpose, because that gate would
// answer an unverified visitor with a 302 or a 401 and so tell them the path
// exists. Its paths are registered with r.Handle, so a wrong method reaches
// this gate instead of chi's 405 with an Allow header.
//
// allowed holds the normalised addresses parsed once at startup from
// MODERATOR_EMAILS. An empty or nil set admits nobody. The email read from the
// store was normalised at sign in, so a plain key lookup is enough.
//
// On a refusal the gate writes nothing but the not found answer: no header of
// its own, and no log line that holds an email, the list or the session id.
func requireModerator(q *sqlcgen.Queries, allowed map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID, ok := session.FromContext(r.Context())
			if !ok || q == nil || len(allowed) == 0 {
				http.NotFound(w, r)
				return
			}
			row, err := q.GetAccountBySessionID(r.Context(), sessionID)
			if err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					log.Printf("api: requireModerator: looking up account for session: %v", err)
				}
				http.NotFound(w, r)
				return
			}
			if _, listed := allowed[row.Email]; !listed {
				http.NotFound(w, r)
				return
			}
			ctx := account.WithAccount(r.Context(), account.Account{ID: row.ID, Email: row.Email})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
