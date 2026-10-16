package api

// Tests for the Low data view routes (plan 04-18): the page render, the gate
// branch for a logged out visitor, the remembered choice cookie and the
// redirects around it. Every helper here starts with low so it never collides
// with gate_test.go or a later Phase 4 test file in this package.

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/api/handlers"
	"pinalert/internal/session"
	"pinalert/web"
)

// lowBarSentence is the exact default sentence of the unofficial bar (D-22).
const lowBarSentence = "Unofficial, not a government service. Emergency? Call 112 (older numbers 100, 101, 108 may also work). Disclaimer"

var (
	lowTagPattern   = regexp.MustCompile(`<[^>]*>`)
	lowSpacePattern = regexp.MustCompile(`\s+`)
)

// lowRequestWithAccount builds a GET request whose context carries a verified
// account, as the access gate would leave it.
func lowRequestWithAccount(target string, id int64, email string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx := account.WithAccount(req.Context(), account.Account{ID: id, Email: email})
	return req.WithContext(ctx)
}

// lowBarText cuts the disclaimer bar element out of body, strips its tags and
// collapses whitespace.
func lowBarText(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `<aside class="disclaimer-bar"`)
	if start < 0 {
		t.Fatalf("no disclaimer bar in the page")
	}
	end := strings.Index(body[start:], "</aside>")
	if end < 0 {
		t.Fatalf("the disclaimer bar is not closed")
	}
	text := lowTagPattern.ReplaceAllString(body[start:start+end], "")
	return strings.TrimSpace(lowSpacePattern.ReplaceAllString(text, " "))
}

// lowMainTag returns the opening tag of the main element that holds id="app-shell".
func lowMainTag(t *testing.T, body string) string {
	t.Helper()
	idx := strings.Index(body, `id="app-shell"`)
	if idx < 0 {
		t.Fatalf("no app-shell element in the page")
	}
	open := strings.LastIndex(body[:idx], "<")
	closeAt := strings.Index(body[idx:], ">")
	if open < 0 || closeAt < 0 {
		t.Fatalf("the app-shell tag is not well formed")
	}
	return body[open : idx+closeAt+1]
}

// lowCookie returns the named cookie from resp, or nil.
func lowCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// lowGet issues a GET and returns the response with its body read and closed.
func lowGet(t *testing.T, client *http.Client, url string) (*http.Response, string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET %s: %v", url, err)
	}
	return resp, string(b)
}

// lowAllowedLocations is every redirect target this feature may produce (T-04-13).
var lowAllowedLocations = map[string]bool{"/low": true, "/": true, "/login": true, "/login?low=1": true}

func lowCheckLocation(t *testing.T, resp *http.Response) string {
	t.Helper()
	loc := resp.Header.Get("Location")
	if !lowAllowedLocations[loc] {
		t.Errorf("Location %q is not one of the four constants", loc)
	}
	return loc
}

func TestLowPageRenders(t *testing.T) {
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}
	cfg := handlers.PageConfig{FallbackLat: 12.9716, FallbackLon: 77.5946, DefaultRadiusKm: 10, VoteRadiusKm: 1, AssetVersion: "v418"}
	handler := handlers.LowPage(tmpl, cfg)

	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}

	t.Run("full render", func(t *testing.T) {
		rec := serve(lowRequestWithAccount("/low", 42, "reader@example.com"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != "pinalert_low_data" {
			t.Fatalf("cookies = %v, want exactly pinalert_low_data", cookies)
		}
		c := cookies[0]
		if c.Value != "1" || c.Path != "/" || c.MaxAge != 31536000 || c.SameSite != http.SameSiteLaxMode || c.HttpOnly || c.Secure {
			t.Errorf("cookie = %+v", c)
		}

		body := rec.Body.String()
		for _, want := range []string{
			`<title>Pinalert: Low data view</title>`,
			`<h1>Low data view</h1>`,
			`Text only, no map`,
			`This device opens the Low data view by default.`,
			`href="/?full=1"`,
			`Open full version`,
			`<h2 id="low-list-heading">Active reports</h2>`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("body is missing %q", want)
			}
		}
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-11
		// and D-26): the person menu no longer shows the signed in email, so it
		// is on no page (T-34-18). The Phase 3.1 and Phase 4 hands-on tests are
		// walked on this layout after Phase 3.4 is built.
		if strings.Contains(body, `reader@example.com`) {
			t.Errorf("body renders the signed in email")
		}
		if got := strings.Count(body, `class="top-chrome"`); got != 1 {
			t.Errorf("top-chrome appears %d times, want 1", got)
		}
		if got := strings.Count(body, `id="notice-live"`); got != 1 {
			t.Errorf("notice-live appears %d times, want 1", got)
		}
		if got := lowBarText(t, body); got != lowBarSentence {
			t.Errorf("bar = %q, want %q", got, lowBarSentence)
		}

		main := lowMainTag(t, body)
		if !strings.HasPrefix(main, "<main") {
			t.Errorf("app-shell is not on a main element: %s", main)
		}
		for _, want := range []string{
			`class="low-page"`, `data-fallback-lat="12.9716"`, `data-fallback-lon="77.5946"`,
			`data-default-radius-km="10"`, `data-poll-interval-ms="60000"`,
			`data-vote-radius-km="1"`, `data-account-id="42"`,
		} {
			if !strings.Contains(main, want) {
				t.Errorf("main tag is missing %s: %s", want, main)
			}
		}
		if strings.Contains(main, "data-view") {
			t.Errorf("main tag carries data-view: %s", main)
		}
		classAttr := regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(main)
		if classAttr == nil {
			t.Fatalf("main tag has no class attribute")
		}
		for _, tok := range strings.Fields(classAttr[1]) {
			if tok == "app-shell" {
				t.Errorf("main class holds the app-shell token")
			}
		}

		for _, m := range regexp.MustCompile(`(?:href|src)="(/static/[^"]*)"`).FindAllStringSubmatch(body, -1) {
			if !strings.HasSuffix(m[1], "?v=v418") {
				t.Errorf("local asset %q has no ?v=v418", m[1])
			}
		}
		for _, banned := range []string{"unpkg.com", "<img", "/static/js/map.js", "/static/js/modal.js", "/static/js/feed.js", "modal-map.css"} {
			if strings.Contains(body, banned) {
				t.Errorf("body holds %q", banned)
			}
		}
	})

	t.Run("https cookie is Secure", func(t *testing.T) {
		req := lowRequestWithAccount("/low", 42, "reader@example.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := serve(req)
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].Secure {
			t.Errorf("cookies = %v, want one Secure cookie", cookies)
		}
	})

	t.Run("zero config renders the default sentence", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handlers.LowPage(tmpl, handlers.PageConfig{})(rec, lowRequestWithAccount("/low", 7, "a@example.com"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := lowBarText(t, rec.Body.String()); got != lowBarSentence {
			t.Errorf("bar = %q", got)
		}
	})

	t.Run("no account is a 500 without a cookie", func(t *testing.T) {
		rec := serve(httptest.NewRequest(http.MethodGet, "/low", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("a cookie was set on the 500: %v", rec.Result().Cookies())
		}
	})
}

func TestLowDataRoutes(t *testing.T) {
	srv, pool, mailer := newGateTestServer(t)

	t.Run("logged out", func(t *testing.T) {
		client := newGateJarClient(t)
		resp, _ := lowGet(t, client, srv.URL+"/low")
		if resp.StatusCode != http.StatusFound {
			t.Fatalf("GET /low status = %d, want 302", resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "/login?low=1" {
			t.Errorf("GET /low Location = %q, want /login?low=1", loc)
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET /low Cache-Control = %q", got)
		}
		if lowCookie(resp, "pinalert_low_data") != nil {
			t.Errorf("GET /low set the low data cookie for a logged out visitor")
		}

		resp, _ = lowGet(t, client, srv.URL+"/")
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/login" {
			t.Errorf("GET / = %d to %q, want 302 to /login", resp.StatusCode, resp.Header.Get("Location"))
		}
	})

	t.Run("verified", func(t *testing.T) {
		client := newGateJarClient(t)
		const email = "low-reader@example.com"
		gateVerifySession(t, client, srv, mailer, email)

		var id int64
		if err := pool.QueryRow(context.Background(), "SELECT id FROM accounts WHERE email = $1", email).Scan(&id); err != nil {
			t.Fatalf("reading the account id: %v", err)
		}

		resp, body := lowGet(t, client, srv.URL+"/low")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /low status = %d, want 200", resp.StatusCode)
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q", got)
		}
		c := lowCookie(resp, "pinalert_low_data")
		if c == nil || c.Value != "1" || c.Path != "/" || c.MaxAge != 31536000 || c.SameSite != http.SameSiteLaxMode || c.HttpOnly {
			t.Errorf("cookie = %+v", c)
		}
		if !strings.Contains(body, `<title>Pinalert: Low data view</title>`) {
			t.Errorf("body is not the Low data page")
		}
		want := `data-account-id="` + strconv.FormatInt(id, 10) + `"`
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %s", want)
		}
	})
}

func TestLowDataRoutesCookieRedirect(t *testing.T) {
	srv, _, mailer := newGateTestServer(t)

	t.Run("verified with the cookie", func(t *testing.T) {
		client := newGateJarClient(t)
		gateVerifySession(t, client, srv, mailer, "low-redirect@example.com")

		// Serving /low sets the cookie through the jar.
		resp, _ := lowGet(t, client, srv.URL+"/low")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /low status = %d, want 200", resp.StatusCode)
		}

		resp, _ = lowGet(t, client, srv.URL+"/")
		if resp.StatusCode != http.StatusFound || lowCheckLocation(t, resp) != "/low" {
			t.Errorf("bare / with the cookie = %d to %q, want 302 to /low", resp.StatusCode, resp.Header.Get("Location"))
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("bare / redirect Cache-Control = %q", got)
		}

		for _, target := range []string{"/?view=resolved", "/?report=7", "/?resend=abc123"} {
			resp, _ = lowGet(t, client, srv.URL+target)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("GET %s = %d, want 200 (deep links are never redirected)", target, resp.StatusCode)
			}
		}

		resp, _ = lowGet(t, client, srv.URL+"/?full=1")
		if resp.StatusCode != http.StatusFound || lowCheckLocation(t, resp) != "/" {
			t.Errorf("/?full=1 = %d to %q, want 302 to /", resp.StatusCode, resp.Header.Get("Location"))
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("/?full=1 Cache-Control = %q", got)
		}
		cleared := lowCookie(resp, "pinalert_low_data")
		if cleared == nil || cleared.MaxAge >= 0 {
			t.Errorf("/?full=1 cookie = %+v, want a cleared cookie with a negative MaxAge", cleared)
		}

		resp, _ = lowGet(t, client, srv.URL+"/")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET / after the clear = %d, want 200 (no loop)", resp.StatusCode)
		}
	})

	t.Run("verified with another cookie value", func(t *testing.T) {
		client := newGateJarClient(t)
		gateVerifySession(t, client, srv, mailer, "low-value@example.com")
		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatalf("parsing the server URL: %v", err)
		}
		client.Jar.SetCookies(u, []*http.Cookie{{Name: "pinalert_low_data", Value: "0", Path: "/"}})
		resp, _ := lowGet(t, client, srv.URL+"/")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET / with cookie value 0 = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("logged out with the cookie", func(t *testing.T) {
		client := newGateJarClient(t)
		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatalf("parsing the server URL: %v", err)
		}
		client.Jar.SetCookies(u, []*http.Cookie{{Name: "pinalert_low_data", Value: "1", Path: "/"}})
		for _, target := range []string{"/", "/?full=1"} {
			resp, _ := lowGet(t, client, srv.URL+target)
			if resp.StatusCode != http.StatusFound || lowCheckLocation(t, resp) != "/login" {
				t.Errorf("logged out GET %s = %d to %q, want 302 to /login", target, resp.StatusCode, resp.Header.Get("Location"))
			}
		}
	})
}

// lowGunzip decompresses a gzip body.
func lowGunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("opening the gzip body: %v", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("reading the gzip body: %v", err)
	}
	return out
}

// lowCompressionRouter builds the real router with no database behind it. A
// request that carries a validly signed session cookie never reaches the
// persist callback, so none of the routes used here touches the database.
func lowCompressionRouter(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	mgr, err := session.NewManager([]byte("compression-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing the session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing the page template: %v", err)
	}
	cookie := &http.Cookie{Name: "pinalert_session", Value: mgr.Sign(mgr.NewID())}
	return NewRouter(Deps{Session: mgr, Template: tmpl}), cookie
}

func lowServe(h http.Handler, cookie *http.Cookie, path string, acceptGzip bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(cookie)
	if acceptGzip {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCompressionMiddleware(t *testing.T) {
	h, cookie := lowCompressionRouter(t)

	t.Run("compressible types are gzipped", func(t *testing.T) {
		for _, tc := range []struct{ path, file string }{
			{"/static/css/main.css", "static/css/main.css"},
			{"/static/js/app.js", "static/js/app.js"},
			{"/static/icons/user.svg", "static/icons/user.svg"},
			{"/login", ""},
		} {
			rec := lowServe(h, cookie, tc.path, true)
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want 200", tc.path, rec.Code)
				continue
			}
			if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
				t.Errorf("%s: Content-Encoding = %q, want gzip", tc.path, got)
				continue
			}
			if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
				t.Errorf("%s: Vary = %q, want it to name Accept-Encoding", tc.path, rec.Header().Get("Vary"))
			}
			if got := rec.Header().Get("Content-Length"); got != "" {
				t.Errorf("%s: Content-Length = %q on a compressed body", tc.path, got)
			}
			plain := lowGunzip(t, rec.Body.Bytes())
			if tc.file == "" {
				if !strings.Contains(string(plain), "<html") {
					t.Errorf("%s: the unzipped body is not a page", tc.path)
				}
				continue
			}
			want, err := web.StaticFS.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("reading %s: %v", tc.file, err)
			}
			if !bytes.Equal(plain, want) {
				t.Errorf("%s: the unzipped body differs from the embedded file", tc.path)
			}
		}
	})

	t.Run("no gzip without Accept-Encoding", func(t *testing.T) {
		rec := lowServe(h, cookie, "/static/css/main.css", false)
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding = %q, want none", got)
		}
		want, err := web.StaticFS.ReadFile("static/css/main.css")
		if err != nil {
			t.Fatalf("reading main.css: %v", err)
		}
		if !bytes.Equal(rec.Body.Bytes(), want) {
			t.Errorf("the plain body differs from the embedded file")
		}
	})

	t.Run("plain text is never compressed", func(t *testing.T) {
		rec := lowServe(h, cookie, "/no-such-path-p418", true)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding = %q on the 404 page, want none", got)
		}
		if got := rec.Body.String(); got != "404 page not found\n" {
			t.Errorf("404 body = %q", got)
		}
	})

	t.Run("settings and position in the router", func(t *testing.T) {
		want := []string{"text/html", "text/css", "text/javascript", "application/javascript", "application/json", "image/svg+xml"}
		if !reflect.DeepEqual(compressibleContentTypes, want) {
			t.Errorf("compressibleContentTypes = %v, want %v", compressibleContentTypes, want)
		}
		if compressionLevel != 5 {
			t.Errorf("compressionLevel = %d, want 5", compressionLevel)
		}
		raw, err := os.ReadFile("router.go")
		if err != nil {
			t.Fatalf("reading router.go: %v", err)
		}
		var code []string
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				code = append(code, line)
			}
		}
		src := strings.Join(code, "\n")
		if got := strings.Count(src, "middleware.Compress("); got != 1 {
			t.Fatalf("middleware.Compress( appears %d times, want 1", got)
		}
		logAt := strings.Index(src, "r.Use(requestLogger(deps.RequestLog))")
		compressAt := strings.Index(src, "middleware.Compress(")
		sessionAt := strings.Index(src, "deps.Session.Middleware(")
		if logAt < 0 || sessionAt < 0 || !(logAt < compressAt && compressAt < sessionAt) {
			t.Errorf("Compress must sit after the request logger and before the session middleware (log=%d, compress=%d, session=%d)", logAt, compressAt, sessionAt)
		}
	})
}
