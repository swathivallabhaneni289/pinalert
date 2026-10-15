package handlers_test

// moderator_e2e_test.go is V-19: the moderator page, its 404 gate and (from
// Tasks 2 and 3) the queue, Find, Keep, Remove and the audit row. Helpers are
// prefixed mod so they cannot collide with another plan's.

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// modBarSentence is the unofficial bar text on the moderator page with the
// default emergency numbers.
const modBarSentence = "Unofficial, not a government service. Emergency? Call 112 (older numbers 100, 101, 108 may also work). Disclaimer"

// modNoFollow stops a client following a redirect, so a 302 is seen as a 302.
func modNoFollow(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// modDo sends one request with the given method. POST, PUT and PATCH send the
// body {} as JSON; the other methods send none.
func modDo(t *testing.T, client *http.Client, method, url string) *http.Response {
	t.Helper()
	var body io.Reader
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("building %s %s: %v", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// modSameAsUnknown sends the same method to path and to unknownPath with the
// same client and requires equal status (404), equal body bytes, equal headers
// once Date and Set-Cookie are dropped, and no Allow, Location or
// WWW-Authenticate header on either (D-08, correction 2, T-04-06).
func modSameAsUnknown(t *testing.T, client *http.Client, baseURL, method, path, unknownPath string) {
	t.Helper()
	read := func(p string) (int, []byte, http.Header) {
		resp := modDo(t, client, method, baseURL+p)
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading %s %s: %v", method, p, err)
		}
		h := resp.Header.Clone()
		h.Del("Date")
		h.Del("Set-Cookie")
		return resp.StatusCode, b, h
	}
	gotStatus, gotBody, gotHeader := read(path)
	wantStatus, wantBody, wantHeader := read(unknownPath)
	if gotStatus != http.StatusNotFound || wantStatus != http.StatusNotFound {
		t.Errorf("%s %s status = %d, unknown path status = %d, want both 404", method, path, gotStatus, wantStatus)
	}
	if !bytes.Equal(gotBody, wantBody) {
		t.Errorf("%s %s body = %q, unknown path body = %q, want equal", method, path, gotBody, wantBody)
	}
	if !reflect.DeepEqual(gotHeader, wantHeader) {
		t.Errorf("%s %s headers = %v, unknown path headers = %v, want equal", method, path, gotHeader, wantHeader)
	}
	for _, h := range []http.Header{gotHeader, wantHeader} {
		for _, name := range []string{"Allow", "Location", "Www-Authenticate"} {
			if v := h.Values(name); len(v) > 0 {
				t.Errorf("%s %s answered a %s header %v", method, path, name, v)
			}
		}
	}
}

// modStrip removes tags from an HTML fragment, unescapes entities and collapses
// whitespace.
var modTagPattern = regexp.MustCompile(`<[^>]*>`)

func modStrip(s string) string {
	s = modTagPattern.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// modRenderPage calls ModeratePage with the given config and request context
// and returns the recorder.
func modRenderPage(t *testing.T, cfg handlers.PageConfig, method string, withAccount bool) *httptest.ResponseRecorder {
	t.Helper()
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page templates: %v", err)
	}
	req := httptest.NewRequest(method, "/moderate", nil)
	if withAccount {
		req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 7, Email: "mod@example.com"}))
	}
	rec := httptest.NewRecorder()
	handlers.ModeratePage(tmpl, cfg)(rec, req)
	return rec
}

func TestModeratorPageRenders(t *testing.T) {
	rec := modRenderPage(t, handlers.PageConfig{AssetVersion: "test"}, http.MethodGet, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	body := rec.Body.String()

	once := func(what, needle string) {
		t.Helper()
		if got := strings.Count(body, needle); got != 1 {
			t.Errorf("%s: %q appears %d times, want exactly 1", what, needle, got)
		}
	}
	once("robots meta", `<meta name="robots" content="noindex">`)
	once("title", `<title>Pinalert: Moderator screen</title>`)
	once("top chrome", `class="top-chrome"`)
	once("h1", `<h1>Moderator screen</h1>`)
	for _, id := range []string{
		`id="mod-refresh"`, `id="mod-refresh-error"`, `id="mod-find-form"`, `id="mod-find"`,
		`id="mod-find-help"`, `id="mod-find-result"`, `<h2 id="mod-h-auto">`, `<h2 id="mod-h-people">`,
		`<h2 id="mod-h-pending">`, `id="toast"`,
	} {
		once("element", id)
	}
	for _, text := range []string{
		"Flagged by the automatic check", "Flagged by people", "Text check not finished",
		"Posted normally, but the text scored near the line. Newest first.",
		"People used Flag this report. Most flags first.",
		"The check couldn't finish. Serious reports in this list are already published. Waiting longest first.",
	} {
		once("heading or line", text)
	}
	if !strings.Contains(body, "mod@example.com") {
		t.Error("the account email is missing from the page")
	}

	start := strings.Index(body, `<aside class="disclaimer-bar"`)
	end := strings.Index(body, "</aside>")
	if start < 0 || end < start {
		t.Fatal("the unofficial bar is missing")
	}
	if got := modStrip(body[start : end+len("</aside>")]); got != modBarSentence {
		t.Errorf("bar text = %q, want %q", got, modBarSentence)
	}
	// The bar comes first in the top chrome, then the account header.
	if bar, header := strings.Index(body, `class="disclaimer-bar"`), strings.Index(body, `class="app-header"`); bar < 0 || header < bar {
		t.Errorf("the bar (%d) must come before the account header (%d)", bar, header)
	}

	for _, banned := range []string{"leaflet", "unpkg", "feed.css", "modal.css", "map.js"} {
		if strings.Contains(strings.ToLower(body), banned) {
			t.Errorf("the page mentions %q, but it loads no map or feed code", banned)
		}
	}

	custom := modRenderPage(t, handlers.PageConfig{Chrome: handlers.Chrome{Numbers: handlers.EmergencyNumbers{Primary: "999"}}}, http.MethodGet, true)
	if !strings.Contains(custom.Body.String(), "Call 999") {
		t.Error("a configured primary number does not reach the bar")
	}

	if rec := modRenderPage(t, handlers.PageConfig{}, http.MethodGet, false); rec.Code != http.StatusInternalServerError {
		t.Errorf("no account in context: status = %d, want 500", rec.Code)
	}
	if rec := modRenderPage(t, handlers.PageConfig{}, http.MethodPost, true); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status = %d, want 405", rec.Code)
	}
}

// modAllowList parses the list the way main.go does.
func modAllowList(t *testing.T, raw string) map[string]struct{} {
	t.Helper()
	set, err := service.ParseModeratorEmails(raw)
	if err != nil {
		t.Fatalf("ParseModeratorEmails(%q): %v", raw, err)
	}
	return set
}

func TestModeratorRoutesIndistinguishableFrom404(t *testing.T) {
	t.Run("empty_list_nobody", func(t *testing.T) {
		srv, mailer, _ := newE2EServerWith(t, nil)
		client := newVerifiedClient(t, srv, mailer, "mod@example.com")
		client.CheckRedirect = modNoFollow
		modSameAsUnknown(t, client, srv.URL, http.MethodGet, "/moderate", "/no-such-page")
	})

	srv, mailer, _ := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.ModeratorEmails = modAllowList(t, "Mod@Example.com")
	})

	anonymous := &http.Client{CheckRedirect: modNoFollow}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	unverified := &http.Client{Jar: jar, CheckRedirect: modNoFollow}
	if resp, err := unverified.Get(srv.URL + "/login"); err != nil {
		t.Fatalf("GET /login: %v", err)
	} else {
		resp.Body.Close()
	}

	reader := newVerifiedClient(t, srv, mailer, "reader@example.com")
	reader.CheckRedirect = modNoFollow

	moderator := newVerifiedClient(t, srv, mailer, "mod@example.com")
	moderator.CheckRedirect = modNoFollow

	all := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	rows := []struct {
		path    string
		methods []string
	}{
		{"/moderate", all},
		{"/api/moderation/queue", all},
		{"/api/moderation/reports/1", []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}},
		{"/api/moderation/reports/1/keep", []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut}},
		{"/api/moderation/reports/1/remove", []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut}},
		{"/api/moderation/reports/abc", []string{http.MethodGet, http.MethodPost}},
	}
	visitors := []struct {
		name   string
		client *http.Client
	}{
		{"anonymous", anonymous},
		{"unverified", unverified},
		{"verified_not_listed", reader},
	}
	for _, v := range visitors {
		for _, row := range rows {
			unknown := "/api/no-such-route"
			if row.path == "/moderate" {
				unknown = "/no-such-page"
			}
			for _, method := range row.methods {
				t.Run(fmt.Sprintf("%s/%s/%s", v.name, method, row.path), func(t *testing.T) {
					modSameAsUnknown(t, v.client, srv.URL, method, row.path, unknown)
				})
			}
		}
	}

	t.Run("moderator_gets_the_page", func(t *testing.T) {
		resp, err := moderator.Get(srv.URL + "/moderate")
		if err != nil {
			t.Fatalf("GET /moderate: %v", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", resp.StatusCode, b)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", cc)
		}
		if !strings.Contains(string(b), "<title>Pinalert: Moderator screen</title>") {
			t.Error("the moderator page title is missing")
		}
	})

	t.Run("moderator_wrong_method_gets_405", func(t *testing.T) {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			resp := modDo(t, moderator, method, srv.URL+"/moderate")
			resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s /moderate status = %d, want 405", method, resp.StatusCode)
			}
		}
	})
}
