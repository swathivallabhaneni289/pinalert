// people_mute_e2e_test.go proves the Phase 9 plan 19 person mute routes end to
// end over the real router and a real Postgres (D-12): mute, unmute and the
// caller's own list, the uniform 404 for a person the caller may not see, the
// safe undo (review W7), the quiet side of a mute (the muted person learns
// nothing) and the rule that a mute never hides a report. It reuses the plan 07
// helpers by name. Every package level identifier added here carries the p919
// prefix.
package handlers_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/testutil"
)

// p919MuteRows counts the person_mutes rows a muter owns.
func p919MuteRows(t *testing.T, pool *pgxpool.Pool, muterID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM person_mutes WHERE muter_id = $1", muterID).Scan(&n); err != nil {
		t.Fatalf("counting the mutes of %d: %v", muterID, err)
	}
	return n
}

// TestPersonMuteRoutes is V12 (mute part, D-12, review W7): mute a visible person (204),
// the profile says muted, the list names them, a repeat keeps one row, unmute
// clears it; a private person, yourself and a nobody all answer the one uniform
// 404 on mute; unmute is safe for a person who went private, for a well formed
// name that matches nobody, and 404 only for a malformed name; the muted person
// sees nothing of it.
func TestPersonMuteRoutes(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	muter, muterID := p907Person(t, srv, mailer, pool, "p919-muter@example.com", "mute_me", true)
	target, targetID := p907Reporter(t, srv, mailer, pool, "p919-target@example.com", "mute_tgt")

	listed := func(client *http.Client) []string {
		t.Helper()
		_, obj := p907Get(t, client, srv, "/api/me/muted-people", http.StatusOK)
		entries, ok := obj["entries"].([]any)
		if !ok {
			t.Fatalf("entries is not a list: %v", obj)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			row := e.(map[string]any)
			if len(row) != 1 {
				t.Fatalf("an entry carries more than the username: %v", row)
			}
			names = append(names, row["username"].(string))
		}
		return names
	}
	mutedFlag := func(client *http.Client, username string) any {
		t.Helper()
		_, profile := p907Get(t, client, srv, "/api/people/"+username, http.StatusOK)
		v, has := profile["muted"]
		if !has {
			t.Fatalf("the profile of %s has no muted key: %v", username, profile)
		}
		return v
	}

	// Before anything: not muted, empty list.
	if got := mutedFlag(muter, "mute_tgt"); got != false {
		t.Fatalf("muted before muting = %v, want false", got)
	}
	if got := listed(muter); len(got) != 0 {
		t.Fatalf("list before muting = %v, want empty", got)
	}

	// Mute: 204 with no body, the profile says muted, the list names the person.
	if body := p907Post(t, muter, srv, "/api/people/mute_tgt/mute", http.StatusNoContent); body != "" {
		t.Fatalf("mute body = %q, want empty", body)
	}
	if got := mutedFlag(muter, "mute_tgt"); got != true {
		t.Fatalf("muted after muting = %v, want true", got)
	}
	if got := listed(muter); len(got) != 1 || got[0] != "mute_tgt" {
		t.Fatalf("list after muting = %v, want [mute_tgt]", got)
	}

	// A repeat is a 204 and still one row. The mute ended no follow and made none.
	p907Post(t, muter, srv, "/api/people/mute_tgt/mute", http.StatusNoContent)
	if got := p919MuteRows(t, pool, muterID); got != 1 {
		t.Fatalf("rows after a repeat mute = %d, want 1", got)
	}

	// The muted person sees nothing: their own profile has muted false, the
	// muter's profile has muted false, and their list is empty.
	if got := mutedFlag(target, "mute_tgt"); got != false {
		t.Fatalf("the muted person's own profile says muted = %v, want false", got)
	}
	if got := mutedFlag(target, "mute_me"); got != false {
		t.Fatalf("the muter's profile seen by the muted person says muted = %v, want false", got)
	}
	if got := listed(target); len(got) != 0 {
		t.Fatalf("the muted person's own list = %v, want empty", got)
	}
	// The muter's own page is never muted.
	if got := mutedFlag(muter, "mute_me"); got != false {
		t.Fatalf("your own page says muted = %v, want false", got)
	}

	// Unmute: 204, the flag and the list clear, a repeat is still 204.
	p907Post(t, muter, srv, "/api/people/mute_tgt/unmute", http.StatusNoContent)
	if got := mutedFlag(muter, "mute_tgt"); got != false {
		t.Fatalf("muted after unmuting = %v, want false", got)
	}
	if got := listed(muter); len(got) != 0 {
		t.Fatalf("list after unmuting = %v, want empty", got)
	}
	p907Post(t, muter, srv, "/api/people/mute_tgt/unmute", http.StatusNoContent)

	// A person the caller may not see, yourself and a nobody all answer the one
	// uniform 404 body, byte for byte, and mute nothing.
	testutil.SeedPerson(t, pool, "p919-private@example.com", "mute_private", false)
	pendingID := testutil.SeedPerson(t, pool, "p919-pending@example.com", "mute_pending", true)
	testutil.SeedUsername(t, pool, pendingID, "mute_pending", "pending")
	_, wantBody, _ := p907Do(t, muter, http.MethodGet, srv.URL+"/api/people/mute_nobody", "")
	for _, name := range []string{"mute_nobody", "mute_private", "mute_pending", "mute_me"} {
		body := p907Post(t, muter, srv, "/api/people/"+name+"/mute", http.StatusNotFound)
		if body != wantBody {
			t.Fatalf("mute of %s: body %q differs from the uniform 404 %q", name, body, wantBody)
		}
	}
	if got := p919MuteRows(t, pool, muterID); got != 0 {
		t.Fatalf("rows after the refused mutes = %d, want 0", got)
	}

	// A person who went private after being muted can still be unmuted and
	// leaves the list.
	p907Post(t, muter, srv, "/api/people/mute_tgt/mute", http.StatusNoContent)
	if _, err := pool.Exec(context.Background(), "UPDATE accounts SET profile_public = false WHERE id = $1", targetID); err != nil {
		t.Fatalf("making the target private: %v", err)
	}
	if got := listed(muter); len(got) != 1 || got[0] != "mute_tgt" {
		t.Fatalf("the list must keep a person who went private, got %v", got)
	}
	p907Post(t, muter, srv, "/api/people/mute_tgt/unmute", http.StatusNoContent)
	if got := listed(muter); len(got) != 0 {
		t.Fatalf("list after unmuting a private person = %v, want empty", got)
	}
	if got := p919MuteRows(t, pool, muterID); got != 0 {
		t.Fatalf("rows after unmuting a private person = %d, want 0", got)
	}

	// Review W7: unmute of a well formed username that matches nobody is a 204
	// and changes nothing; a malformed username is the uniform 404.
	if _, err := pool.Exec(context.Background(), "UPDATE accounts SET profile_public = true WHERE id = $1", targetID); err != nil {
		t.Fatalf("making the target public again: %v", err)
	}
	p907Post(t, muter, srv, "/api/people/mute_tgt/mute", http.StatusNoContent)
	p907Post(t, muter, srv, "/api/people/mute_nobody/unmute", http.StatusNoContent)
	if got := listed(muter); len(got) != 1 || got[0] != "mute_tgt" {
		t.Fatalf("an unmute of nobody changed the list: %v", got)
	}
	body := p907Post(t, muter, srv, "/api/people/!!/unmute", http.StatusNotFound)
	if body != wantBody {
		t.Fatalf("a malformed unmute: body %q differs from the uniform 404 %q", body, wantBody)
	}

	// A body with content is refused like the other body free people POSTs.
	status, raw, _ := p907Do(t, muter, http.MethodPost, srv.URL+"/api/people/mute_tgt/mute", `{"x":1}`)
	if status != http.StatusBadRequest {
		t.Fatalf("mute with a body: status = %d, want 400, body %s", status, raw)
	}

	// The list is private to its owner: another account never sees it.
	if got := listed(target); len(got) != 0 {
		t.Fatalf("another account reads the list %v", got)
	}
}

// TestMuteKeepsReportsVisible is V12 (D-12, T-09-65): a mute never hides a
// report. After the mute the muted person's live report is still in the feed
// the muter reads, their profile still opens and still lists it.
func TestMuteKeepsReportsVisible(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	muter, _ := p907Person(t, srv, mailer, pool, "p919-keep-muter@example.com", "keep_me", false)
	p907Reporter(t, srv, mailer, pool, "p919-keep-target@example.com", "keep_tgt")
	liveID := testutil.SeedHistoryReport(t, pool, "sess-keep_tgt", "medium", "flood", time.Now().UTC().Add(2*time.Hour))

	const feed = "/api/reports?lat=12.9716&lon=77.5946&radius_km=5&view=all"
	hasLive := func(label string, rows any) {
		t.Helper()
		list, _ := rows.([]any)
		for _, r := range list {
			if row, ok := r.(map[string]any); ok && row["id"] == float64(liveID) {
				return
			}
		}
		t.Fatalf("%s: report %d is missing from %v", label, liveID, rows)
	}
	check := func(stage string) {
		t.Helper()
		_, feedObj := p907Get(t, muter, srv, feed, http.StatusOK)
		hasLive(stage+" feed", feedObj["reports"])
		_, profile := p907Get(t, muter, srv, "/api/people/keep_tgt", http.StatusOK)
		if profile["username"] != "keep_tgt" {
			t.Fatalf("%s: the profile: %v", stage, profile)
		}
		_, page := p907Get(t, muter, srv, "/api/people/keep_tgt/reports", http.StatusOK)
		hasLive(stage+" profile reports", page["rows"])
	}

	check("before the mute")
	p907Post(t, muter, srv, "/api/people/keep_tgt/mute", http.StatusNoContent)
	_, profile := p907Get(t, muter, srv, "/api/people/keep_tgt", http.StatusOK)
	if profile["muted"] != true {
		t.Fatalf("the mute did not take: %v", profile)
	}
	check("after the mute")
}
