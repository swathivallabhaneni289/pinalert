package handlers

// public_status_error_test.go proves review IN-S02 (2026-10-29): the 500
// answers of GET /r/{token}, a failed status read and a failed render, carry the
// same no-store, noindex and no-referrer headers as every other answer of the
// page, so a crawler never caches or indexes the error. It needs no database.

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// failingShareQuerier answers the token read with an error. Every other method
// is the nil embedded interface and is never reached.
type failingShareQuerier struct {
	service.ShareQuerier
}

func (failingShareQuerier) ReportByShareToken(context.Context, string) (sqlcgen.ReportByShareTokenRow, error) {
	return sqlcgen.ReportByShareTokenRow{}, errors.New("database is down")
}

func TestPublicStatusErrorHeaders(t *testing.T) {
	token, err := service.NewShareToken()
	if err != nil {
		t.Fatalf("making a share token: %v", err)
	}
	svc := service.NewShareService(failingShareQuerier{}, nil)
	// A template set without share_status, so a render always fails.
	noPage := template.Must(template.New("other").Parse("x"))

	cases := []struct {
		name, path string
	}{
		{"the status read fails", "/r/" + token},
		{"the render fails", "/r/not-a-token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/r/{token}", PublicStatusPage(noPage, AuthConfig{}, svc, "https://pinalert.example", nil))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("GET %s = %d, want 500", c.path, rec.Code)
			}
			want := map[string]string{
				"Cache-Control":   "no-store",
				"X-Robots-Tag":    "noindex, nofollow",
				"Referrer-Policy": "no-referrer",
			}
			for k, v := range want {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
			if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
				t.Errorf("the 500 set a cookie: %v", got)
			}
		})
	}
}
