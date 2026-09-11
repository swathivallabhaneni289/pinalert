package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinalert/internal/account"
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
		`id="app-shell"`, `id="map"`, `id="fab-report"`, `id="view-toggle"`,
		`id="report-list"`, `id="feed-skeleton"`, `id="feed-empty"`, `id="feed-error"`,
		`id="feed-retry"`, `id="report-modal"`, `id="modal-map"`, `id="coord-readout"`,
		`id="location-notice"`, `id="category-grid"`, `id="severity"`,
		`id="severity-readout"`, `id="description"`, `id="shelter-fields"`,
		`id="shelter-capacity-status"`, `id="shelter-headcount"`, `id="form-error"`,
		`id="submit-report"`, `id="cancel-report"`, `id="discard-confirm"`, `id="toast"`,
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
		pageTestAccount.Email,
		`role="menuitem"`,
		`href="/profile"`,
		`method="post"`,
		`action="/auth/logout"`,
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			t.Errorf("body missing account menu contract substring: %s", want)
		}
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
