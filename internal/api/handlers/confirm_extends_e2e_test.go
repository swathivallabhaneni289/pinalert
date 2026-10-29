// confirm_extends_e2e_test.go is backlog 999.12: the feed and the report link say
// whether a confirmation from a new spot can still push the fade time back, so
// the map popup can add "unless confirmed again" only when it is true. It runs
// over the real router and a real Postgres, through newE2EServerWith like the
// other e2e tests here, and reuses the helpers of comment_count_e2e_test.go.
package handlers_test

import (
	"net/http"
	"testing"

	"pinalert/internal/testutil"
)

// TestFeedAndFocusCarryConfirmExtends: a fresh report reads confirm_extends true
// on its feed object and its focus object; once its expires_at is at the cap
// (24 hours after posting for a low report) both read false; a report its poster
// resolved reads false on its focus object.
func TestFeedAndFocusCarryConfirmExtends(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a := newVerifiedClient(t, srv, mailer, "q64-extends-a@example.com")

	fresh := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	capped := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	closing := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))

	objects, focus := p32ViewerFeed(t, "fresh", a, srv.URL, fresh)
	if !p810Bool(t, "fresh (feed)", objects[fresh], "confirm_extends") {
		t.Error("a fresh report must answer confirm_extends true on the feed")
	}
	if !p810Bool(t, "fresh (focus)", focus, "confirm_extends") {
		t.Error("a fresh report must answer confirm_extends true on the report link")
	}

	testutil.MustExec(t, pool, `UPDATE reports SET expires_at = created_at + interval '24 hours' WHERE id = $1`, capped)
	objects, focus = p32ViewerFeed(t, "capped", a, srv.URL, capped)
	if p810Bool(t, "capped (feed)", objects[capped], "confirm_extends") {
		t.Error("a report at its cap must answer confirm_extends false on the feed")
	}
	if p810Bool(t, "capped (focus)", focus, "confirm_extends") {
		t.Error("a report at its cap must answer confirm_extends false on the report link")
	}

	resp := postVote(t, a, srv.URL, closing, "resolve", map[string]any{"latitude": p20Lat, "longitude": p20Lon})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve: status %d, want 200", resp.StatusCode)
	}
	_, resolvedFocus := p32ViewerFeed(t, "resolved", a, srv.URL, closing)
	if p810Bool(t, "resolved (focus)", resolvedFocus, "confirm_extends") {
		t.Error("a resolved report must answer confirm_extends false")
	}
}
