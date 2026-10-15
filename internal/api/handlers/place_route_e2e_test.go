// place_route_e2e_test.go proves the reader place search route is mounted
// inside the verified account group and that its budget is per account, not
// per client address (HIST-05, D-14, D-16). newE2EServer leaves
// Deps.PlaceSearch nil, so every request that passes the gate and the budget
// answers the search unavailable 503, which is enough to tell the three
// outcomes (401, 503, 429) apart without a live geocoder.
package handlers_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"
)

const (
	placeUnavailable = "Search unavailable. Try again in a moment."
	placeRateLimited = "Too many searches. Wait a few seconds and try again."
)

func getPlace(t *testing.T, client *http.Client, baseURL string) *http.Response {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/geocode/place?q=Velachery")
	if err != nil {
		t.Fatalf("GET /api/geocode/place: %v", err)
	}
	return resp
}

func expectPlaceError(t *testing.T, resp *http.Response, status int, field, message string) {
	t.Helper()
	if resp.StatusCode != status {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected %d, got %d: %s", status, resp.StatusCode, b)
	}
	gotField, gotMessage := decodeErrorResponse(t, resp)
	if gotField != field || gotMessage != message {
		t.Fatalf("error = (%q, %q), want (%q, %q)", gotField, gotMessage, field, message)
	}
}

// TestGeocodePlaceRouteIsGated: an unverified session is refused by the gate,
// a verified one reaches the handler.
func TestGeocodePlaceRouteIsGated(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	anon := &http.Client{Jar: jar}
	resp := getPlace(t, anon, srv.URL)
	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 401 for an unverified session, got %d: %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	verified := newVerifiedClient(t, srv, mailer, "place-gated@example.com")
	expectPlaceError(t, getPlace(t, verified, srv.URL), http.StatusServiceUnavailable, "q", placeUnavailable)
}

// TestGeocodePlaceRouteBudgetPerAccount: the fourth request in a row from one
// account is refused, while a second account behind the same client address
// still gets through, so the budget belongs to the account and not the IP.
func TestGeocodePlaceRouteBudgetPerAccount(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	first := newVerifiedClient(t, srv, mailer, "place-budget-a@example.com")
	second := newVerifiedClient(t, srv, mailer, "place-budget-b@example.com")

	for i := 1; i <= 3; i++ {
		expectPlaceError(t, getPlace(t, first, srv.URL), http.StatusServiceUnavailable, "q", placeUnavailable)
	}
	expectPlaceError(t, getPlace(t, first, srv.URL), http.StatusTooManyRequests, "q", placeRateLimited)

	// Same test server, same client IP, different verified account.
	expectPlaceError(t, getPlace(t, second, srv.URL), http.StatusServiceUnavailable, "q", placeUnavailable)
}
