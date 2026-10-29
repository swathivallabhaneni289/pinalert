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

// Amended 2026-10-29 by plan 05-07 (D-03, UI-SPEC 4e): the quiet row now reads
// Try the demo, Disclaimer, Policy, so the panel text and the panel links gain
// Try the demo and /demo ahead of the two legal pages. Every earlier word and
// link is kept, in the same order.
//
// Amended 2026-10-29 by Phase 7.1 plan 11 (UI-SPEC 1e, departure D11, owner
// 2026-10-27): the separate Low data panel and its hint line are gone. Under the
// card there is one row of four buttons, Try the demo, Low data view, Disclaimer,
// Policy, and with the Low data view on, one status line above the row and the
// second button reads Turn off low data view. The words and links are kept; only
// their order and the removed hint line changed.
const (
	p421OffText = "Try the demo Low data view Disclaimer Policy"
	p421OnText  = "Low data view is on. It opens after you sign in. Try the demo Turn off low data view Disclaimer Policy"
	// p421WaysOpen opens the wrapper of the row (the wrapper carries an id now).
	p421WaysOpen = `<div class="login-ways" id="login-ways">`
)

func TestLoginLowDataPanel(t *testing.T) {
	if handlers.LowDataCookie != "pinalert_low_data" {
		t.Fatalf("LowDataCookie = %q, want pinalert_low_data", handlers.LowDataCookie)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}

	offLinks := []string{"/demo", "/login?low=1", "/disclaimer", "/policy"}
	onLinks := []string{"/demo", "/login?low=0", "/disclaimer", "/policy"}

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
			panel := p421Region(t, body, p421WaysOpen)
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

			// Focus order: the email field, the send button, then the panel.
			// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
			// D-31): the bar on every page is replaced by the laptop strip on Nearby and
			// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
			// every page) is retired as worded; Disclaimer and Policy are reached from the
			// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
			// first-time flow, the login screen and the one reader. The Phase 3.1 and
			// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
			// The login screen has no bar link first; it keeps its own Disclaimer and
			// Policy links in the panel.
			//
			// Amended 2026-10-29 by Phase 7.1 plan 11 (UI-SPEC departure D10): the focus
			// order is the choice control, then the email field, then the card's button,
			// then the four buttons of the row in order and last the globe credit (when
			// the globe is on). Phase 5's 05-UI-SPEC 5c "email field first" is amended:
			// the two choices come before the field.
			p421AssertInOrder(t, "focus order", body,
				`id="login-tab-create"`, `id="login-tab-signin"`, `id="create-email"`, `id="create-submit"`,
				`id="login-try-demo"`, `id="login-low"`, `href="/disclaimer"`, `href="/policy"`)
			if tc.wantText == p421OffText {
				p421AssertInOrder(t, "credit last", body, `href="/policy"`, `id="globe-credit"`)
			}
			// Column order inside the wrapper (the inbox card is gone with the link).
			p421AssertInOrder(t, "column order", body,
				`class="login-column"`, `id="login-form-panel"`, `class="login-ways"`)

			// The sent query is ignored now: no address is ever read from the URL, so
			// the card is the same entry card (D-05).
			if tc.sent {
				if strings.Contains(body, "reader@example.com") || strings.Contains(body, `id="check-inbox"`) {
					t.Errorf("the sent query was read into the page")
				}
				form := body[strings.Index(body, `id="login-form-panel"`):]
				if formTag := form[:strings.Index(form, ">")]; strings.Contains(formTag, " hidden") {
					t.Errorf("the card is hidden: %s", formTag)
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
	if got := p421Text(p421Region(t, body, p421WaysOpen)); got != p421OnText {
		t.Errorf("panel text = %q, want %q", got, p421OnText)
	}

	// Amended 2026-10-29 by Phase 7.1 plan 08: the reader used to open the
	// mailed link by hand and check the verify page. It now signs in with the
	// password (creating the account with a typed code the first time), and
	// the assertion is that the client reaches the pages read below: the
	// remembered Low data choice still sends / to /low and /low answers 200.
	verifySession(t, client, srv, mailer, "waysin-p421-reader@example.com")

	resp, _ = p421Get(t, client, srv.URL+"/")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/low" {
		t.Fatalf("GET / with the remembered choice = %d to %q, want 302 to /low", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp, _ = p421Get(t, client, srv.URL+"/low")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /low = %d, want 200", resp.StatusCode)
	}

	// Amended 2026-10-29 by Phase 7.1 plan 11 (D-10, research Pitfall 16): the
	// reader is signed in here, so /login answers 303 to / instead of drawing the
	// form, after the ?low=0 choice was saved (the deleting cookie below). It used
	// to answer 200 with the form.
	resp, _ = p421Get(t, client, srv.URL+"/login?low=0")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("signed in GET /login?low=0 = %d to %q, want 303 to /", resp.StatusCode, resp.Header.Get("Location"))
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

// TestActivityBackLinkFollowsDeviceDefault is retired.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21 and
// D-02): the icon only back link is removed from My activity; Nearby is one tap
// away in the app bars. What the link carried for the saved reports strip, the
// page that has a report form (the Low data view when it is this device's
// remembered choice), the server now writes as data-resend-page on the page, and
// TestActivityResendPageFollowsDeviceDefault below proves the cookie drives it.
// The Phase 3.1 and Phase 4 hands-on tests are walked on this layout after Phase
// 3.4 is built.
func TestActivityResendPageFollowsDeviceDefault(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "waysin-p421-activity@example.com")

	check := func(label, resendPage string) {
		t.Helper()
		body := getProfile(t, client, srv.URL)
		if !strings.Contains(body, `data-resend-page="`+resendPage+`"`) {
			t.Errorf("%s: the page does not write data-resend-page=%q", label, resendPage)
		}
		if strings.Contains(body, "profile-back-link") {
			t.Errorf("%s: the page still holds the removed back link", label)
		}
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-11
		// and D-02): the Low data view link left the person menu for Settings,
		// About Pinalert, so the Activity menu must not offer it.
		if strings.Contains(p421Region(t, body, `<div id="account-menu"`), `href="/low"`) {
			t.Errorf("%s: the Activity menu still offers the Low data view", label)
		}
	}

	check("no remembered choice", "/")

	// /login?low=1 answers 200 and the jar keeps the cookie it sets.
	resp, _ := p421Get(t, client, srv.URL+"/login?low=1")
	if _, ok := p421LowCookie(resp.Cookies()); !ok {
		t.Fatalf("GET /login?low=1 set no cookie")
	}
	check("low=1", "/low")

	p421Get(t, client, srv.URL+"/login?low=0")
	check("low=0", "/")
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
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-11,
		// D-02 and D-26): the person menu holds Settings and Log out only on
		// every page, and shows no email. The Phase 3.1 and Phase 4 hands-on
		// tests are walked on this layout after Phase 3.4 is built.
		//
		// Amended 2026-10-29 by Phase 9 plan 12 (D-04, owner decision 2026-10-16, UI-SPEC
		// 7c): My profile is the first item of the menu on every page, a link to /me, so
		// the text starts with it. Every banned part is still banned.
		menu := p421Region(t, rec.Body.String(), `<div id="account-menu"`)
		p421AssertInOrder(t, "menu markup", menu, `href="/me"`, `href="/settings"`, `action="/auth/logout"`)
		text := p421Text(menu)
		if text != "My profile Settings Log out" {
			t.Errorf("the menu text = %q, want %q", text, "My profile Settings Log out")
		}
		for _, banned := range []string{"/?full=1", `href="/low"`, `href="/disclaimer"`, `href="/policy"`, `href="/profile"`, "waysin-p421@example.com"} {
			if strings.Contains(menu, banned) {
				t.Errorf("the main page menu still holds %s", banned)
			}
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
		// Same amendment as the main page above. The Low data view's own Open
		// full version button is on the page body, not in the menu.
		// Amended 2026-10-29 by Phase 9 plan 12 (D-04, UI-SPEC 7c): My profile is the
		// first item here too.
		menu := p421Region(t, rec.Body.String(), `<div id="account-menu"`)
		p421AssertInOrder(t, "menu markup", menu, `href="/me"`, `href="/settings"`, `action="/auth/logout"`)
		text := p421Text(menu)
		if text != "My profile Settings Log out" {
			t.Errorf("the menu text = %q, want %q", text, "My profile Settings Log out")
		}
		for _, banned := range []string{`href="/low"`, "Text only, no map", `href="/?full=1"`, `href="/disclaimer"`, `href="/policy"`, `href="/profile"`} {
			if strings.Contains(menu, banned) {
				t.Errorf("the Low data view menu still holds %s", banned)
			}
		}
	})
}
