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

// TestVisibilityTagHiddenGuard is this repo's third encounter with the
// [hidden] specificity trap (see TestModalBackdropHiddenGuard and
// TestVoteBlockHiddenGuard): the browser's native [hidden] { display: none }
// rule is user-agent-origin and loses to any author-origin display
// declaration at equal specificity. The chip carries vertical padding, and
// vertical padding on a default-inline element overflows the line box
// instead of expanding it — so the chip must be inline-block, which is
// exactly the author-origin declaration that beats the UA rule.
// Unguarded, an empty bordered chip would render under every single live
// report on both surfaces, with a green build and no other signal.
func TestVisibilityTagHiddenGuard(t *testing.T) {
	const target = ".visibility-tag"
	const guard = ":not([hidden])"
	sawGuarded := false

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

			if !strings.Contains(selectorHead, target) {
				continue
			}
			if !strings.Contains(declBody, "display") {
				continue
			}
			if strings.Contains(selectorHead, guard) {
				if path == "static/css/trust.css" {
					sawGuarded = true
				}
				continue
			}
			t.Errorf(
				"%s: found a rule setting `display` on the %s selector without a %q guard — selector head: %q",
				path, target, guard, strings.TrimSpace(selectorHead),
			)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}

	if !sawGuarded {
		t.Fatalf("expected static/css/trust.css to contain at least one guarded %s%s selector — found none (the rule may have been deleted outright)", target, guard)
	}
}

// TestVisibilityCascadeOverridesAgeRamp proves D-09's mechanism
// structurally: the Provisional and Hidden visibility-state rules win over
// main.css's .sev-*/.age-* ramp purely through trust.css's load position in
// index.html.tmpl, with no specificity trick and no !important
// (02-UI-SPEC.md's cascade contract).
func TestVisibilityCascadeOverridesAgeRamp(t *testing.T) {
	normalize := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}

	mainRaw, err := fs.ReadFile(StaticFS, "static/css/main.css")
	if err != nil {
		t.Fatalf("failed to read embedded static/css/main.css: %v", err)
	}
	mainRules := parseCSSRules(string(mainRaw))

	ageAgingRule, ok := ruleBySelector(mainRules, ".age-aging")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/main.css", ".age-aging")
	}
	wantSeverityCurrent, ok := declsOf(ageAgingRule.declBody)["--severity-current"]
	if !ok {
		t.Fatalf(".age-aging in main.css does not declare --severity-current")
	}

	trustRaw, err := fs.ReadFile(StaticFS, "static/css/trust.css")
	if err != nil {
		t.Fatalf("failed to read embedded static/css/trust.css: %v", err)
	}
	trustText := stripCSSComments(string(trustRaw))
	trustRules := parseCSSRules(trustText)

	provisionalRule, ok := ruleBySelector(trustRules, ".vis-provisional")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/trust.css", ".vis-provisional")
	}
	provisionalDecls := declsOf(provisionalRule.declBody)
	if gotSeverityCurrent, ok := provisionalDecls["--severity-current"]; !ok {
		t.Errorf(".vis-provisional does not declare --severity-current")
	} else if normalize(gotSeverityCurrent) != normalize(wantSeverityCurrent) {
		t.Errorf(".vis-provisional's --severity-current must be string-equal (after whitespace "+
			"normalisation) to .age-aging's own value in main.css — D-09's instruction is to reuse "+
			"the expiry-fade pattern, not to approximate it. Found %q, want %q",
			gotSeverityCurrent, wantSeverityCurrent)
	}
	if _, ok := provisionalDecls["--badge-glyph-fg"]; !ok {
		t.Errorf(".vis-provisional does not declare --badge-glyph-fg")
	}

	hiddenRule, ok := ruleBySelector(trustRules, ".vis-hidden")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/trust.css", ".vis-hidden")
	}
	hiddenDecls := declsOf(hiddenRule.declBody)
	if _, ok := hiddenDecls["--severity-current"]; !ok {
		t.Errorf(".vis-hidden does not declare --severity-current")
	}
	if v := hiddenDecls["--severity-tint"]; v != "transparent" {
		t.Errorf(".vis-hidden's --severity-tint must be exactly \"transparent\" (the row's severity "+
			"background wash removed), found %q", v)
	}
	if _, ok := hiddenDecls["--badge-glyph-fg"]; !ok {
		t.Errorf(".vis-hidden does not declare --badge-glyph-fg")
	}

	var badgeRulesWithBackground []cssRule
	for _, r := range trustRules {
		if strings.Contains(r.selectorHead, "vis-hidden") && strings.Contains(r.declBody, "background") {
			badgeRulesWithBackground = append(badgeRulesWithBackground, r)
		}
	}
	if len(badgeRulesWithBackground) != 1 {
		t.Fatalf("expected exactly one rule in trust.css mentioning the hidden state class and "+
			"declaring background, found %d", len(badgeRulesWithBackground))
	}
	badgeSelector := badgeRulesWithBackground[0].selectorHead
	if !strings.Contains(badgeSelector, ".icon-badge.vis-hidden") {
		t.Errorf("the hidden badge rule's selector head must contain the compound form "+
			"(class on the badge element itself, which covers the map pin) — main.css's own "+
			".icon-badge comment says a descendant form alone covers only the feed row and a "+
			"compound form alone only the map pin. Found selector head: %q", badgeSelector)
	}
	if !strings.Contains(badgeSelector, ".vis-hidden .icon-badge") {
		t.Errorf("the hidden badge rule's selector head must contain the descendant form "+
			"(class on a row ancestor, which covers the feed row) — main.css's own .icon-badge "+
			"comment says a descendant form alone covers only the feed row and a compound form "+
			"alone only the map pin. Found selector head: %q", badgeSelector)
	}

	allowedCustomProps := map[string]bool{
		"--severity-current": true,
		"--severity-tint":    true,
		"--badge-glyph-fg":   true,
	}
	for _, r := range trustRules {
		for name := range declsOf(r.declBody) {
			if strings.HasPrefix(name, "--") && !allowedCustomProps[name] {
				t.Errorf("trust.css declares an unexpected custom property %q — this allowlist "+
					"supersedes 02-05's cruder `grep -cE '^\\s*--[a-z-]+:' web/static/css/trust.css` "+
					"= 0 gate, which could not distinguish re-declaring an existing main.css property "+
					"from inventing a new token; inventing a new token remains forbidden and is what "+
					"this allowlist enforces", name)
			}
		}
	}

	htmlRaw, err := TemplatesFS.ReadFile("templates/index.html.tmpl")
	if err != nil {
		t.Fatalf("failed to read templates/index.html.tmpl from the embedded TemplatesFS: %v", err)
	}
	html := string(htmlRaw)
	mainCSSPos := strings.Index(html, "/static/css/main.css")
	trustCSSPos := strings.Index(html, "/static/css/trust.css")
	if mainCSSPos == -1 || trustCSSPos == -1 {
		t.Fatalf("could not locate both main.css and trust.css stylesheet links in the template")
	}
	if trustCSSPos <= mainCSSPos {
		t.Errorf("trust.css must be linked after main.css in index.html.tmpl, so its .vis-* rules " +
			"win over main.css's .age-* ramp through source order alone")
	}
}

// TestVisibilityTagCopyAndFallback proves T-01-17's validate-before-reflect
// discipline is applied to the two new server-supplied fields this plan
// reads, that the fallback direction never lets an unvalidated value read
// as trusted, and that the chip's Copywriting Contract strings ship
// verbatim.
func TestVisibilityTagCopyAndFallback(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/visibility.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/visibility.js: %v", err)
	}
	text := stripCSSComments(string(raw))

	for _, label := range []string{"Unconfirmed", "Disputed", "Resolved"} {
		if !strings.Contains(text, label) {
			t.Errorf("visibility.js does not contain the Copywriting Contract's %q label verbatim", label)
		}
	}

	stateBody := jsFunctionBody(t, text, "visibilityState(report)")
	if !strings.Contains(stateBody, ".indexOf(") {
		t.Errorf("visibilityState's own body does not call .indexOf( — a server-supplied visibility " +
			"value must be validated against a fixed allowlist before it is reflected into a class " +
			"name (T-01-17), the same discipline app.js's iconClass applies to category")
	}
	if !strings.Contains(stateBody, "'provisional'") {
		t.Errorf("visibilityState's own body does not contain the provisional fallback slug literal")
	}
	if strings.Contains(stateBody, "'live'") {
		t.Errorf("visibilityState's own body contains the live slug literal — the only slug literal " +
			"inside this function must be its fallback. An unrecognised, missing or null visibility " +
			"must render as not-yet-trusted, never as trusted, because the one thing this product " +
			"must never do is overstate corroboration it cannot substantiate")
	}

	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(text, sink) {
			t.Errorf("visibility.js contains %q — every element must be built with "+
				"document.createElement and every string must reach the DOM through Pinalert.setText "+
				"(T-01-03)", sink)
		}
	}
}

// TestVisibilityModuleLoadsBeforeItsConsumers proves visibility.js loads
// after the vendor bridge and before both of its consumers (feed.js,
// map.js) — both resolve window.PinalertVisibility at script-evaluation
// time, and a module hoisted above its provider ships an undefined global
// with no build-time signal.
func TestVisibilityModuleLoadsBeforeItsConsumers(t *testing.T) {
	raw, err := TemplatesFS.ReadFile("templates/index.html.tmpl")
	if err != nil {
		t.Fatalf("failed to read templates/index.html.tmpl from the embedded TemplatesFS: %v", err)
	}
	html := string(raw)

	const visSrc = "/static/js/visibility.js"
	if n := strings.Count(html, visSrc); n != 1 {
		t.Fatalf("expected exactly one occurrence of %q in the template, found %d", visSrc, n)
	}

	window := findTagWindow(t, html, visSrc)
	if !strings.Contains(window, " defer") {
		t.Errorf("visibility.js tag is missing the defer attribute — window: %q", window)
	}
	if !strings.Contains(window, "?v={{.AssetVersion}}") {
		t.Errorf("visibility.js tag is missing the ?v={{.AssetVersion}} cache-busting suffix every "+
			"other app module carries — window: %q", window)
	}

	visPos := strings.Index(html, visSrc)
	bridgeSrc := vendorScriptSources[len(vendorScriptSources)-1]
	bridgePos := strings.Index(html, bridgeSrc)
	if bridgePos == -1 {
		t.Fatalf("could not locate the vendor bridge script %q in the template", bridgeSrc)
	}
	if visPos <= bridgePos {
		t.Fatalf("visibility.js (%d) must load strictly after the vendor bridge script (%d)", visPos, bridgePos)
	}

	for _, consumer := range []string{"/static/js/map.js", "/static/js/feed.js"} {
		consumerPos := strings.Index(html, consumer)
		if consumerPos == -1 {
			t.Fatalf("could not locate consumer module %q in the template", consumer)
		}
		if visPos >= consumerPos {
			t.Fatalf("visibility.js (%d) must load strictly before its consumer %q (%d) — both "+
				"consumers resolve window.PinalertVisibility at evaluation time", visPos, consumer, consumerPos)
		}
	}
}

// TestBothSurfacesMountTheSameVisibilityTag proves D-01's structural
// pattern once more, this time for the display half: both feed.js and
// map.js call the same builder rather than growing their own
// implementation of the visibility tag, following
// TestIconGlyphsAreClassDriven's own shape of iterating a fixed module list
// and asserting a shared property across all of them, and
// TestBothSurfacesMountTheSameVoteBlock's own reasoning for why a class
// literal or a response-field name typed into a consumer is the cheapest
// possible detector for a second implementation.
func TestBothSurfacesMountTheSameVisibilityTag(t *testing.T) {
	modules := []string{"static/js/feed.js", "static/js/map.js"}
	texts := map[string]string{}

	for _, module := range modules {
		raw, err := fs.ReadFile(StaticFS, module)
		if err != nil {
			t.Fatalf("%s: could not read embedded file — %v", module, err)
		}
		text := stripCSSComments(string(raw))
		texts[module] = text

		if !strings.Contains(text, "PinalertVisibility.createVisibilityTag(") {
			t.Errorf("%s: expected a call to PinalertVisibility.createVisibilityTag( — a surface "+
				"calling none of the three PinalertVisibility functions has either dropped the "+
				"treatment entirely, or grown a second implementation, which is exactly what "+
				"02-UI-SPEC.md's \"identical markup/classes so the same script handles both\" exists "+
				"to prevent", module)
		}
		if !strings.Contains(text, "PinalertVisibility.updateVisibilityTag(") {
			t.Errorf("%s: expected a call to PinalertVisibility.updateVisibilityTag( — see the "+
				"message above", module)
		}
		if !strings.Contains(text, "PinalertVisibility.applyVisibilityClass(") {
			t.Errorf("%s: expected a call to PinalertVisibility.applyVisibilityClass( — see the "+
				"message above", module)
		}

		for _, literal := range []string{"visibility-tag", "vis-provisional", "vis-hidden", "visibility_reason"} {
			if strings.Contains(text, literal) {
				t.Errorf("%s: contains the literal %q — this class name or response-field name "+
					"belongs to visibility.js and trust.css only. A class name or a response key typed "+
					"into a consumer is the first step of a second implementation of the visibility "+
					"display, and it would be invisible until someone compared a list against a map "+
					"by hand", module, literal)
			}
		}
		if strings.Contains(text, "report.visibility") {
			t.Errorf("%s: contains %q — a consumer reading the visibility field directly would mean "+
				"a consumer deciding something about trust state itself, rather than rendering the "+
				"resolver's answer through visibility.js", module, "report.visibility")
		}

		// 02-05's own cross-surface prohibitions still hold — a regression
		// here would mean this task disturbed 02-05's vote block.
		for _, literal := range []string{"vote-btn", "vote-controls", "vote-error"} {
			if strings.Contains(text, literal) {
				t.Errorf("%s: contains the literal %q — 02-05's own cross-surface prohibition, still "+
					"in force", module, literal)
			}
		}
		for _, field := range []string{"your_vote", "is_own_report"} {
			if strings.Contains(text, field) {
				t.Errorf("%s: contains the literal %q — 02-05's own cross-surface prohibition, still "+
					"in force", module, field)
			}
		}
	}

	createRowBody := jsFunctionBody(t, texts["static/js/feed.js"], "createRow(id)")
	metaIdx := strings.Index(createRowBody, "body.appendChild(meta)")
	tagIdx := strings.Index(createRowBody, "PinalertVisibility.createVisibilityTag(")
	voteBlockIdx := strings.Index(createRowBody, "PinalertVotes.createVoteBlock(")
	if metaIdx == -1 || tagIdx == -1 || voteBlockIdx == -1 {
		t.Fatalf("createRow's own body is missing one of the three anchors needed to check "+
			"insertion order (meta append=%d, tag builder=%d, vote block builder=%d)",
			metaIdx, tagIdx, voteBlockIdx)
	}
	if !(metaIdx < tagIdx && tagIdx < voteBlockIdx) {
		t.Errorf("createRow's own body must append the visibility tag after the meta paragraph and "+
			"before the vote block is built (found meta=%d, tag=%d, vote block=%d) — this is the "+
			"insertion point 02-05's artifact table reserved", metaIdx, tagIdx, voteBlockIdx)
	}

	popupBody := jsFunctionBody(t, texts["static/js/map.js"], "buildPopupContent(report)")
	descIdx := strings.Index(popupBody, "wrap.appendChild(description)")
	popupTagIdx := strings.Index(popupBody, "PinalertVisibility.createVisibilityTag(")
	popupVoteBlockIdx := strings.Index(popupBody, "PinalertVotes.createVoteBlock(")
	if descIdx == -1 || popupTagIdx == -1 || popupVoteBlockIdx == -1 {
		t.Fatalf("buildPopupContent's own body is missing one of the three anchors needed to check "+
			"insertion order (description append=%d, tag builder=%d, vote block builder=%d)",
			descIdx, popupTagIdx, popupVoteBlockIdx)
	}
	if !(descIdx < popupTagIdx && popupTagIdx < popupVoteBlockIdx) {
		t.Errorf("buildPopupContent's own body must append the visibility tag after the description "+
			"and before the vote block is built (found description=%d, tag=%d, vote block=%d)",
			descIdx, popupTagIdx, popupVoteBlockIdx)
	}

	// Honest limits of this test's claim: static inspection proves the
	// calls are present, in the right functions, in the right order. It
	// cannot prove the resulting node lands in the right place in the
	// rendered DOM, that the cascade resolves as intended, or that the
	// treatment is legible — the end-of-phase human check on Task 3
	// carries those claims; the two are complementary, not redundant.
}

// TestShowDisputedUsesOneSharedQueryParam proves D-10, D-11 and TRUST-02's
// client-side mechanism: the "Show disputed reports" filter contributes one
// boolean to the one report fetch the shared store owns, so the list and
// the map cannot disagree about which reports exist. There is no
// map-specific query, and neither renderer builds one of its own.
func TestShowDisputedUsesOneSharedQueryParam(t *testing.T) {
	appRaw, err := fs.ReadFile(StaticFS, "static/js/app.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/app.js: %v", err)
	}
	appText := stripCSSComments(string(appRaw))

	if n := strings.Count(appText, "show_disputed"); n != 1 {
		t.Fatalf("expected exactly one occurrence of the shared query parameter's name in app.js, "+
			"found %d", n)
	}

	fetchReportsBody := jsFunctionBody(t, appText, "fetchReports()")
	if !strings.Contains(fetchReportsBody, "show_disputed") {
		t.Errorf("fetchReports's own body does not reference the shared query parameter — it must " +
			"sit inside this function's own body so the parameter is conditional on state rather " +
			"than hardcoded")
	}
	if !strings.Contains(fetchReportsBody, "state.showDisputed") {
		t.Errorf("fetchReports's own body does not reference state.showDisputed — the parameter must " +
			"be conditional on the store's own flag")
	}

	if n := strings.Count(appText, "function setShowDisputed("); n != 1 {
		t.Errorf("expected exactly one setShowDisputed( function declaration, found %d", n)
	}
	setterExportRE := regexp.MustCompile(`(?m)^\s*setShowDisputed: setShowDisputed,?$`)
	if !setterExportRE.MatchString(appText) {
		t.Errorf("setShowDisputed must be exported as its own \"name: name\" line in the returned " +
			"object literal")
	}

	// fetch( staying at exactly two (the report fetch, the submit) is the
	// cheapest possible detector for a second report query: a third fetch
	// call would mean the map or the list building its own request, which
	// is exactly the shape that lets one surface see a different set of
	// reports than the other.
	if n := strings.Count(appText, "fetch("); n != 2 {
		t.Errorf("expected exactly two fetch( calls in app.js (the report fetch and the submit "+
			"call), found %d — a third would mean a second report query", n)
	}

	for _, module := range []string{"static/js/feed.js", "static/js/map.js"} {
		raw, err := fs.ReadFile(StaticFS, module)
		if err != nil {
			t.Fatalf("%s: could not read embedded file — %v", module, err)
		}
		text := stripCSSComments(string(raw))
		if strings.Contains(text, "show_disputed") {
			t.Errorf("%s: contains the shared query parameter's name — a surface building its own "+
				"query is exactly the feed/map divergence TRUST-02 forbids, and it would be invisible "+
				"until someone compared a list against a map by hand", module)
		}
	}

	// Honest limits of this test's claim: static inspection proves there is
	// one parameter on one fetch and that neither renderer builds a query.
	// It cannot prove the server honours the parameter (02-04's own
	// TestShowDisputedRevealsHiddenReports covers that against real
	// Postgres) nor that the pins actually appear, which the human check
	// covers.
}

// TestDisputedEmptyStateHiddenGuard proves the OTHER [hidden] guard form is
// used correctly for the disputed empty state: unlike the visibility-tag
// chip (a rule this plan owns), this element's `display: flex` comes from
// main.css's shared .empty-state rule, so a :not([hidden]) guard on a rule
// of this file's own would not be in the cascade path at all. The
// id-and-attribute override — the same form feed.css already uses for its
// four elements — is the one that actually works here.
func TestDisputedEmptyStateHiddenGuard(t *testing.T) {
	trustRaw, err := fs.ReadFile(StaticFS, "static/css/trust.css")
	if err != nil {
		t.Fatalf("failed to read embedded static/css/trust.css: %v", err)
	}
	trustText := stripCSSComments(string(trustRaw))
	trustRules := parseCSSRules(trustText)

	guardRule, ok := ruleBySelector(trustRules, "#disputed-empty[hidden]")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/trust.css", "#disputed-empty[hidden]")
	}
	if v := declsOf(guardRule.declBody)["display"]; v != "none" {
		t.Errorf("#disputed-empty[hidden] must declare display: none, found %q", v)
	}

	// Walk every stylesheet and fail any rule whose selector head mentions
	// the new element WITHOUT the attribute qualifier and whose body sets
	// display — a bare id rule would reintroduce exactly the specificity
	// problem this override exists to fix.
	err = fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".css") {
			return nil
		}
		raw, readErr := fs.ReadFile(StaticFS, path)
		if readErr != nil {
			return readErr
		}
		text := stripCSSComments(string(raw))
		for _, r := range parseCSSRules(text) {
			if !strings.Contains(r.selectorHead, "disputed-empty") {
				continue
			}
			if strings.Contains(r.selectorHead, "[hidden]") {
				continue
			}
			if _, hasDisplay := declsOf(r.declBody)["display"]; hasDisplay {
				t.Errorf("%s: found a rule setting display on a #disputed-empty selector without the "+
					"[hidden] attribute qualifier — selector head: %q", path, r.selectorHead)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}
}

// TestFeedExplainsAnEmptyDisputedResult proves D-10: turning the filter on
// with nothing disputed nearby renders the Copywriting Contract's own copy
// instead of a blank list, and that setEmptyState stays the sole owner of
// both empty-state elements after a fifth element joined render's state
// machine.
func TestFeedExplainsAnEmptyDisputedResult(t *testing.T) {
	htmlRaw, err := TemplatesFS.ReadFile("templates/index.html.tmpl")
	if err != nil {
		t.Fatalf("failed to read templates/index.html.tmpl from the embedded TemplatesFS: %v", err)
	}
	html := string(htmlRaw)

	if n := strings.Count(html, `id="show-disputed-toggle"`); n != 1 {
		t.Fatalf("expected exactly one show-disputed-toggle element, found %d", n)
	}
	toggleWindow := findTagWindow(t, html, `id="show-disputed-toggle"`)
	if !strings.Contains(toggleWindow, `type="checkbox"`) {
		t.Errorf("show-disputed-toggle must be a checkbox input — window: %q", toggleWindow)
	}

	toggleLabelWindow := findTagWindow(t, html, "filter-toggle")
	if !strings.Contains(toggleLabelWindow, "<label") {
		t.Errorf("expected the filter-toggle class on a label element — window: %q", toggleLabelWindow)
	}

	if !strings.Contains(html, "Show disputed reports") {
		t.Errorf("template does not contain the Copywriting Contract's toggle label verbatim")
	}

	togglePos := strings.Index(html, `id="show-disputed-toggle"`)
	listPos := strings.Index(html, `id="report-list"`)
	if togglePos == -1 || listPos == -1 {
		t.Fatalf("could not locate both the toggle and the report list in the template")
	}
	if togglePos >= listPos {
		t.Errorf("the show-disputed-toggle must appear before #report-list in the template, matching "+
			"02-UI-SPEC.md's placement immediately above the list — found toggle at %d, list at %d",
			togglePos, listPos)
	}

	if n := strings.Count(html, `id="disputed-empty"`); n != 1 {
		t.Fatalf("expected exactly one disputed-empty element, found %d", n)
	}
	emptyWindow := findTagWindow(t, html, `id="disputed-empty"`)
	if !strings.Contains(emptyWindow, "empty-state") {
		t.Errorf("disputed-empty must carry the empty-state class — window: %q", emptyWindow)
	}
	if !strings.Contains(emptyWindow, "hidden") {
		t.Errorf("disputed-empty must ship the hidden attribute — window: %q", emptyWindow)
	}

	for _, copy := range []string{
		"No disputed reports nearby",
		"Reports only show up here if enough nearby people have disputed them.",
	} {
		if !strings.Contains(html, copy) {
			t.Errorf("template does not contain the Copywriting Contract's %q verbatim", copy)
		}
	}

	feedRaw, err := fs.ReadFile(StaticFS, "static/js/feed.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/feed.js: %v", err)
	}
	feedText := stripCSSComments(string(feedRaw))

	if n := strings.Count(feedText, "function setEmptyState("); n != 1 {
		t.Fatalf("expected exactly one setEmptyState( function declaration, found %d", n)
	}
	setEmptyStateBody := jsFunctionBody(t, feedText, "setEmptyState(kind)")

	if n := strings.Count(feedText, "setHidden(emptyEl"); n != 1 {
		t.Fatalf("expected exactly one setHidden(emptyEl call in the whole file, found %d — every "+
			"call touching either empty-state element must live inside setEmptyState and nowhere "+
			"else, or render's own \"exactly one place decides which is showing\" comment stops "+
			"being true", n)
	}
	if !strings.Contains(setEmptyStateBody, "setHidden(emptyEl") {
		t.Errorf("the one setHidden(emptyEl call in the file is not inside setEmptyState's own body")
	}

	if n := strings.Count(feedText, "setHidden(disputedEmptyEl"); n != 1 {
		t.Fatalf("expected exactly one setHidden(disputedEmptyEl call in the whole file, found %d",
			n)
	}
	if !strings.Contains(setEmptyStateBody, "setHidden(disputedEmptyEl") {
		t.Errorf("the one setHidden(disputedEmptyEl call in the file is not inside setEmptyState's " +
			"own body")
	}

	if !strings.Contains(feedText, "Pinalert.state.showDisputed") {
		t.Errorf("feed.js does not reference Pinalert.state.showDisputed — the empty-state choice " +
			"must describe the data actually in hand (the store's flag), not an intent that may not " +
			"have been fetched yet (the checkbox)")
	}

	if !strings.Contains(feedText, "addEventListener('change'") {
		t.Errorf("feed.js does not attach a change listener — the toggle must call the store's " +
			"setter and refetch on change")
	}
	if !strings.Contains(feedText, "Pinalert.fetchReports()") {
		t.Errorf("feed.js does not call Pinalert.fetchReports() — checking the toggle must refetch, " +
			"never filter state.reports in place")
	}
}
