// home_centre_e2e_test.go: Phase 3.4 plan 33, task 2, through the real router
// over a real Postgres. An account with a home area gets the centre of its home
// cell as the page start centre (Nearby and the Low data view); an account
// without one gets the configured fallback; and the saved point never reaches
// the page. They need the disposable database the e2e harness opens.
package handlers_test

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	sqlcgen "pinalert/internal/store/sqlc"
)

func homeCentreAttr(t *testing.T, body, name string) float64 {
	t.Helper()
	m := regexp.MustCompile(name + `="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no %s attribute in the page", name)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("%s = %q: %v", name, m[1], err)
	}
	return v
}

func TestPagesStartAtTheStoredHomeAreaCentreEndToEnd(t *testing.T) {
	srv, mailer, _ := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.Page = handlers.PageConfig{FallbackLat: 12.9716, FallbackLon: 77.5946, DefaultRadiusKm: 10}
	})
	client := newJarClient(t)
	verifySession(t, client, srv, mailer, "home-centre-e2e@example.com")

	// No home area yet: the configured centre, no zoom attribute.
	for _, path := range []string{"/?centre=1", "/low"} {
		resp, body := welcomeGet(t, client, srv.URL, path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: %d", path, resp.StatusCode)
		}
		if got := homeCentreAttr(t, body, "data-fallback-lat"); got != 12.9716 {
			t.Errorf("GET %s without a home area: data-fallback-lat = %v, want the configured 12.9716", path, got)
		}
		if strings.Contains(body, "data-fallback-zoom") {
			t.Errorf("GET %s without a home area carries a zoom attribute", path)
		}
	}

	// The person picks a home area. The page then starts at the cell centre.
	const pickedLat, pickedLon = 16.50354321, 80.65461234
	resp := meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"home_area":{"name":"Gayatri Nagar, Vijayawada","lat":16.50354321,"lon":80.65461234}}`)
	meWant(t, "PATCH home area", resp, http.StatusOK)
	wantLat, wantLon := geohash.DecodeCenter(geohash.EncodeWithPrecision(pickedLat, pickedLon, 5))

	for _, path := range []string{"/?centre=1", "/low"} {
		resp, body := welcomeGet(t, client, srv.URL, path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: %d", path, resp.StatusCode)
		}
		lat := homeCentreAttr(t, body, "data-fallback-lat")
		lon := homeCentreAttr(t, body, "data-fallback-lon")
		if diff := lat - wantLat; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("GET %s: data-fallback-lat = %v, want the cell centre %v", path, lat, wantLat)
		}
		if diff := lon - wantLon; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("GET %s: data-fallback-lon = %v, want the cell centre %v", path, lon, wantLon)
		}
		// Nothing finer than the cell centre: the picked point is nowhere in the
		// page, in any spelling the template could print it.
		for _, finer := range []string{"16.50354", "80.65461", "16.5035", "80.6546"} {
			if strings.Contains(body, finer) {
				t.Errorf("GET %s holds %q, a position finer than the home area cell centre", path, finer)
			}
		}
		if path == "/?centre=1" && !strings.Contains(body, `data-fallback-zoom="13"`) {
			t.Errorf("GET / with a home area must start at zoom 13")
		}
		if path == "/low" && strings.Contains(body, "data-fallback-zoom") {
			t.Errorf("the Low data view has no map and carries no zoom")
		}
	}

	// Clearing the home area puts the configured centre back.
	resp = meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile", `{"home_area":null}`)
	meWant(t, "PATCH clear home area", resp, http.StatusOK)
	_, body := welcomeGet(t, client, srv.URL, "/?centre=1")
	if got := homeCentreAttr(t, body, "data-fallback-lat"); got != 12.9716 {
		t.Errorf("after clearing the home area data-fallback-lat = %v, want 12.9716", got)
	}
}
