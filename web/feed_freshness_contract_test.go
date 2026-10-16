// feed_freshness_contract_test.go proves two UAT-gap-closure guarantees by
// static inspection of the shipped static/js/feed.js: that a
// back-forward-cache restore refetches the feed (gap 3, 02-UAT.md Test 3,
// "major"), and — added by this plan's second task — that the "Show
// disputed" filter's state round-trips through the page URL rather than
// resetting on reload (gap 5, 02-UAT.md Test 2, a deliberate 2026-09-17
// scope reversal recorded as a dated amendment on 02-UI-SPEC.md's "Show
// disputed filter (D-10, D-11)" section — see that amendment before
// treating either behaviour as a defect).
package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// TestFeedRefetchesOnBackForwardCacheRestore proves the bfcache-restore
// guard (UAT gap 3): exactly one pageshow listener exists in the file,
// guarded on the event's persisted property before it calls the shared
// store's fetch, and neither unload nor beforeunload appears anywhere —
// either one would make the page permanently ineligible for the
// back-forward cache, which would "fix" the symptom by destroying the very
// restore this listener makes correct.
//
// Honest limit of this test's claim: static inspection proves the listener
// is wired and guarded in the right order. It cannot prove a real Safari
// actually restored the page from its cache — that is the human-check's
// job (02-09-PLAN.md's verification section).
func TestFeedRefetchesOnBackForwardCacheRestore(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/feed.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/feed.js: %v", err)
	}
	text := string(raw)

	if n := strings.Count(text, "'pageshow'"); n != 1 {
		t.Fatalf("expected exactly one 'pageshow' listener, found %d", n)
	}
	if n := strings.Count(text, "'unload'") + strings.Count(text, "'beforeunload'"); n != 0 {
		t.Fatalf("found %d unload/beforeunload listener(s) — either one would make the page "+
			"permanently ineligible for the back-forward cache, destroying the restore this "+
			"listener is making correct", n)
	}

	body, anchorFound, terminatorFound := windowAfter(text, "addEventListener('pageshow'", "});")
	if !anchorFound {
		t.Fatalf("could not locate the pageshow listener registration in feed.js")
	}
	if !terminatorFound {
		t.Fatalf("found the pageshow listener registration but no closing \"});\" that follows it")
	}

	persistedIdx := strings.Index(body, "persisted")
	if persistedIdx == -1 {
		t.Fatalf("pageshow listener body does not reference the event's persisted property")
	}
	fetchIdx := strings.Index(body, "Pinalert.fetchReports()")
	if fetchIdx == -1 {
		t.Fatalf("pageshow listener body does not call Pinalert.fetchReports()")
	}
	if persistedIdx > fetchIdx {
		t.Fatalf("persisted reference (offset %d) must appear before the fetchReports call "+
			"(offset %d) — a fetch not downstream of the guard would fire on every ordinary "+
			"navigation", persistedIdx, fetchIdx)
	}
	if n := strings.Count(body, "Pinalert.fetchReports()"); n != 1 {
		t.Fatalf("expected exactly one Pinalert.fetchReports() call inside the pageshow "+
			"listener, found %d", n)
	}
}

// TestDisputedFilterIsCarriedInThePageURL proves the address half of Show
// Disputed (UAT gap 5, 2026-09-17, in its Phase 3.4 form).
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-18): the tab strip is gone, so the page address no longer carries the
// choice. The old link /?view=disputed (and the older show_disputed=true
// bookmark) is an alias that show-menu.js reads once at boot to open Show
// Disputed, and nothing is written back: feed.js and show-menu.js call neither
// replaceState nor pushState and keep nothing in localStorage. The word the
// alias reads is byte-identical to the word app.js sends to the server, so the
// page and the server still share one vocabulary. The Phase 3.1 and Phase 4
// hands-on tests are walked on this layout after Phase 3.4 is built.
func TestDisputedFilterIsCarriedInThePageURL(t *testing.T) {
	feedRaw, err := fs.ReadFile(StaticFS, "static/js/feed.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/feed.js: %v", err)
	}
	menuRaw, err := fs.ReadFile(StaticFS, "static/js/show-menu.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/show-menu.js: %v", err)
	}
	appRaw, err := fs.ReadFile(StaticFS, "static/js/app.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/app.js: %v", err)
	}
	feedText, menuText, appText := string(feedRaw), string(menuRaw), string(appRaw)

	appParamRE := regexp.MustCompile(`'&([A-Za-z_]+)=' \+ sentView`)
	appMatch := appParamRE.FindStringSubmatch(appText)
	if appMatch == nil {
		t.Fatalf("could not find app.js's fetchReports view parameter literal")
	}
	aliasRE := regexp.MustCompile(`params\.get\('([A-Za-z_]+)'\)`)
	paramNames := map[string]bool{}
	for _, m := range aliasRE.FindAllStringSubmatch(menuText, -1) {
		paramNames[m[1]] = true
	}
	if !paramNames[appMatch[1]] {
		t.Fatalf("show-menu.js does not read the %q parameter app.js sends: the page URL and the "+
			"fetch URL must share one vocabulary (it reads %v)", appMatch[1], paramNames)
	}
	if !strings.Contains(menuText, "word === 'disputed'") {
		t.Fatalf("show-menu.js does not open Show Disputed from view=disputed")
	}
	if !strings.Contains(menuText, "params.get('show_disputed') === 'true'") {
		t.Fatalf("show-menu.js dropped the old show_disputed=true bookmark alias")
	}

	for name, text := range map[string]string{"feed.js": feedText, "show-menu.js": menuText} {
		for _, banned := range []string{"replaceState", "pushState", "localStorage", "sessionStorage"} {
			if strings.Contains(text, banned) {
				t.Fatalf("%s references %s: the Show choice is view state, written nowhere, and must "+
					"not silently persist into a later visit the viewer never asked for", name, banned)
			}
		}
	}
	if n := strings.Count(menuText, "readAlias("); n != 2 {
		t.Fatalf("show-menu.js must define readAlias once and call it once at boot, found %d mentions", n)
	}
	if strings.Contains(feedText, "readAlias(") {
		t.Fatalf("feed.js reads the alias too: it is read in exactly one place")
	}
	if !strings.Contains(feedText, "Pinalert.setShow(Pinalert.state.show);") {
		t.Fatalf("expected feed.js's boot step to hand the store's Show choice back to the store")
	}
}
