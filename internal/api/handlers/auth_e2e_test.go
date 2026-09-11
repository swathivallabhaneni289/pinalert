// Package handlers_test also covers the request-a-magic-link slice: it
// drives the real router (internal/api.NewRouter) over a real HTTP server
// against a real Postgres, with a recording fake Mailer injected so the
// mailed link's token can be hashed and compared against what was actually
// persisted — the same real-router-plus-real-Postgres shape
// reports_e2e_test.go already established for the report submission slice.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/auth"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
)

// recordingMailer captures the most recent SendVerificationLink call so
// TestRequestLinkValidEmailMailsHashMatchingLink can assert the emailed
// link's token hashes to the persisted token_hash — not merely that some
// link was produced.
type recordingMailer struct {
	mu    sync.Mutex
	email string
	link  string
}

func (m *recordingMailer) SendVerificationLink(ctx context.Context, email, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.email = email
	m.link = link
	return nil
}

func (m *recordingMailer) last() (email, link string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.email, m.link
}

func newAuthE2EServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *recordingMailer) {
	t.Helper()
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("auth-e2e-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}

	queries := sqlcgen.New(pool)
	fm := &recordingMailer{}
	deps := api.Deps{
		Session:     mgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries),
		AuthService: service.NewAuthService(queries, fm, "https://pinalert.example"),
		Auth:        handlers.AuthConfig{AssetVersion: "test"},
		Template:    tmpl,
	}
	router := api.NewRouter(deps)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, pool, fm
}

// TestLoginGateServesInteractiveForm asserts GET /login returns 200 and
// contains the email input id — the field is interactive from first paint
// per D-06, so its mere presence in the initial response (not something
// added later by client-side JS) is the load-bearing claim.
func TestLoginGateServesInteractiveForm(t *testing.T) {
	srv, _, _ := newAuthE2EServer(t)

	resp, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /login body: %v", err)
	}
	if !bytes.Contains(body, []byte(`id="login-email"`)) {
		t.Fatalf("GET /login response does not contain the email input")
	}
}

// TestRequestLinkValidEmailMailsHashMatchingLink is this plan's core
// end-to-end claim: a valid POST persists a magic_link_tokens row, and the
// token inside the link the fake Mailer captured hashes (via
// auth.HashToken, the same function the verify path in plan 01.1-02 will
// reuse) to exactly that row's stored token_hash — proving the raw secret
// never touches the database while the emailed link is still genuinely
// usable.
func TestRequestLinkValidEmailMailsHashMatchingLink(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)

	resp, err := http.Post(srv.URL+"/api/auth/request-link", "application/json",
		bytes.NewReader([]byte(`{"email":"visitor@example.com"}`)))
	if err != nil {
		t.Fatalf("POST /api/auth/request-link: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var out struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if out.Email != "visitor@example.com" {
		t.Fatalf("response email = %q, want visitor@example.com", out.Email)
	}

	mailedEmail, link := mailer.last()
	if mailedEmail != "visitor@example.com" {
		t.Fatalf("mailed email = %q, want visitor@example.com", mailedEmail)
	}

	parsedLink, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parsing mailed link %q: %v", link, err)
	}
	token := parsedLink.Query().Get("token")
	if token == "" {
		t.Fatalf("mailed link %q has no token query parameter", link)
	}

	var storedHash string
	err = pool.QueryRow(context.Background(),
		`SELECT token_hash FROM magic_link_tokens WHERE email = $1 ORDER BY created_at DESC LIMIT 1`,
		"visitor@example.com",
	).Scan(&storedHash)
	if err != nil {
		t.Fatalf("querying persisted token_hash: %v", err)
	}

	if auth.HashToken(token) != storedHash {
		t.Fatalf("hash of mailed token (%s) does not equal the persisted token_hash (%s)",
			auth.HashToken(token), storedHash)
	}
	if token == storedHash {
		t.Fatalf("the raw token must never equal the stored hash")
	}
}

// TestRequestLinkMalformedEmailReturns400 asserts the field-named 400 path
// and that no row was written for a malformed address.
func TestRequestLinkMalformedEmailReturns400(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)

	resp, err := http.Post(srv.URL+"/api/auth/request-link", "application/json",
		bytes.NewReader([]byte(`{"email":"not-an-email"}`)))
	if err != nil {
		t.Fatalf("POST /api/auth/request-link: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, b)
	}

	var out struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if out.Error.Field != "email" {
		t.Fatalf("error field = %q, want %q", out.Error.Field, "email")
	}

	if mailedEmail, _ := mailer.last(); mailedEmail != "" {
		t.Fatalf("mailer should never have been called for a malformed address, but saw %q", mailedEmail)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM magic_link_tokens`).Scan(&count); err != nil {
		t.Fatalf("counting magic_link_tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero magic_link_tokens rows after a malformed request, got %d", count)
	}
}
