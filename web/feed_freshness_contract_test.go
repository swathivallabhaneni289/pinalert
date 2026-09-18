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

// TestDisputedFilterIsCarriedInThePageURL proves the UAT gap 5 scope
// reversal (dated 2026-09-17, recorded on 02-UI-SPEC.md's "Show disputed
// filter (D-10, D-11)" section): the filter's state round-trips through the
// page URL instead of resetting on reload, using history.replaceState
// (never pushState, so a filter toggle never creates a Back-button step)
// and never localStorage (the filter is view state, not a preference, and
// must not silently persist into a later visit). The URL parameter name is
// read out of BOTH embedded files rather than hardcoded here, so a rename
// on either side can only drift this test, never let the two silently
// diverge into two vocabularies for one idea.
//
// Honest limit of this test's claim: static inspection proves the
// parameter name, the replaceState/pushState/localStorage shape, and that
// the checkbox is assigned from a URL reader are all present and mutually
// consistent. It cannot prove a real browser's address bar actually
// updates, or that a reload actually restores the checked state with no
// visible flash of the unfiltered feed — that is the human-check's job
// (02-09-PLAN.md's verification section).
func TestDisputedFilterIsCarriedInThePageURL(t *testing.T) {
	feedRaw, err := fs.ReadFile(StaticFS, "static/js/feed.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/feed.js: %v", err)
	}
	feedText := string(feedRaw)

	appRaw, err := fs.ReadFile(StaticFS, "static/js/app.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/app.js: %v", err)
	}
	appText := string(appRaw)

	paramConstRE := regexp.MustCompile(`DISPUTED_PARAM\s*=\s*'([^']+)'`)
	paramMatch := paramConstRE.FindStringSubmatch(feedText)
	if paramMatch == nil {
		t.Fatalf("could not find feed.js's DISPUTED_PARAM constant declaration")
	}
	feedParamName := paramMatch[1]

	appParamRE := regexp.MustCompile(`&([A-Za-z_]+)=true`)
	appMatch := appParamRE.FindStringSubmatch(appText)
	if appMatch == nil {
		t.Fatalf("could not find app.js's fetchReports query parameter literal")
	}
	appParamName := appMatch[1]

	if feedParamName != appParamName {
		t.Fatalf("feed.js's URL parameter name %q is not byte-identical to app.js's fetch "+
			"parameter name %q — the page URL and the fetch URL must share one vocabulary",
			feedParamName, appParamName)
	}

	if n := strings.Count(feedText, "replaceState"); n < 1 {
		t.Fatalf("expected at least one replaceState reference in feed.js, found %d", n)
	}
	if n := strings.Count(feedText, "pushState"); n != 0 {
		t.Fatalf("expected zero pushState references in feed.js — a filter toggle must not "+
			"create a Back-button step, found %d", n)
	}
	if strings.Contains(feedText, "localStorage") {
		t.Fatalf("feed.js references localStorage — the disputed filter is view state, not a " +
			"preference, and must not silently persist into a later visit the viewer never " +
			"asked for")
	}
	if !strings.Contains(feedText, "showDisputedToggleEl.checked =") {
		t.Fatalf("expected the checkbox's checked property to be assigned from a URL reader " +
			"somewhere in feed.js's module body")
	}
}
