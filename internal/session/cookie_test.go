package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"pinalert/internal/session"
)

func testManager(t *testing.T, secret string, secure bool) *session.Manager {
	t.Helper()
	mgr, err := session.NewManager([]byte(secret), secure)
	if err != nil {
		t.Fatalf("NewManager(%q, %v) returned unexpected error: %v", secret, secure, err)
	}
	return mgr
}

// fakePersist records every session id passed to it, so persistence tests
// need no database and run under -short.
type fakePersist struct {
	mu  sync.Mutex
	ids []string
}

func (f *fakePersist) persist(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, id)
	return nil
}

func (f *fakePersist) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.ids))
	copy(out, f.ids)
	return out
}

func newTestServer(t *testing.T, mgr *session.Manager, persist *fakePersist) *httptest.Server {
	t.Helper()
	var handledID string
	var handledOK bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handledID, handledOK = session.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mgr.Middleware(persist.persist)(handler)
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	t.Cleanup(func() {
		_ = handledID
		_ = handledOK
	})
	return srv
}

func TestSessionIssuance(t *testing.T) {
	t.Run("first request with no cookie receives a Set-Cookie and a fresh session id", func(t *testing.T) {
		mgr := testManager(t, "test-secret-one", false)
		persist := &fakePersist{}

		var gotID string
		var gotOK bool
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotID, gotOK = session.FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})
		wrapped := mgr.Middleware(persist.persist)(handler)
		srv := httptest.NewServer(wrapped)
		defer srv.Close()

		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if !gotOK || gotID == "" {
			t.Fatalf("expected a session id in request context, got ok=%v id=%q", gotOK, gotID)
		}

		var setCookie *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "pinalert_session" {
				setCookie = c
			}
		}
		if setCookie == nil {
			t.Fatalf("expected a Set-Cookie for pinalert_session, got none")
		}
		if !setCookie.HttpOnly {
			t.Errorf("expected HttpOnly cookie")
		}
		if setCookie.SameSite != http.SameSiteLaxMode {
			t.Errorf("expected SameSite=Lax, got %v", setCookie.SameSite)
		}
		if setCookie.Path != "/" {
			t.Errorf("expected Path=/, got %q", setCookie.Path)
		}
		if setCookie.MaxAge < 364*24*3600 {
			t.Errorf("expected a ~1 year MaxAge, got %d seconds", setCookie.MaxAge)
		}

		calls := persist.calls()
		if len(calls) != 1 || calls[0] != gotID {
			t.Fatalf("expected persist called once with %q, got %v", gotID, calls)
		}
	})

	t.Run("follow-up request with the issued cookie reuses the same session id and sets no new cookie", func(t *testing.T) {
		mgr := testManager(t, "test-secret-two", false)
		persist := &fakePersist{}

		var gotID string
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotID, _ = session.FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})
		wrapped := mgr.Middleware(persist.persist)(handler)
		srv := httptest.NewServer(wrapped)
		defer srv.Close()

		client := srv.Client()
		jar := &cookieJar{}
		client.Jar = jar

		resp1, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("first GET failed: %v", err)
		}
		resp1.Body.Close()
		firstID := gotID

		resp2, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("second GET failed: %v", err)
		}
		defer resp2.Body.Close()
		secondID := gotID

		if firstID != secondID {
			t.Fatalf("expected same session id across requests, got %q then %q", firstID, secondID)
		}

		for _, c := range resp2.Cookies() {
			if c.Name == "pinalert_session" {
				t.Fatalf("expected no new Set-Cookie on a request carrying a valid cookie")
			}
		}

		if len(persist.calls()) != 1 {
			t.Fatalf("expected persist called exactly once (first visit only), got %v", persist.calls())
		}
	})

	t.Run("a cookie with an altered signature byte is rejected and a new session id is issued", func(t *testing.T) {
		mgr := testManager(t, "test-secret-three", false)
		persist := &fakePersist{}
		srv := newTestServer(t, mgr, persist)

		id := mgr.NewID()
		signed := mgr.Sign(id)
		tampered := tamperLastChar(signed)

		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		req.AddCookie(&http.Cookie{Name: "pinalert_session", Value: tampered})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		var setCookie *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "pinalert_session" {
				setCookie = c
			}
		}
		if setCookie == nil {
			t.Fatalf("expected a fresh Set-Cookie for a tampered signature")
		}
		if strings.Contains(setCookie.Value, id) {
			t.Fatalf("expected the tampered id to never be trusted/reissued, got %q", setCookie.Value)
		}
	})

	t.Run("a cookie signed with a different secret is rejected the same way", func(t *testing.T) {
		mgrA := testManager(t, "secret-A", false)
		mgrB := testManager(t, "secret-B", false)
		persist := &fakePersist{}
		srv := newTestServer(t, mgrB, persist)

		id := mgrA.NewID()
		signed := mgrA.Sign(id)

		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		req.AddCookie(&http.Cookie{Name: "pinalert_session", Value: signed})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		var setCookie *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "pinalert_session" {
				setCookie = c
			}
		}
		if setCookie == nil {
			t.Fatalf("expected a fresh Set-Cookie when the signing secret differs")
		}
	})

	t.Run("malformed cookies are rejected without panicking", func(t *testing.T) {
		mgr := testManager(t, "test-secret-four", false)
		persist := &fakePersist{}
		srv := newTestServer(t, mgr, persist)

		malformed := []string{
			"no-separator-here",
			".",
			"id.",
			".sig",
			"id.sig.extra",
			"",
		}

		for _, val := range malformed {
			t.Run(val, func(t *testing.T) {
				req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
				if val != "" {
					req.AddCookie(&http.Cookie{Name: "pinalert_session", Value: val})
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("GET failed (should not panic): %v", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("expected 200 (handler still runs with a fresh session), got %d", resp.StatusCode)
				}
			})
		}
	})

	t.Run("verification of both a valid and an invalid signature succeed/fail correctly", func(t *testing.T) {
		mgr := testManager(t, "test-secret-five", false)
		id := mgr.NewID()
		signed := mgr.Sign(id)

		gotID, ok := mgr.Verify(signed)
		if !ok || gotID != id {
			t.Fatalf("expected Verify to accept a validly signed id, got ok=%v id=%q", ok, gotID)
		}

		tampered := tamperLastChar(signed)
		_, ok = mgr.Verify(tampered)
		if ok {
			t.Fatalf("expected Verify to reject a tampered signature")
		}
	})

	t.Run("every issued session id is persisted on first sight, even for a visitor who never submits anything", func(t *testing.T) {
		mgr := testManager(t, "test-secret-six", false)
		persist := &fakePersist{}
		srv := newTestServer(t, mgr, persist)

		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		resp.Body.Close()

		if len(persist.calls()) != 1 {
			t.Fatalf("expected exactly one persist call for a first-time visitor, got %v", persist.calls())
		}
	})

	t.Run("constructing a Manager with an empty secret returns an error", func(t *testing.T) {
		if _, err := session.NewManager(nil, false); err == nil {
			t.Fatalf("expected NewManager(nil, ...) to return an error")
		}
		if _, err := session.NewManager([]byte(""), false); err == nil {
			t.Fatalf("expected NewManager(empty, ...) to return an error")
		}
	})
}

// TestClearCookieMatchesIssuedAttributes is threat T-01-85's regression
// guard: it compares ClearCookie's Name/Path/HttpOnly/Secure/SameSite
// against the REAL cookie Middleware issues (never against hard-coded
// literals that could quietly drift out of sync with Middleware's own
// values), and asserts MaxAge is negative — a browser only actually
// deletes a cookie whose every other attribute matches the one it's
// holding.
func TestClearCookieMatchesIssuedAttributes(t *testing.T) {
	mgr := testManager(t, "test-secret-clear", true)
	persist := &fakePersist{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mgr.Middleware(persist.persist)(handler)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var issued *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "pinalert_session" {
			issued = c
		}
	}
	if issued == nil {
		t.Fatalf("expected an issued Set-Cookie to compare ClearCookie against")
	}

	rec := httptest.NewRecorder()
	mgr.ClearCookie(rec)

	var cleared *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pinalert_session" {
			cleared = c
		}
	}
	if cleared == nil {
		t.Fatalf("ClearCookie did not set a pinalert_session cookie")
	}

	if cleared.Name != issued.Name {
		t.Errorf("Name = %q, want %q (matching the issued cookie)", cleared.Name, issued.Name)
	}
	if cleared.Path != issued.Path {
		t.Errorf("Path = %q, want %q (matching the issued cookie)", cleared.Path, issued.Path)
	}
	if cleared.HttpOnly != issued.HttpOnly {
		t.Errorf("HttpOnly = %v, want %v (matching the issued cookie)", cleared.HttpOnly, issued.HttpOnly)
	}
	if cleared.Secure != issued.Secure {
		t.Errorf("Secure = %v, want %v (matching the issued cookie)", cleared.Secure, issued.Secure)
	}
	if cleared.SameSite != issued.SameSite {
		t.Errorf("SameSite = %v, want %v (matching the issued cookie)", cleared.SameSite, issued.SameSite)
	}
	if cleared.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative (browser must delete the cookie immediately)", cleared.MaxAge)
	}
}

func tamperLastChar(s string) string {
	if s == "" {
		return "x"
	}
	b := []byte(s)
	last := b[len(b)-1]
	if last == 'a' {
		b[len(b)-1] = 'b'
	} else {
		b[len(b)-1] = 'a'
	}
	return string(b)
}

// cookieJar is a minimal http.CookieJar so the "reuse the same session"
// sub-test can carry the Set-Cookie from the first response to the second
// request without pulling in a third-party cookie-jar dependency.
type cookieJar struct {
	mu      sync.Mutex
	cookies []*http.Cookie
}

func (j *cookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cookies = append(j.cookies, cookies...)
}

func (j *cookieJar) Cookies(_ *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]*http.Cookie, len(j.cookies))
	copy(out, j.cookies)
	return out
}
