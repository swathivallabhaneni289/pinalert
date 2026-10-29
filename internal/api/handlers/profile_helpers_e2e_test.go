// profile_helpers_e2e_test.go: the step 4 helper of Phase 7.1 plan 10. Once the
// server requires a username, a name and a home area before first-run-done
// (D-06), every test that finishes the first-time flow fills these in first.
package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pinalert/internal/service"
)

// fillProfileBody is the body fillProfile sends. The home area is a town the
// place search offers, with a point inside it; the server keeps only the name
// and a cell about 5 km across.
func fillProfileBody(username string) string {
	return `{"username":"` + username + `","display_name":"Test Person",` +
		`"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`
}

// fillProfile does what step 4 About you does: it saves a username, a name and
// a home area through PATCH /api/me/profile and fails on anything but 200.
// username must be a free, usable name; the helper checks it against the real
// rules first, so a bad fixture fails here with its own message and not as a
// confusing 400 later. Every account in one test needs its own name.
func fillProfile(t *testing.T, client *http.Client, srv *httptest.Server, username string) {
	t.Helper()
	fillProfileAt(t, client, srv.URL, username)
}

// fillProfileAt is fillProfile for a test that holds only the base address of
// the server (the welcome tests do).
func fillProfileAt(t *testing.T, client *http.Client, base, username string) {
	t.Helper()
	if name, class := service.ClassifyUsername(username); class != service.UsernameUsable || name != username {
		t.Fatalf("fillProfile: %q is not a usable lower case username (normalised %q, class %d)", username, name, class)
	}
	resp := meSend(t, client, http.MethodPatch, base+"/api/me/profile", fillProfileBody(username))
	meWant(t, "fillProfile "+username, resp, http.StatusOK)
}
