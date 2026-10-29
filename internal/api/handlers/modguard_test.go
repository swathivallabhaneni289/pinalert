package handlers

// modguard_test.go pins WR-E03 of the Phase 9 code review (2026-10-30): the
// moderator handlers check moderator rights themselves. A request that carries
// a verified account but not the marker only requireModerator sets is answered
// with the plain not found, for every moderator handler and every method.
// No database.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pinalert/internal/account"
)

func TestModeratorHandlersRefuseAVerifiedAccountThatIsNotAModerator(t *testing.T) {
	handlers := map[string]struct {
		h      http.HandlerFunc
		method string
	}{
		"queue":                {ModerationQueue(nil), http.MethodGet},
		"keep":                 {ModerationKeep(nil), http.MethodPost},
		"remove":               {ModerationRemove(nil), http.MethodPost},
		"report":               {ModerationReport(nil), http.MethodGet},
		"comments":             {ModerationComments(nil), http.MethodGet},
		"comment keep":         {ModerationCommentKeep(nil), http.MethodPost},
		"comment remove":       {ModerationCommentRemove(nil), http.MethodPost},
		"ratings list":         {ModerationRatingsList(nil), http.MethodGet},
		"ratings remove":       {ModerationRatingRemove(nil), http.MethodPost},
		"ratings remove rater": {ModerationRaterRemoveAll(nil), http.MethodPost},
	}
	for name, c := range handlers {
		for _, method := range []string{c.method, http.MethodDelete} {
			t.Run(name+" "+method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/x", nil)
				req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 7, Email: "someone@example.com"}))
				rec := httptest.NewRecorder()
				c.h(rec, req)
				if rec.Code != http.StatusNotFound {
					t.Fatalf("status = %d, want 404 (the body %q)", rec.Code, rec.Body.String())
				}
				if rec.Header().Get("Allow") != "" {
					t.Errorf("Allow = %q, want none: a non moderator must not learn the method", rec.Header().Get("Allow"))
				}
			})
		}
	}
}

func TestModGuardAdmitsTheMarkedModerator(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(account.WithModerator(account.WithAccount(req.Context(), account.Account{ID: 7})))
	rec := httptest.NewRecorder()
	acc, ok := modGuard(rec, req, http.MethodGet)
	if !ok || acc.ID != 7 {
		t.Fatalf("modGuard = %+v, %v; want account 7 admitted", acc, ok)
	}
}
