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
func TestAppBarRendersSameDestinations(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "app-bar-same@example.com")

	want := []appBarLink{
		{href: "/", label: "Nearby"},
		{href: "/alerts", label: "Official alerts"},
		{href: "/profile", label: "My activity"},
	}
	for _, path := range []string{"/", "/profile"} {
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
func TestAppBarCurrentItem(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "app-bar-current@example.com")

	for _, tc := range []struct{ path, current string }{
		{"/", "Nearby"},
		{"/profile", "My activity"},
	} {
		body := getPageBody(t, client, srv.URL, tc.path)
		top, bottom := appBarLinks(t, body)
		for name, links := range map[string][]appBarLink{"top bar": top, "bottom bar": bottom} {
			var current []string
			for _, l := range links {
				if l.current {
					current = append(current, l.label)
				}
			}
			if len(current) != 1 || current[0] != tc.current {
				t.Errorf("%s %s: current places are %v, want only %q", tc.path, name, current, tc.current)
			}
		}
		// The phone title is the current place's name (the Nearby search field
		// and the part screens arrive in later plans).
		title := regexp.MustCompile(`<h1 class="app-topbar__title">([^<]*)</h1>`).FindStringSubmatch(body)
		if title == nil || title[1] != tc.current {
			t.Errorf("%s: phone title is %v, want %q", tc.path, title, tc.current)
		}
		if strings.Contains(body, `class="app-topbar__gear"`) == false {
			t.Errorf("%s: the phone title bar has no Settings gear", tc.path)
		}
	}
}
