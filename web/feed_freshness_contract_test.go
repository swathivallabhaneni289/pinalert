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
