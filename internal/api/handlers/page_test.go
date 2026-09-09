package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

	req := httptest.NewRequest(http.MethodGet, "/", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	body := rec.Body.String()

	localAssets := []string{
		"/static/css/main.css", "/static/css/modal.css", "/static/css/feed.css",
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
