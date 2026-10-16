// geocode_route_e2e_test.go proves the report form's address search route
// keeps its budget per verified account, not per client address, at the
// router's default of 8 searches at once and then 1 a second (quick
// 261016-sne). newE2EServer leaves Deps.Geocode nil, so every request that
// passes the gate and the budget answers the search unavailable 503, which is
// enough to tell the 503 and the 429 apart without a live geocoder.
package handlers_test

import (
	"net/http"
	"testing"
	"time"
)

const (
	geocodeUnavailable = "Search unavailable, try tapping the map instead."
	searchRateLimited  = "Too many searches. Wait a few seconds and try again."
)

func getGeocode(t *testing.T, client *http.Client, baseURL string) *http.Response {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/geocode?q=Velachery")
	if err != nil {
		t.Fatalf("GET /api/geocode: %v", err)
	}
	return resp
}

// TestGeocodeRouteBudgetPerAccount: one account gets 8 searches, then a 429;
// a second account behind the same client address still gets through; and a
// second later the first account has exactly one more.
func TestGeocodeRouteBudgetPerAccount(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	first := newVerifiedClient(t, srv, mailer, "geocode-budget-a@example.com")
	second := newVerifiedClient(t, srv, mailer, "geocode-budget-b@example.com")

	for i := 1; i <= 8; i++ {
		expectPlaceError(t, getGeocode(t, first, srv.URL), http.StatusServiceUnavailable, "q", geocodeUnavailable)
	}
	expectPlaceError(t, getGeocode(t, first, srv.URL), http.StatusTooManyRequests, "q", searchRateLimited)

	// Same test server, same client IP, different verified account.
	expectPlaceError(t, getGeocode(t, second, srv.URL), http.StatusServiceUnavailable, "q", geocodeUnavailable)

	time.Sleep(1100 * time.Millisecond)
	expectPlaceError(t, getGeocode(t, first, srv.URL), http.StatusServiceUnavailable, "q", geocodeUnavailable)
	expectPlaceError(t, getGeocode(t, first, srv.URL), http.StatusTooManyRequests, "q", searchRateLimited)
}
