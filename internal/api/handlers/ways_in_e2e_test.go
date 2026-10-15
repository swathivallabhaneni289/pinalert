// Plan 04-21 pins the ways in to the Low data view and the logged out links to
// the legal pages: the login page panel (D-17, D-23, P4-A18), the account menu
// items (P4-A11) and the Activity back link that follows the device default
// (P4-A21). Every database test here uses a disposable database through
// testutil.NewTestDB. The tests without a database serve a handler directly.

package handlers_test

import (
	"html"
	"html/template"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/api/handlers"
)

var (
	p421TagRE   = regexp.MustCompile(`<[^>]*>`)
	p421SpaceRE = regexp.MustCompile(`\s+`)
	p421HrefRE  = regexp.MustCompile(`href="([^"]*)"`)
)

// p421Text turns markup into the words a reader sees. Every tag becomes one
// space, never nothing, so two adjacent spans read as two words.
func p421Text(markup string) string {
	s := p421TagRE.ReplaceAllString(markup, " ")
	s = html.UnescapeString(s)
	s = p421SpaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// p421Region returns the markup from open to the first closing div after it.
// It fails unless open occurs exactly once.
func p421Region(t *testing.T, body, open string) string {
	t.Helper()
	if n := strings.Count(body, open); n != 1 {
		t.Fatalf("%q appears %d times, want exactly 1", open, n)
	}
	start := strings.Index(body, open)
	end := strings.Index(body[start:], "</div>")
	if end == -1 {
		t.Fatalf("no closing div after %q", open)
	}
	return body[start : start+end]
}

// p421LowCookie finds the remembered choice cookie among the cookies set.
func p421LowCookie(cookies []*http.Cookie) (*http.Cookie, bool) {
	for _, c := range cookies {
		if c.Name == handlers.LowDataCookie {
			return c, true
		}
	}
	return nil, false
}

// p421Login serves the login gate for GET target, with the remembered choice
// cookie on the request when withCookie is true.
func p421Login(t *testing.T, tmpl *template.Template, target string, withCookie bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if withCookie {
		req.AddCookie(&http.Cookie{Name: handlers.LowDataCookie, Value: "1"})
	}
	rec := httptest.NewRecorder()
	handlers.LoginGate(tmpl, handlers.AuthConfig{AssetVersion: "test"})(rec, req)
	return rec
}

// p421Client is a cookie keeping client that never follows a redirect.
func p421Client(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// p421Get reads and closes the body.
func p421Get(t *testing.T, client *http.Client, url string) (*http.Response, string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s body: %v", url, err)
	}
	return resp, string(b)
}

// p421CheckSet fails unless c is the persistent, Lax, script readable device
// choice cookie for the whole site.
func p421CheckSet(t *testing.T, c *http.Cookie) {
	t.Helper()
	if c.Value != "1" || c.Path != "/" || c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie = %+v, want value 1, Path /, not HttpOnly, SameSite Lax", c)
	}
	year := 360 * 24 * 60 * 60
	persistent := c.MaxAge >= year || (!c.Expires.IsZero() && c.Expires.After(time.Now().Add(time.Duration(year)*time.Second)))
	if !persistent {
		t.Errorf("cookie is not persistent for at least 360 days: %+v", c)
	}
}

// p421CheckCleared fails unless c deletes the device choice cookie.
func p421CheckCleared(t *testing.T, c *http.Cookie) {
	t.Helper()
	if c.MaxAge >= 0 && !(!c.Expires.IsZero() && c.Expires.Before(time.Now())) {
		t.Errorf("cookie = %+v, want a deleting cookie", c)
	}
	if c.Path != "/" {
		t.Errorf("deleting cookie Path = %q, want /", c.Path)
	}
}

// p421AssertInOrder checks that each string occurs after the previous match.
func p421AssertInOrder(t *testing.T, label, body string, parts ...string) {
	t.Helper()
	pos := 0
	for _, p := range parts {
		i := strings.Index(body[pos:], p)
		if i == -1 {
			t.Errorf("%s: %q not found after offset %d", label, p, pos)
			return
		}
		pos += i + len(p)
	}
}

const (
	p421OffText = "Low data view Text only, no map Disclaimer Policy"
	p421OnText  = "Low data view is on for this device. It opens after you sign in. Turn off Disclaimer Policy"
)

func TestLoginLowDataPanel(t *testing.T) {
	if handlers.LowDataCookie != "pinalert_low_data" {
		t.Fatalf("LowDataCookie = %q, want pinalert_low_data", handlers.LowDataCookie)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}

	offLinks := []string{"/login?low=1", "/disclaimer", "/policy"}
	onLinks := []string{"/login?low=0", "/disclaimer", "/policy"}

	cases := []struct {
		name       string
		target     string
		withCookie bool
		wantText   string
		wantLinks  []string
		wantSet    bool
		wantClear  bool
		sent       bool
	}{
		{name: "plain", target: "/login", wantText: p421OffText, wantLinks: offLinks},
		{name: "low_1", target: "/login?low=1", wantText: p421OnText, wantLinks: onLinks, wantSet: true},
		{name: "cookie_only", target: "/login", withCookie: true, wantText: p421OnText, wantLinks: onLinks},
		{name: "low_0_with_cookie", target: "/login?low=0", withCookie: true, wantText: p421OffText, wantLinks: offLinks, wantClear: true},
		{name: "low_invalid", target: "/login?low=yes", wantText: p421OffText, wantLinks: offLinks},
		{name: "sent_and_low_1", target: "/login?sent=reader@example.com&low=1", wantText: p421OnText, wantLinks: onLinks, wantSet: true, sent: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := p421Login(t, tmpl, tc.target, tc.withCookie)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			c, found := p421LowCookie(rec.Result().Cookies())
			switch {
			case tc.wantSet:
				if !found {
					t.Fatalf("no %s cookie set", handlers.LowDataCookie)
				}
				p421CheckSet(t, c)
			case tc.wantClear:
				if !found {
					t.Fatalf("no deleting %s cookie", handlers.LowDataCookie)
				}
				p421CheckCleared(t, c)
			default:
				if found {
					t.Errorf("unexpected Set-Cookie for %s: %+v", handlers.LowDataCookie, c)
				}
			}

			body := rec.Body.String()
			if n := strings.Count(body, `class="login-ways"`); n != 1 {
				t.Fatalf(`class="login-ways" appears %d times, want 1`, n)
			}
			panel := p421Region(t, body, `<div class="login-ways">`)
			if got := p421Text(panel); got != tc.wantText {
				t.Errorf("panel text = %q, want %q", got, tc.wantText)
			}
			var hrefs []string
			for _, m := range p421HrefRE.FindAllStringSubmatch(panel, -1) {
				hrefs = append(hrefs, m[1])
			}
			if strings.Join(hrefs, " ") != strings.Join(tc.wantLinks, " ") {
				t.Errorf("panel hrefs = %q, want %q", hrefs, tc.wantLinks)
			}

			// Focus order: the bar link, the email field, the send button, then the panel.
			p421AssertInOrder(t, "focus order", body,
				`href="/disclaimer"`, `id="login-email"`, `id="request-link"`, `class="login-ways"`)
			// Column order inside the wrapper.
			p421AssertInOrder(t, "column order", body,
				`class="login-column"`, `id="login-form-panel"`, `id="check-inbox"`, `class="login-ways"`)

			if tc.sent {
				open := p421Region(t, body, `<div id="check-inbox"`)
				tag := open[:strings.Index(open, ">")]
				if strings.Contains(tag, " hidden") {
					t.Errorf("check inbox card is hidden in the sent state: %s", tag)
				}
				form := body[strings.Index(body, `id="login-form-panel"`):]
				formTag := form[:strings.Index(form, ">")]
				if !strings.Contains(formTag, " hidden") {
					t.Errorf("login form is not hidden in the sent state: %s", formTag)
				}
			}
			if tc.name == "low_invalid" && strings.Contains(body, "low=yes") {
				t.Errorf("the value of the low query was written into the page")
			}
			if strings.Contains(body, "/moderate") {
				t.Errorf("the login page links to the moderator screen")
			}
		})
	}
}

func TestLoginLowParamSetsCookie(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := p421Client(t)

	resp, _ := p421Get(t, client, srv.URL+"/low")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/login?low=1" {
		t.Fatalf("logged out GET /low = %d to %q, want 302 to /login?low=1", resp.StatusCode, resp.Header.Get("Location"))
	}

	resp, body := p421Get(t, client, srv.URL+"/login?low=1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login?low=1 = %d, want 200", resp.StatusCode)
	}
	c, ok := p421LowCookie(resp.Cookies())
	if !ok {
		t.Fatalf("GET /login?low=1 set no %s cookie", handlers.LowDataCookie)
	}
	p421CheckSet(t, c)
	if got := p421Text(p421Region(t, body, `<div class="login-ways">`)); got != p421OnText {
		t.Errorf("panel text = %q, want %q", got, p421OnText)
	}

	token := requestLink(t, client, srv, mailer, "waysin-p421-reader@example.com")
	status, vbody := getVerify(t, client, srv, token)
	if status != http.StatusOK || !strings.Contains(vbody, "Email verified") || !strings.Contains(vbody, `href="/"`) {
		t.Fatalf("verify = %d, want 200 with Email verified and a Continue link to /: %s", status, vbody)
	}

	resp, _ = p421Get(t, client, srv.URL+"/")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/low" {
		t.Fatalf("GET / with the remembered choice = %d to %q, want 302 to /low", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp, _ = p421Get(t, client, srv.URL+"/low")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /low = %d, want 200", resp.StatusCode)
	}

	resp, _ = p421Get(t, client, srv.URL+"/login?low=0")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login?low=0 = %d, want 200", resp.StatusCode)
	}
	c, ok = p421LowCookie(resp.Cookies())
	if !ok {
		t.Fatalf("GET /login?low=0 set no deleting cookie")
	}
	p421CheckCleared(t, c)

	resp, _ = p421Get(t, client, srv.URL+"/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / after Turn off = %d, want 200", resp.StatusCode)
	}
}

// p421GatedRequest builds a GET request that carries a verified account, the
// way the gate would have left it.
func p421GatedRequest(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := account.WithAccount(req.Context(), account.Account{ID: 421, Email: "waysin-p421@example.com"})
	return req.WithContext(ctx)
}

func TestAccountMenuRendersPerPage(t *testing.T) {
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}
	cfg := handlers.PageConfig{AssetVersion: "test"}

	t.Run("main", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handlers.Page(tmpl, cfg)(rec, p421GatedRequest("/"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		menu := p421Region(t, rec.Body.String(), `<div id="account-menu"`)
		p421AssertInOrder(t, "menu markup", menu,
			`href="/profile"`, `href="/low"`, `href="/disclaimer"`, `href="/policy"`, `action="/auth/logout"`)
		if !strings.Contains(menu, `aria-label="Low data view, Text only, no map"`) {
			t.Errorf("the Low data view item lacks its accessible name")
		}
		text := p421Text(menu)
		p421AssertInOrder(t, "menu text", text,
			"Activity", "Low data view Text only, no map", "Disclaimer", "Policy", "Log out")
		if !strings.HasSuffix(text, "Log out") {
			t.Errorf("the menu text does not end with Log out: %q", text)
		}
		if strings.Contains(menu, "/?full=1") {
			t.Errorf("the main page menu offers Open full version")
		}
		for _, tag := range regexp.MustCompile(`<a [^>]*>`).FindAllString(menu, -1) {
			if !strings.Contains(tag, `role="menuitem"`) {
				t.Errorf("menu link without role=menuitem: %s", tag)
			}
		}
	})

	t.Run("low", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handlers.LowPage(tmpl, cfg)(rec, p421GatedRequest("/low"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		menu := p421Region(t, rec.Body.String(), `<div id="account-menu"`)
		p421AssertInOrder(t, "menu markup", menu,
			`href="/profile"`, `href="/?full=1"`, `href="/disclaimer"`, `href="/policy"`, `action="/auth/logout"`)
		if strings.Contains(menu, `href="/low"`) || strings.Contains(menu, "Text only, no map") {
			t.Errorf("the Low data view menu still offers the Low data view item")
		}
		text := p421Text(menu)
		p421AssertInOrder(t, "menu text", text,
			"Activity", "Open full version", "Disclaimer", "Policy", "Log out")
		if !strings.HasSuffix(text, "Log out") {
			t.Errorf("the menu text does not end with Log out: %q", text)
		}
	})
}
