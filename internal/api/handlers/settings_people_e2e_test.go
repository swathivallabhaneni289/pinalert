// settings_people_e2e_test.go proves the Phase 9 plan 12 people rows of /settings
// end to end over the real router and a real Postgres (D-04, D-12, UI-SPEC
// Surface 7): the four rows in the owner's order with the right counts, the
// switch state, and the switch saving through PATCH /api/me/visibility. It reuses
// the 09-07 helpers (p907Server, p907Person, p907Do) by name. Every package level
// identifier added here carries the p912 prefix.
package handlers_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"pinalert/internal/testutil"
)

// p912Positions returns where each needle first appears in body, failing the
// test when one is missing, so a caller can check an order.
func p912Positions(t *testing.T, body string, needles []string) []int {
	t.Helper()
	at := make([]int, len(needles))
	for i, n := range needles {
		at[i] = strings.Index(body, n)
		if at[i] == -1 {
			t.Fatalf("/settings lacks %q", n)
		}
	}
	return at
}

// p912Value returns the text of the element with this id, which is a span that
// holds a count.
func p912Value(t *testing.T, body, id string) string {
	t.Helper()
	m := regexp.MustCompile(`id="` + id + `">([^<]*)</span>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("/settings has no count element %q", id)
	}
	return m[1]
}

// TestSettingsPeopleRows is the Settings half of D-04: People and reports shows
// Let others follow me, Following, Followers, Muted reports and Blocked people in
// that order, with the counts of a seeded account that follows two people, has one
// follower and blocked one person, and the switch unchecked by default. There is
// no Someone I follow posts row (D-11), and View my profile is a link to /me.
func TestSettingsPeopleRows(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	me, meID := p907Person(t, srv, mailer, pool, "p912-rows@example.com", "rows_me", false)
	now := time.Now().UTC()

	a := testutil.SeedPublicReporter(t, pool, "p912-rows-a@example.com", "rows_a", "sess-rows-a", now)
	b := testutil.SeedPublicReporter(t, pool, "p912-rows-b@example.com", "rows_b", "sess-rows-b", now)
	fan := testutil.SeedPerson(t, pool, "p912-rows-fan@example.com", "rows_fan", true)
	foe := testutil.SeedPerson(t, pool, "p912-rows-foe@example.com", "rows_foe", true)
	testutil.SeedFollow(t, pool, meID, a, now.Add(-3*time.Hour))
	testutil.SeedFollow(t, pool, meID, b, now.Add(-2*time.Hour))
	testutil.SeedFollow(t, pool, fan, meID, now.Add(-time.Hour))
	testutil.SeedBlock(t, pool, meID, foe)
	// Amended 2026-10-29 by Phase 9 plan 20 (D-12, UI-SPEC Defaults 7): the account
	// also muted one person, so the Muted people row reads 1.
	testutil.SeedMute(t, pool, meID, fan)

	resp, body := getPlacePage(t, me, srv.URL, "/settings")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings status = %d, want 200: %s", resp.StatusCode, body)
	}

	// The order of the five rows.
	order := []string{
		`id="settings-profile-switch"`,
		`id="settings-following-row"`,
		`id="settings-followers-row"`,
		`id="settings-muted-row"`,
		`id="settings-muted-people-row"`,
		`id="settings-blocked-row"`,
	}
	at := p912Positions(t, body, order)
	for i := 1; i < len(at); i++ {
		if at[i] <= at[i-1] {
			t.Errorf("People and reports row %d (%s) is not after row %d (%s)", i, order[i], i-1, order[i-1])
		}
	}
	for _, want := range []string{
		"Your public profile and the lists that go with it.",
		"Let others follow me", "Following", "Followers", "Muted reports", "Muted people", "Blocked people",
		`data-username="rows_me"`,
		`id="settings-profile-off-confirm"`, "Turn off your public profile?",
		`id="people-list-window"`,
		`/static/js/settings-people.js`, `/static/js/people-list.js`, `/static/css/person.css`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings lacks %q", want)
		}
	}

	// The counts.
	for id, want := range map[string]string{
		"settings-following-value": "2",
		"settings-followers-value": "1",
		"settings-blocked-value":   "1",
		"settings-muted-value":     "0",
		// Amended 2026-10-29 by Phase 9 plan 20 (D-12): the Muted people count.
		"settings-muted-people-value": "1",
	} {
		if got := p912Value(t, body, id); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}

	// The switch is a native checkbox with role switch and is off by default.
	sw := regexp.MustCompile(`<input[^>]*id="settings-profile-switch"[^>]*>`).FindString(body)
	if !strings.Contains(sw, `type="checkbox"`) || !strings.Contains(sw, `role="switch"`) {
		t.Errorf("the switch is %q, want a checkbox with role switch", sw)
	}
	if strings.Contains(sw, "checked") {
		t.Errorf("the switch is checked for an account that never turned it on: %q", sw)
	}

	// View my profile is a link to /me, and there is no follow posts switch.
	if !regexp.MustCompile(`<a class="place-row place-row--phone-only" id="settings-view-profile-row" href="/me">`).MatchString(body) {
		t.Errorf("/settings has no View my profile link to /me")
	}
	// Amended 2026-10-29 by Phase 9 plan 20 (D-12, UI-SPEC Defaults 7): "Muted
	// people" is drawn now and left this list; the other two stay banned.
	for _, banned := range []string{"Someone I follow posts", "follow_posts"} {
		if strings.Contains(body, banned) {
			t.Errorf("/settings holds %q", banned)
		}
	}

	// The person menu holds My profile first.
	menu := p421Text(p421Region(t, body, `<div id="account-menu"`))
	if menu != "My profile Settings Log out" {
		t.Errorf("person menu text = %q, want %q", menu, "My profile Settings Log out")
	}
	if !strings.Contains(body, `<a role="menuitem" class="account-menu__item" href="/me">`) {
		t.Errorf("the person menu has no My profile link to /me")
	}
}

// TestSettingsProfileSwitchSaves proves the switch end to end: a refused Name is a
// 422 and the page still shows the switch off; PATCH true then GET /settings shows
// it checked; PATCH false with a follower ends the follow and the page shows 0
// Followers. It also proves that the person's own profile read, which the Settings
// script uses to redraw the counts, answers 200 with the counts while the profile
// is switched off.
func TestSettingsProfileSwitchSaves(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	me, meID := p907Person(t, srv, mailer, pool, "p912-switch@example.com", "switch_me", false)
	checked := regexp.MustCompile(`<input[^>]*id="settings-profile-switch"[^>]*checked`)

	// A Name that cannot be public is refused and the switch stays off.
	if _, err := pool.Exec(t.Context(), "UPDATE accounts SET display_name = 'NDMA Control Room' WHERE id = $1", meID); err != nil {
		t.Fatalf("setting the name: %v", err)
	}
	status, body, _ := p907Do(t, me, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":true}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("a refused Name: status = %d, want 422, body %s", status, body)
	}
	if _, page := getPlacePage(t, me, srv.URL, "/settings"); checked.MatchString(page) {
		t.Fatalf("the switch is checked after a refusal")
	}

	// An ordinary Name turns it on, and Settings draws it checked.
	if _, err := pool.Exec(t.Context(), "UPDATE accounts SET display_name = 'Asha Rao' WHERE id = $1", meID); err != nil {
		t.Fatalf("setting the name: %v", err)
	}
	status, body, _ = p907Do(t, me, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":true}`)
	if status != http.StatusOK || !strings.Contains(body, `"profile_public":true`) {
		t.Fatalf("switching on: status = %d, body %s", status, body)
	}
	_, page := getPlacePage(t, me, srv.URL, "/settings")
	if !checked.MatchString(page) {
		t.Fatalf("the switch is not checked after it was turned on")
	}

	// One person follows; Settings counts one follower.
	fan := testutil.SeedPerson(t, pool, "p912-switch-fan@example.com", "switch_fan", true)
	testutil.SeedFollow(t, pool, fan, meID, time.Now().UTC().Add(-time.Hour))
	_, page = getPlacePage(t, me, srv.URL, "/settings")
	if got := p912Value(t, page, "settings-followers-value"); got != "1" {
		t.Fatalf("Followers before switching off = %q, want 1", got)
	}

	// Off ends the follow; Settings shows the switch off and 0 Followers.
	status, body, _ = p907Do(t, me, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":false}`)
	if status != http.StatusOK || !strings.Contains(body, `"profile_public":false`) {
		t.Fatalf("switching off: status = %d, body %s", status, body)
	}
	if got := p907ActiveFollows(t, pool, meID); got != 0 {
		t.Fatalf("active follows after switching off = %d, want 0", got)
	}
	_, page = getPlacePage(t, me, srv.URL, "/settings")
	if checked.MatchString(page) {
		t.Errorf("the switch is checked after it was turned off")
	}
	if got := p912Value(t, page, "settings-followers-value"); got != "0" {
		t.Errorf("Followers after switching off = %q, want 0", got)
	}

	// The own profile read the script uses to redraw counts works while the
	// profile is off.
	_, own := p907Get(t, me, srv, "/api/people/switch_me", http.StatusOK)
	if own["is_self"] != true || own["followers_count"] != float64(0) {
		t.Errorf("own profile while off = %v, want is_self true and 0 followers", own)
	}
}
