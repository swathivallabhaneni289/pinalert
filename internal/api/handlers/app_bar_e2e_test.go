// app_bar_e2e_test.go: Phase 3.4 plan 03. The two app bars (the laptop top
// bar and the phone bottom bar) are drawn from one Go list of places, so the
// same places reach both sizes under the same names (D-01, D-10, D-12). These
// tests render the real pages through the real router over a real Postgres and
// compare what the two bars say. They need the disposable database the e2e
// harness opens (testutil.NewTestDB); never run them on the owner's data.
package handlers_test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// appBarLink is one anchor of a bar: where it goes, its words and whether it
// is marked as the current place.
type appBarLink struct {
	href, label string
	current     bool
}

// getPageBody GETs path with client and returns the body, failing the test on
// anything but 200.
func getPageBody(t *testing.T, client *http.Client, baseURL, path string) string {
	t.Helper()
	resp, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s body: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200: %s", path, resp.StatusCode, b)
	}
	return string(b)
}

var (
	appBarTopNavRE    = regexp.MustCompile(`(?s)<nav class="app-topbar__nav" aria-label="Main">(.*?)</nav>`)
	appBarTopLinkRE   = regexp.MustCompile(`<a class="app-topbar__link" href="([^"]*)"([^>]*)>([^<]*)</a>`)
	appBarBottomNavRE = regexp.MustCompile(`(?s)<nav class="app-bar" aria-label="Main">(.*?)</nav>`)
	appBarBottomRE    = regexp.MustCompile(`<a class="app-bar__item[^"]*" href="([^"]*)"([^>]*)>.*?<span class="app-bar__label">([^<]*)</span></a>`)
)

// appBarLinks parses the laptop top bar's place links and the phone bottom
// bar's items out of a rendered page.
func appBarLinks(t *testing.T, body string) (top, bottom []appBarLink) {
	t.Helper()
	topNav := appBarTopNavRE.FindStringSubmatch(body)
	if topNav == nil {
		t.Fatalf("page has no top bar nav labelled Main: %s", body)
	}
	for _, m := range appBarTopLinkRE.FindAllStringSubmatch(topNav[1], -1) {
		top = append(top, appBarLink{href: m[1], label: m[3], current: strings.Contains(m[2], `aria-current="page"`)})
	}
	bottomNav := appBarBottomNavRE.FindStringSubmatch(body)
	if bottomNav == nil {
		t.Fatalf("page has no bottom bar nav labelled Main: %s", body)
	}
	for _, m := range appBarBottomRE.FindAllStringSubmatch(bottomNav[1], -1) {
		bottom = append(bottom, appBarLink{href: m[1], label: m[3], current: strings.Contains(m[2], `aria-current="page"`)})
	}
	return top, bottom
}

// TestAppBarRendersSameDestinations proves D-01 and T-34-15: on both pages that
// carry the bars, the three places of the top bar and the first three items of
// the bottom bar have the same addresses and the same words in the same order,
// and the bottom bar adds only Report, which opens the report window.
//
// Amended 2026-11-17 (quick 261117-lq0, owner on the 7.1 UAT: "yes build it"):
// the third place is Profile at /me in both bars, on every page that draws
// them, Settings included.
func TestAppBarRendersSameDestinations(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "app-bar-same@example.com")

	want := []appBarLink{
		{href: "/", label: "Nearby"},
		{href: "/alerts", label: "Official alerts"},
		{href: "/me", label: "Profile"},
	}
	for _, path := range []string{"/", "/profile", "/settings"} {
		body := getPageBody(t, client, srv.URL, path)
		top, bottom := appBarLinks(t, body)
		if len(top) != len(want) {
			t.Fatalf("%s: top bar has %d places, want %d: %+v", path, len(top), len(want), top)
		}
		if len(bottom) != len(want)+1 {
			t.Fatalf("%s: bottom bar has %d items, want %d: %+v", path, len(bottom), len(want)+1, bottom)
		}
		for i, w := range want {
			if top[i].href != w.href || top[i].label != w.label {
				t.Errorf("%s: top bar place %d is %q %q, want %q %q", path, i, top[i].href, top[i].label, w.href, w.label)
			}
			if bottom[i] != top[i] {
				t.Errorf("%s: bottom bar item %d is %+v, the top bar says %+v; both sizes must reach the same places", path, i, bottom[i], top[i])
			}
		}
		report := bottom[len(want)]
		if report.href != "/?post=1" || report.label != "Report" || report.current {
			t.Errorf("%s: last bottom bar item is %+v, want the Report link to /?post=1, never current", path, report)
		}
		if !strings.Contains(body, `<a class="app-topbar__post" href="/?post=1" data-open-report>`) {
			t.Errorf("%s: top bar has no Post a report link to /?post=1", path)
		}
		if strings.Contains(body, "bottom-nav") {
			t.Errorf("%s: the old two tab bottom nav is still rendered", path)
		}
	}
}

// TestAppBarCurrentItem proves the current place is marked in both bars from the
// one list: Nearby on /, My activity on /profile, and exactly one place each.
//
// Amended 2026-11-17 (quick 261117-lq0): My activity is not a main place any
// more, so /profile marks no place current; its phone title stays My activity
// and its back arrow goes to /settings. Nearby on / is unchanged. Profile being
// current on your own profile is pinned by TestAppBarProfileCurrent below.
func TestAppBarCurrentItem(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "app-bar-current@example.com")

	// Amended 2026-11-17 (quick 261117-vuf): Posted left, so a part screen is
	// pinned with Confirmed, and the Updates screen the bell opens is titled
	// Updates with its back arrow to Nearby.
	for _, tc := range []struct{ path, current, title, back string }{
		{"/", "Nearby", "Nearby", ""},
		{"/profile", "", "My activity", "/settings"},
		{"/profile?part=confirmed", "", "Confirmed", "/settings"},
		{"/profile?part=updates", "", "Updates", "/"},
	} {
		body := getPageBody(t, client, srv.URL, tc.path)
		appBarCheckCurrent(t, tc.path, body, tc.current)
		// The phone title is the current place's name, or the page's own words
		// when no place is current.
		title := regexp.MustCompile(`<h1 class="app-topbar__title">([^<]*)</h1>`).FindStringSubmatch(body)
		if title == nil || title[1] != tc.title {
			t.Errorf("%s: phone title is %v, want %q", tc.path, title, tc.title)
		}
		appBarCheckBack(t, tc.path, body, tc.back)
		if strings.Contains(body, `class="app-topbar__gear"`) == false {
			t.Errorf("%s: the phone title bar has no Settings gear", tc.path)
		}
	}
}

// appBarCheckCurrent fails the test unless exactly the place named current is
// marked in both bars, or none when current is empty.
func appBarCheckCurrent(t *testing.T, path, body, current string) {
	t.Helper()
	top, bottom := appBarLinks(t, body)
	for name, links := range map[string][]appBarLink{"top bar": top, "bottom bar": bottom} {
		var got []string
		for _, l := range links {
			if l.current {
				got = append(got, l.label)
			}
		}
		if current == "" {
			if len(got) != 0 {
				t.Errorf("%s %s: current places are %v, want none", path, name, got)
			}
			continue
		}
		if len(got) != 1 || got[0] != current {
			t.Errorf("%s %s: current places are %v, want only %q", path, name, got, current)
		}
	}
}

// appBarCheckBack fails the test unless the phone back arrow goes to back, or
// is absent when back is empty.
func appBarCheckBack(t *testing.T, path, body, back string) {
	t.Helper()
	m := regexp.MustCompile(`<a class="app-topbar__back" href="([^"]*)"`).FindStringSubmatch(body)
	switch {
	case back == "" && m != nil:
		t.Errorf("%s: the phone top bar has a back arrow to %q, want none", path, m[1])
	case back != "" && (m == nil || m[1] != back):
		t.Errorf("%s: the phone back arrow is %v, want %q", path, m, back)
	}
}

// TestAppBarProfileCurrent pins quick 261117-lq0 (owner, 2026-11-17, 7.1 UAT):
// Profile is the current place on your own profile only, with the title My
// profile and no back arrow, like Nearby. /me goes on to your profile. Someone
// else's profile marks no place and keeps the title Profile and the back arrow
// to /, and Settings marks no place.
func TestAppBarProfileCurrent(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	me, _ := p907Person(t, srv, mailer, pool, "lq0-me@example.com", "lq0_me", false)
	p907Person(t, srv, mailer, pool, "lq0-other@example.com", "lq0_other", true)

	resp, err := me.Get(srv.URL + "/me")
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	resp.Body.Close()
	if resp.Request.URL.Path != "/u/lq0_me" {
		t.Errorf("/me ended at %q, want /u/lq0_me", resp.Request.URL.Path)
	}

	for _, tc := range []struct{ path, current, title, back string }{
		{"/u/lq0_me", "Profile", "My profile", ""},
		{"/u/lq0_other", "", "Profile", "/"},
		{"/settings", "", "Settings", "/"},
	} {
		body := getPageBody(t, me, srv.URL, tc.path)
		appBarCheckCurrent(t, tc.path, body, tc.current)
		title := regexp.MustCompile(`<h1 class="app-topbar__title">([^<]*)</h1>`).FindStringSubmatch(body)
		if title == nil || title[1] != tc.title {
			t.Errorf("%s: phone title is %v, want %q", tc.path, title, tc.title)
		}
		appBarCheckBack(t, tc.path, body, tc.back)
	}
}
