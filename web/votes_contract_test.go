package web

import (
	"io/fs"
	"strings"
	"testing"
)

// jsFunctionBody locates "function "+name in text and returns the span from
// there to the next occurrence of "\n  function " (module-level two-space
// indentation), or to end of text if it is the last one. This is
// TestIconGlyphsAreClassDriven's own body-bounding idiom (js_contract_test.go)
// lifted into a shared helper, because three tests in this file need it and
// the idiom should drift in one place rather than three. It depends on this
// codebase's module-level two-space indentation, and bounding matters: a
// guard present elsewhere in the file but absent from the function that
// needs it would not actually guard anything.
func jsFunctionBody(t *testing.T, text, name string) string {
	t.Helper()

	anchor := "function " + name
	idx := strings.Index(text, anchor)
	if idx == -1 {
		t.Fatalf("could not locate %q in the text", anchor)
	}

	rest := text[idx:]
	if nextIdx := strings.Index(rest, "\n  function "); nextIdx != -1 {
		return rest[:nextIdx]
	}
	return rest
}

// TestVoteTransportHasNoLocationFallback is the headline gate of plan 02-05
// (D-18, T-02-01): it proves votes.js's location resolver carries no
// fallback-to-a-default-position branch of the kind modal.js's
// initLocation()/showLocationDenied() and map.js's centerOnVisitor()
// correctly ship for report submission and initial map centering. Voting
// has the opposite requirement — a vote cast from a position the voter is
// not standing in silently collapses TRUST-03's distinct-geohash-cell
// independence predicate — so copying that pattern here is this plan's
// single named risk (see .planning/phases/02-trust-mechanic-core-confirm-
// dispute-visibility/02-PATTERNS.md, the "web/static/js/votes.js" section).
//
// Honest limit of this test's claim, in the same spirit as this package's
// other contract tests: static inspection can prove the forbidden shapes
// are absent, that there is exactly one location prompt and one fetch, and
// that the fetch is structurally downstream of the location promise. It
// CANNOT prove a real browser actually refuses a real denied prompt, or
// that the promise chain resolves in the order this file's text suggests.
// That runtime claim belongs to plan 02-05 Task 3's human-check; the two
// are complementary, not redundant.
func TestVoteTransportHasNoLocationFallback(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/votes.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/votes.js: %v", err)
	}
	text := stripCSSComments(string(raw))

	const promptCall = "navigator.geolocation.getCurrentPosition("
	if n := strings.Count(text, promptCall); n != 1 {
		t.Fatalf("expected exactly one %q call, found %d — two call sites would mean two denial "+
			"branches, and only one of them would be under test", promptCall, n)
	}

	// The two fingerprints of the submission path's fall-back-to-a-default-
	// centre branch: modal.js's showLocationDenied() and map.js's
	// centerOnVisitor() both read the shared store's configuration object
	// and both call Leaflet's view-setting method. Either fingerprint
	// appearing here means a vote can be cast from a position the voter is
	// not standing in, which collapses TRUST-03's distinct-cell predicate.
	const configFingerprint = "Pinalert.config"
	const viewSetFingerprint = "setView("
	if strings.Contains(text, configFingerprint) {
		t.Errorf("votes.js references the shared store's configuration object (%q) — this is one "+
			"of the two fingerprints of the submission path's fall-back-to-a-default-centre branch "+
			"(modal.js's showLocationDenied, map.js's centerOnVisitor). See 02-PATTERNS.md's "+
			"\"web/static/js/votes.js\" section", configFingerprint)
	}
	if strings.Contains(text, viewSetFingerprint) {
		t.Errorf("votes.js calls a Leaflet view-setting method (%q) — this is the other fingerprint "+
			"of the submission path's fall-back-to-a-default-centre branch. votes.js has no map and "+
			"needs none; this string appearing here means the denial branch was copy-pasted from "+
			"modal.js or map.js rather than rejecting outright. See 02-PATTERNS.md's "+
			"\"web/static/js/votes.js\" section", viewSetFingerprint)
	}

	if !strings.Contains(text, "reject(") {
		t.Errorf("expected at least one %q call — the denial path must terminate the promise "+
			"instead of resolving with a substitute position", "reject(")
	}

	castVoteBody := jsFunctionBody(t, text, "castVote(reportId, action)")

	locIdx := strings.Index(castVoteBody, "getVoterLocation(")
	fetchIdxInCastVote := strings.Index(castVoteBody, "fetch(")
	if locIdx == -1 {
		t.Fatalf("castVote's own body does not call getVoterLocation( at all")
	}
	if fetchIdxInCastVote == -1 {
		t.Fatalf("castVote's own body does not call fetch( at all")
	}
	if locIdx >= fetchIdxInCastVote {
		t.Errorf("within castVote's own body, getVoterLocation( must appear strictly before "+
			"fetch( (found at %d and %d) — this is the structural form of \"no request is ever "+
			"sent on denial\": the only network call in this file must be downstream of the "+
			"location promise", locIdx, fetchIdxInCastVote)
	}

	if n := strings.Count(text, "fetch("); n != 1 {
		t.Errorf("expected exactly one %q call in the whole module, found %d — a second fetch "+
			"call would mean a network request that is not structurally downstream of the "+
			"location promise", "fetch(", n)
	}

	if !strings.Contains(castVoteBody, "VOTE_ACTIONS.indexOf(") {
		t.Errorf("castVote's own body must reference %q — a caller-supplied action string must be "+
			"validated against a fixed allowlist before it is interpolated into a URL path "+
			"(T-01-17, the same discipline app.js's iconClass applies to category)",
			"VOTE_ACTIONS.indexOf(")
	}
}

// TestVoterLocationIsCachedPerSession proves D-17: the voter's location is
// read from a sessionStorage cache before the native geolocation prompt is
// ever raised, and the cache lives for the browser session rather than
// across visits.
func TestVoterLocationIsCachedPerSession(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/votes.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/votes.js: %v", err)
	}
	text := stripCSSComments(string(raw))

	if strings.Contains(text, "localStorage") {
		t.Errorf("votes.js references localStorage — D-17's cache lives for the browser session " +
			"only, not across visits; a person's coordinates must not outlive the tab")
	}

	getVoterLocationBody := jsFunctionBody(t, text, "getVoterLocation()")

	const getItemCall = "sessionStorage.getItem("
	const setItemCall = "sessionStorage.setItem("
	const promptCall = "navigator.geolocation.getCurrentPosition("

	if n := strings.Count(getVoterLocationBody, getItemCall); n != 1 {
		t.Errorf("expected exactly one %q call within getVoterLocation's own body, found %d",
			getItemCall, n)
	}
	if n := strings.Count(getVoterLocationBody, setItemCall); n != 1 {
		t.Errorf("expected exactly one %q call within getVoterLocation's own body, found %d",
			setItemCall, n)
	}

	getIdx := strings.Index(getVoterLocationBody, getItemCall)
	promptIdx := strings.Index(getVoterLocationBody, promptCall)
	if getIdx == -1 {
		t.Fatalf("getVoterLocation's own body does not call %q at all", getItemCall)
	}
	if promptIdx == -1 {
		t.Fatalf("getVoterLocation's own body does not call %q at all", promptCall)
	}
	if getIdx >= promptIdx {
		t.Errorf("within getVoterLocation's own body, %q must appear strictly before %q (found "+
			"at %d and %d) — the cache must be consulted before the prompt is raised, which is "+
			"the whole of \"asked once per session and cached, not re-prompted on every vote\"",
			getItemCall, promptCall, getIdx, promptIdx)
	}
}

// TestVoteModuleLoadsBeforeItsConsumers proves votes.js loads after the
// vendor bridge and before both of its consumers (map.js, feed.js) — both
// read window.PinalertVotes at script-evaluation time, and a module hoisted
// above its provider ships an undefined global with no build-time signal.
func TestVoteModuleLoadsBeforeItsConsumers(t *testing.T) {
	raw, err := TemplatesFS.ReadFile("templates/index.html.tmpl")
	if err != nil {
		t.Fatalf("failed to read templates/index.html.tmpl from the embedded TemplatesFS: %v", err)
	}
	html := string(raw)

	const votesSrc = "/static/js/votes.js"
	if n := strings.Count(html, votesSrc); n != 1 {
		t.Fatalf("expected exactly one occurrence of %q in the template, found %d", votesSrc, n)
	}

	window := findTagWindow(t, html, votesSrc)
	if !strings.Contains(window, " defer") {
		t.Errorf("votes.js tag is missing the defer attribute — window: %q", window)
	}
	if !strings.Contains(window, "?v={{.AssetVersion}}") {
		t.Errorf("votes.js tag is missing the ?v={{.AssetVersion}} cache-busting suffix every "+
			"other app module carries — window: %q", window)
	}

	votesPos := strings.Index(html, votesSrc)
	bridgeSrc := vendorScriptSources[len(vendorScriptSources)-1]
	bridgePos := strings.Index(html, bridgeSrc)
	if bridgePos == -1 {
		t.Fatalf("could not locate the vendor bridge script %q in the template", bridgeSrc)
	}
	if votesPos <= bridgePos {
		t.Fatalf("votes.js (%d) must load strictly after the vendor bridge script (%d)",
			votesPos, bridgePos)
	}

	for _, consumer := range []string{"/static/js/map.js", "/static/js/feed.js"} {
		consumerPos := strings.Index(html, consumer)
		if consumerPos == -1 {
			t.Fatalf("could not locate consumer module %q in the template", consumer)
		}
		if votesPos >= consumerPos {
			t.Fatalf("votes.js (%d) must load strictly before its consumer %q (%d) — both "+
				"consumers read window.PinalertVotes at evaluation time", votesPos, consumer,
				consumerPos)
		}
	}
}
