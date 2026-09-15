package web

import (
	"io/fs"
	"regexp"
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

// TestVoteBlockHiddenGuard is this plan's highest-value automated gate. The
// browser's native [hidden] { display: none } rule is user-agent-origin and
// loses to any author-origin display declaration at equal specificity —
// exactly the specificity trap that shipped a permanently-open modal
// backdrop in Phase 1 (a live UAT blocker, see TestModalBackdropHiddenGuard)
// and threatened the same for the account menu in Phase 1.1 (see
// TestAccountMenuHiddenGuard). Here the consequence would be an empty error
// paragraph rendered under every single feed row and every popup, with a
// green build and no other signal — and, for .vote-controls, an empty
// bordered strip under every report the viewer submitted themselves.
func TestVoteBlockHiddenGuard(t *testing.T) {
	const guard = ":not([hidden])"
	targets := []string{".vote-error", ".vote-controls"}
	sawGuarded := map[string]bool{}

	err := fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".css") {
			return nil
		}

		raw, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}
		text := stripCSSComments(string(raw))

		for chunk := range strings.SplitSeq(text, "}") {
			lastOpen := strings.LastIndex(chunk, "{")
			if lastOpen == -1 {
				continue
			}
			selectorHead := chunk[:lastOpen]
			declBody := chunk[lastOpen+1:]

			for _, target := range targets {
				if !strings.Contains(selectorHead, target) {
					continue
				}
				if !strings.Contains(declBody, "display") {
					continue
				}
				if strings.Contains(selectorHead, guard) {
					if path == "static/css/trust.css" {
						sawGuarded[target] = true
					}
					continue
				}
				t.Errorf(
					"%s: found a rule setting `display` on the %s selector without a %q guard — selector head: %q",
					path, target, guard, strings.TrimSpace(selectorHead),
				)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}

	for _, target := range targets {
		if !sawGuarded[target] {
			t.Fatalf("expected static/css/trust.css to contain at least one guarded %s%s selector — found none (the rule may have been deleted outright)", target, guard)
		}
	}
}

// anyDeclReferences reports whether any declaration value in decls contains
// token as a substring — used below to check that a rule references a
// given custom property without pinning to one specific declaration name (a
// color token might land on border-color or color depending on the rule).
func anyDeclReferences(decls map[string]string, token string) bool {
	for _, v := range decls {
		if strings.Contains(v, token) {
			return true
		}
	}
	return false
}

// TestVoteButtonsMeetTouchTargetAndUseTokensOnly proves trust.css introduces
// no new color and resolves every value through main.css's existing tokens,
// and that the vote buttons and their active/disabled states are declared
// exactly as 02-UI-SPEC.md's Color and Spacing sections specify.
//
// Honest limit of this test's claim: static inspection proves the
// declarations exist and are token-derived; it cannot prove the rendered
// result is legible or that the touch target is actually 44px after the
// cascade. The end-of-phase human check covers that.
func TestVoteButtonsMeetTouchTargetAndUseTokensOnly(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/css/trust.css")
	if err != nil {
		t.Fatalf("failed to read embedded static/css/trust.css: %v", err)
	}
	text := stripCSSComments(string(raw))

	hexColorRE := regexp.MustCompile(`#([0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})\b`)
	if m := hexColorRE.FindString(text); m != "" {
		t.Errorf("trust.css contains a raw hex colour literal (%q) — every colour must resolve "+
			"through a var(--...) token; this phase introduces no new hex value", m)
	}
	for _, fn := range []string{"rgb(", "rgba(", "hsl("} {
		if strings.Contains(text, fn) {
			t.Errorf("trust.css contains a %q functional colour notation — every colour must "+
				"resolve through a var(--...) token", fn)
		}
	}

	rules := parseCSSRules(text)

	btnRule, ok := ruleBySelector(rules, ".vote-btn")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/trust.css", ".vote-btn")
	}
	btnDecls := declsOf(btnRule.declBody)
	if v := btnDecls["min-height"]; v != "var(--touch-target-min)" {
		t.Errorf(".vote-btn min-height must be exactly var(--touch-target-min), found %q", v)
	}
	padding := btnDecls["padding"]
	if !strings.Contains(padding, "var(--space-sm)") {
		t.Errorf(".vote-btn padding must reference var(--space-sm), found %q", padding)
	}
	if strings.Contains(padding, "var(--space-lg)") {
		t.Errorf(".vote-btn padding must NOT reference var(--space-lg) — 02-UI-SPEC.md's explicit "+
			"deviation from .btn: three buttons at the default horizontal padding overflow a "+
			"narrow feed row, found %q", padding)
	}

	confirmRule, ok := ruleBySelector(rules, `.vote-btn--confirm[aria-pressed="true"]`)
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/trust.css", `.vote-btn--confirm[aria-pressed="true"]`)
	}
	if !anyDeclReferences(declsOf(confirmRule.declBody), "--color-severity-low") {
		t.Errorf(".vote-btn--confirm[aria-pressed=\"true\"] must reference --color-severity-low — "+
			"reusing the existing green hue's established low-severity valence rather than "+
			"inventing a fifth colour, declarations: %v", declsOf(confirmRule.declBody))
	}

	disputeRule, ok := ruleBySelector(rules, `.vote-btn--dispute[aria-pressed="true"]`)
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/trust.css", `.vote-btn--dispute[aria-pressed="true"]`)
	}
	if !anyDeclReferences(declsOf(disputeRule.declBody), "--color-severity-critical") {
		t.Errorf(".vote-btn--dispute[aria-pressed=\"true\"] must reference --color-severity-critical "+
			"— reusing the existing red hue's established critical-severity valence, declarations: %v",
			declsOf(disputeRule.declBody))
	}

	disabledRule, ok := ruleBySelector(rules, ".vote-btn:disabled")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/trust.css", ".vote-btn:disabled")
	}
	if !anyDeclReferences(declsOf(disabledRule.declBody), "--color-text-muted") {
		t.Errorf(".vote-btn:disabled must reference --color-text-muted, declarations: %v",
			declsOf(disabledRule.declBody))
	}
}

// TestOwnReportRuleRemovesControlsRatherThanDisablingThem proves D-03's UI
// courtesy is implemented exactly as 02-UI-SPEC.md specifies (removal, not
// disabling) and proves the disabled attribute has exactly one owner
// (setBlockBusy), so a background poll landing mid-vote can never re-enable
// an in-flight button. It also proves votes.js contains no markup-parsing
// sink anywhere (T-01-03): every element this module builds goes through
// document.createElement, and the server's own error messages — though
// server-authored — still reach the DOM only through Pinalert.setText,
// because the sink discipline is about the sink, not about who wrote the
// string.
func TestOwnReportRuleRemovesControlsRatherThanDisablingThem(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/votes.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/votes.js: %v", err)
	}
	text := stripCSSComments(string(raw))

	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(text, sink) {
			t.Errorf("votes.js contains %q — every element must be built with "+
				"document.createElement and every string must reach the DOM through "+
				"Pinalert.setText (T-01-03)", sink)
		}
	}

	applyOwnReportBody := jsFunctionBody(t, text, "applyOwnReportRule(block, report)")
	if !strings.Contains(applyOwnReportBody, "is_own_report") {
		t.Errorf("applyOwnReportRule's own body does not reference is_own_report")
	}
	if !strings.Contains(applyOwnReportBody, ".remove()") {
		t.Errorf("applyOwnReportRule's own body does not call .remove() — 02-UI-SPEC.md is " +
			"explicit that the reporter's buttons are removed from the DOM, not disabled")
	}
	if strings.Contains(applyOwnReportBody, "disabled") {
		t.Errorf("applyOwnReportRule's own body references \"disabled\" — the reporter's buttons " +
			"must be removed, never disabled; a disabled button reads as a bug on a row already " +
			"tight for space")
	}

	updateVoteBlockBody := jsFunctionBody(t, text, "updateVoteBlock(block, report)")
	if strings.Contains(updateVoteBlockBody, "disabled") {
		t.Errorf("updateVoteBlock's own body references \"disabled\" — setBlockBusy is the sole " +
			"owner of that attribute; a background poll landing mid-vote must not be able to " +
			"re-enable an in-flight button")
	}
	if n := strings.Count(updateVoteBlockBody, "aria-pressed"); n < 2 {
		t.Errorf("updateVoteBlock's own body must reference aria-pressed at least twice (both "+
			"buttons written on every call), found %d — writing only the newly active button "+
			"leaves a stale aria-pressed=\"true\" on the other after a vote change", n)
	}

	setBlockBusyBody := jsFunctionBody(t, text, "setBlockBusy(block, busy)")
	if !strings.Contains(setBlockBusyBody, "disabled") {
		t.Errorf("setBlockBusy's own body does not reference \"disabled\" — it must be the sole " +
			"owner of that attribute")
	}
	if !strings.Contains(setBlockBusyBody, "aria-busy") {
		t.Errorf("setBlockBusy's own body does not reference \"aria-busy\"")
	}
}

// TestBothSurfacesMountTheSameVoteBlock proves D-01 structurally: both
// feed.js and map.js call the same builder rather than growing their own
// implementation of the vote controls, following TestIconGlyphsAreClassDriven's
// own shape of iterating a fixed module list and asserting a shared property
// across all of them.
func TestBothSurfacesMountTheSameVoteBlock(t *testing.T) {
	modules := []string{"static/js/feed.js", "static/js/map.js"}

	for _, module := range modules {
		raw, err := fs.ReadFile(StaticFS, module)
		if err != nil {
			t.Fatalf("%s: could not read embedded file — %v", module, err)
		}
		text := stripCSSComments(string(raw))

		if !strings.Contains(text, "PinalertVotes.createVoteBlock(") {
			t.Errorf("%s: expected a call to PinalertVotes.createVoteBlock( — a surface calling "+
				"neither this nor updateVoteBlock has either dropped its controls or grown a "+
				"second implementation, which is exactly what D-01 exists to prevent", module)
		}
		if !strings.Contains(text, "PinalertVotes.updateVoteBlock(") {
			t.Errorf("%s: expected a call to PinalertVotes.updateVoteBlock( — see the message "+
				"above", module)
		}

		for _, literal := range []string{"vote-btn", "vote-controls", "vote-error"} {
			if strings.Contains(text, literal) {
				t.Errorf("%s: contains the literal %q — this class name belongs to votes.js and "+
					"trust.css only. A class name typed into a consumer is the first step of a "+
					"second implementation of the vote controls", module, literal)
			}
		}
		for _, field := range []string{"your_vote", "is_own_report"} {
			if strings.Contains(text, field) {
				t.Errorf("%s: contains the literal %q — both fields are read inside "+
					"updateVoteBlock; a consumer reading them would mean a consumer deciding "+
					"something about vote state itself", module, field)
			}
		}
	}
}

// TestVoteClickDoesNotActivateItsRow proves a tap on a vote button never
// reaches the feed row's own activation handlers (no map fly-to, no view
// switch, no selection change).
//
// Honest limit of this test's claim: static inspection proves the calls
// are present in the right function; it cannot prove the event actually
// stops, that the listener is attached to the right element, or that
// Leaflet's popup does not intercept first. The end-of-phase human check
// covers the runtime claim; the two are complementary, not redundant.
func TestVoteClickDoesNotActivateItsRow(t *testing.T) {
	votesRaw, err := fs.ReadFile(StaticFS, "static/js/votes.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/votes.js: %v", err)
	}
	votesText := stripCSSComments(string(votesRaw))

	createVoteBlockBody := jsFunctionBody(t, votesText, "createVoteBlock(reportId)")
	if n := strings.Count(createVoteBlockBody, "stopPropagation"); n < 2 {
		t.Errorf("createVoteBlock's own body must call stopPropagation at least twice (once in "+
			"the click listener, once in the keydown listener), found %d", n)
	}

	feedRaw, err := fs.ReadFile(StaticFS, "static/js/feed.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/feed.js: %v", err)
	}
	feedText := stripCSSComments(string(feedRaw))

	if n := strings.Count(feedText, "activateRow(id)"); n < 3 {
		t.Errorf("expected feed.js to still call activateRow(id) at least three times (the "+
			"definition plus both the row's own click and keydown listeners), found %d — this "+
			"guard only matters while the row's own activation handlers still exist; a future "+
			"edit that removed them would leave this test passing vacuously", n)
	}
}
