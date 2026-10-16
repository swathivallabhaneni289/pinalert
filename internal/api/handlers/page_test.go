package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// pageTestAccount is the verified account seeded into every page_test.go
// request's context below. Page is only ever reached through
// internal/api's gated r.Group (requireVerifiedAccount), so a bare
// httptest request has no gate in front of it to populate this itself
// (DEC-P) — these tests do so directly via account.WithAccount, the same
// exported constructor internal/api's gate.go calls in production.
var pageTestAccount = account.Account{ID: 1, Email: "verified@example.com"}

// newPageTestRequest builds a GET / request carrying pageTestAccount in
// its context, so Page's account.FromContext lookup succeeds exactly as it
// would behind the real gate.
func newPageTestRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := account.WithAccount(req.Context(), pageTestAccount)
	return req.WithContext(ctx)
}

// TestPageShellServesDOMContract renders the app shell through httptest and
// asserts every DOM hook the remaining frontend plans (01-05, 01-06) depend
// on is present, plus the stylesheet/script contract's presence and order.
// This test is what stops a later plan from quietly removing a hook its
// sibling depends on.
func TestPageShellServesDOMContract(t *testing.T) {
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}

	handler := Page(tmpl, PageConfig{
		FallbackLat:     12.9716,
		FallbackLon:     77.5946,
		DefaultRadiusKm: 10,
	})

	req := newPageTestRequest()
	rec := httptest.NewRecorder()
	handler(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want it to contain %q", ct, "text/html")
	}

	body := rec.Body.String()

	wantIDs := []string{
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10):
		// id fab-report left the list with the floating report button.
		`id="app-shell"`, `id="map"`, `id="view-toggle"`,
		`id="report-list"`, `id="feed-skeleton"`, `id="feed-empty"`, `id="feed-error"`,
		`id="feed-retry"`, `id="report-modal"`, `id="modal-map"`, `id="coord-readout"`,
		`id="location-notice"`, `id="location-search"`, `id="location-search-results"`,
		`id="location-search-status"`, `id="category-grid"`, `id="severity"`,
		`id="severity-readout"`, `id="description"`, `id="shelter-fields"`,
		`id="shelter-capacity-status"`, `id="shelter-headcount"`, `id="form-error"`,
		`id="submit-report"`, `id="cancel-report"`, `id="discard-confirm"`, `id="toast"`,
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10):
		// the two app bars are part of the page contract (the top bar with the
		// Post a report link, the phone bottom bar with the Report link). The
		// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
		// Phase 3.4 is built.
		`class="app-topbar"`, `class="app-bar"`, `class="app-topbar__post" href="/?post=1" data-open-report`,
	}
	for _, want := range wantIDs {
		if !strings.Contains(body, want) {
			t.Errorf("body missing contract id: %s", want)
		}
	}

	wantStylesheets := []string{
		`/static/css/main.css`, `/static/css/modal.css`, `/static/css/feed.css`,
	}
	for _, want := range wantStylesheets {
		if !strings.Contains(body, want) {
			t.Errorf("body missing stylesheet link: %s", want)
		}
	}

	if !strings.Contains(body, "leaflet@1.9.4") {
		t.Errorf("body missing pinned Leaflet 1.9.4 reference")
	}
	if !strings.Contains(body, "integrity=") {
		t.Errorf("body missing Leaflet integrity hash")
	}

	appIdx := strings.Index(body, "/static/js/app.js")
	mapIdx := strings.Index(body, "/static/js/map.js")
	modalIdx := strings.Index(body, "/static/js/modal.js")
	feedIdx := strings.Index(body, "/static/js/feed.js")
	if appIdx == -1 || mapIdx == -1 || modalIdx == -1 || feedIdx == -1 {
		t.Fatalf("missing one or more script tags: app=%d map=%d modal=%d feed=%d", appIdx, mapIdx, modalIdx, feedIdx)
	}
	if !(appIdx < mapIdx && mapIdx < modalIdx && modalIdx < feedIdx) {
		t.Errorf("script tags out of order: app=%d map=%d modal=%d feed=%d (app.js must load first)", appIdx, mapIdx, modalIdx, feedIdx)
	}
}

// TestPageShellAppliesAssetVersionToLocalStaticAssets is 01-15's regression
// guard for the caching bug that let a human tester keep seeing pre-fix CSS
// for most of a live UAT session: PageConfig.AssetVersion must actually
// reach every local static asset URL as a "?v=" query string (see
// PageConfig's doc comment for why), and must NOT be applied to the
// third-party CDN tags, which are already pinned/SRI-hashed and would only
// gain a meaningless, cache-defeating query string from this mechanism.
func TestPageShellAppliesAssetVersionToLocalStaticAssets(t *testing.T) {
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}

	const version = "test-version-12345"
	handler := Page(tmpl, PageConfig{
		FallbackLat:     12.9716,
		FallbackLon:     77.5946,
		DefaultRadiusKm: 10,
		AssetVersion:    version,
	})

	req := newPageTestRequest()
	rec := httptest.NewRecorder()
	handler(rec, req)
	body := rec.Body.String()

	localAssets := []string{
		"/static/css/main.css", "/static/css/modal.css", "/static/css/feed.css",
		"/static/css/auth.css",
		"/static/js/app.js", "/static/js/map.js", "/static/js/modal.js", "/static/js/feed.js",
		"/static/js/account-menu.js",
	}
	for _, asset := range localAssets {
		want := asset + "?v=" + version
		if !strings.Contains(body, want) {
			t.Errorf("body missing versioned local asset URL %q — AssetVersion did not reach the template for this tag", want)
		}
	}

	if strings.Contains(body, "unpkg.com") && strings.Contains(body, "unpkg.com/leaflet@1.9.4/dist/leaflet.js?v=") {
		t.Errorf("third-party CDN script tag was versioned — only local static assets should carry ?v=")
	}
}

// TestPageShellRendersAccountMenu is plan 01.1-06's core claim (D-12): the
// app shell includes the shared account header partial, its trigger
// carries the accessible-name/state contract UI-SPEC item 14 requires, its
// panel is a real role="menu" that ships hidden, the verified caller's
// email actually reaches the panel, both menu items are present, and the
// response is no longer cacheable (DEC-Q).
func TestPageShellRendersAccountMenu(t *testing.T) {
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}

	handler := Page(tmpl, PageConfig{
		FallbackLat:     12.9716,
		FallbackLon:     77.5946,
		DefaultRadiusKm: 10,
	})

	req := newPageTestRequest()
	rec := httptest.NewRecorder()
	handler(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want %q (DEC-Q — the shell now renders a personal email address)", cc, "no-store")
	}

	body := rec.Body.String()

	wantSubstrings := []string{
		`id="account-menu-trigger"`,
		`aria-label="Account menu"`,
		`aria-haspopup="menu"`,
		`aria-expanded="false"`,
		`aria-controls="account-menu"`,
		`id="account-menu"`,
		`role="menu"`,
		`hidden`,
		`role="menuitem"`,
		`href="/settings"`,
		`method="post"`,
		`action="/auth/logout"`,
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			t.Errorf("body missing account menu contract substring: %s", want)
		}
	}
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-11 and
	// D-26): the person menu holds Settings and Log out only, so the signed in
	// email is no longer on the page (UI-SPEC 1c). The Phase 3.1 and Phase 4
	// hands-on tests are walked on this layout after Phase 3.4 is built.
	if strings.Contains(body, pageTestAccount.Email) {
		t.Errorf("the page still renders the signed in email %q", pageTestAccount.Email)
	}
}

// TestPageShellEscapesAccountEmail is threat T-01-88's regression guard:
// the verified email is attacker-chosen text that has never before reached
// the DOM anywhere in this app, and it must survive html/template's
// contextual auto-escaping rather than being interpolated as raw markup.
func TestPageShellEscapesAccountEmail(t *testing.T) {
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}

	handler := Page(tmpl, PageConfig{
		FallbackLat:     12.9716,
		FallbackLon:     77.5946,
		DefaultRadiusKm: 10,
	})

	const maliciousEmail = `"><script>alert(1)</script>@example.com`
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := account.WithAccount(req.Context(), account.Account{ID: 2, Email: maliciousEmail})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("account email rendered unescaped into the response body — stored XSS (T-01-88): %s", body)
	}
}

// p18AppShellTag returns the opening tag of the #app-shell element, from the
// last "<" before id="app-shell" to the next ">". It fails the test when the
// shell is missing from the body.
func p18AppShellTag(t *testing.T, body string) string {
	t.Helper()
	at := strings.Index(body, `id="app-shell"`)
	if at == -1 {
		t.Fatalf(`body has no id="app-shell"`)
	}
	start := strings.LastIndex(body[:at], "<")
	end := strings.Index(body[at:], ">")
	if start == -1 || end == -1 {
		t.Fatalf("could not bound the app-shell opening tag")
	}
	return body[start : at+end+1]
}

// TestPageShellCarriesVoteRadius is V-34, server half (D-22): the shell tells
// the browser the server's vote radius, taken from the one constant CastVote
// enforces, never from a number typed into the template or the wiring.
func TestPageShellCarriesVoteRadius(t *testing.T) {
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}

	render := func(radius float64) string {
		handler := Page(tmpl, PageConfig{
			FallbackLat:     12.9716,
			FallbackLon:     77.5946,
			DefaultRadiusKm: 10,
			VoteRadiusKm:    radius,
		})
		rec := httptest.NewRecorder()
		handler(rec, newPageTestRequest())
		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
		}
		return p18AppShellTag(t, rec.Body.String())
	}

	// (a) the constant reaches the shell.
	want := `data-vote-radius-km="` + strconv.FormatFloat(service.MaxVoteDistanceKm, 'f', -1, 64) + `"`
	if tag := render(service.MaxVoteDistanceKm); !strings.Contains(tag, want) {
		t.Errorf("app-shell tag %q does not contain %q", tag, want)
	}

	// (b) the attribute follows the config, so it is not a literal in the template.
	if tag := render(2.5); !strings.Contains(tag, `data-vote-radius-km="2.5"`) {
		t.Errorf("app-shell tag %q does not follow the configured radius 2.5", tag)
	}

	// (c) cmd/server/main.go wires the service constant and types no number.
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "cmd", "server", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/server/main.go: %v", err)
	}
	wired := regexp.MustCompile(`(?m)^\s*VoteRadiusKm:\s+service\.MaxVoteDistanceKm,\s*$`)
	if n := len(wired.FindAll(src, -1)); n != 1 {
		t.Errorf("cmd/server/main.go has %d lines wiring VoteRadiusKm to service.MaxVoteDistanceKm, want exactly 1", n)
	}
	typed := regexp.MustCompile(`VoteRadiusKm:\s*[0-9]`)
	if n := len(typed.FindAll(src, -1)); n != 0 {
		t.Errorf("cmd/server/main.go types a number into VoteRadiusKm %d times, want 0", n)
	}
}

// TestNavForMarksOnlyTheCurrentPlace pins the one list of main places (Phase
// 3.4, D-10 and D-12): navFor copies mainNavItems, so a call never changes the
// list a later call reads, marks the matching place current and nothing else,
// and Settings (which is not a main place) marks none and hides the gear.
func TestNavForMarksOnlyTheCurrentPlace(t *testing.T) {
	before := make([]NavItem, len(mainNavItems))
	copy(before, mainNavItems)

	for _, tc := range []struct {
		current, title string
		wantCurrent    int
	}{
		{NavNearby, "Nearby", 0},
		{NavAlerts, "Official alerts", 1},
		{NavActivity, "My activity", 2},
	} {
		v := navFor(tc.current)
		if len(v.Items) != 3 {
			t.Fatalf("navFor(%q) has %d items, want 3", tc.current, len(v.Items))
		}
		for i, item := range v.Items {
			if item.Current != (i == tc.wantCurrent) {
				t.Errorf("navFor(%q) item %q current = %v", tc.current, item.Label, item.Current)
			}
		}
		if v.Title != tc.title {
			t.Errorf("navFor(%q) title = %q, want %q", tc.current, v.Title, tc.title)
		}
		if !v.ShowGear() {
			t.Errorf("navFor(%q) hides the gear", tc.current)
		}
	}

	settings := navFor(NavSettings, navTitle("Settings"), navBack("/"))
	for _, item := range settings.Items {
		if item.Current {
			t.Errorf("Settings marks %q current; it is not a main place", item.Label)
		}
	}
	if settings.Title != "Settings" || settings.BackHref != "/" || settings.ShowGear() {
		t.Errorf("Settings view = %+v, want title Settings, back to /, no gear", settings)
	}

	for i := range mainNavItems {
		if mainNavItems[i] != before[i] {
			t.Errorf("navFor changed mainNavItems[%d] to %+v", i, mainNavItems[i])
		}
	}
}
