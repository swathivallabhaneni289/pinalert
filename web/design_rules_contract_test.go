package web

import (
	"io/fs"
	"strings"
	"testing"
)

// This file gates the two live violations of site-design-rules.md (a
// user-authored memory file, not a repo path — captured 2026-09-21 during
// Phase 2 UAT after the user called the app's buttons, backgrounds and copy
// amateurish) that plan 02-14 (gap closure round 2, plan 4 of 4) fixes:
//
//  1. Long dash characters (em dash and en dash) in any copy a reader can
//     actually see — page titles, body sentences, browser-side error
//     messages, a server-side rate-limit message, and the disclaimer in the
//     verification email sent to real people. TestUserVisibleCopyUsesPlainPunctuation
//     below.
//  2. Pill-shaped (fully rounded) buttons — the list/map view toggle and the
//     toast. TestNoPillShapedControls below.
//
// The rest of site-design-rules.md's gap — a hover/active/transition motion
// system, a radius/shadow/duration token scale, a background/surface-
// elevation treatment, the theme control's icon redesign, the Reopen
// button's wording, and an explicit ruling on the login gate's heading — is
// deliberately NOT fixed here. It is routed to /gsd-ui-phase, per the
// debug session's own recommendation (.planning/debug/ui-visual-polish.md)
// and 02-14-SUMMARY.md's deferral list, because fixing it means designing a
// motion system and a token scale, which is contract-level UI work, not a
// gap-closure code fix.

// templatesWithPlainPunctuationCoverage derives the set of embedded
// templates to scan from TemplatesFS itself, rather than hardcoding a file
// list, so a future template automatically inherits this gate instead of
// silently shipping outside its coverage.
func templatesWithPlainPunctuationCoverage(t *testing.T) map[string]string {
	t.Helper()

	contents := map[string]string{}
	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		raw, err := fs.ReadFile(TemplatesFS, path)
		if err != nil {
			return err
		}
		contents[path] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded templates: %v", err)
	}
	return contents
}

// TestUserVisibleCopyUsesPlainPunctuation is the standing gate for design
// rule 1 above. Whole-file scanning is the correct approach specifically
// for templates: there are no dash-bearing comments in any of them today,
// and template files are almost entirely reader-visible copy, so a false
// positive here would mean this test itself needs updating, not that the
// approach is wrong. The same whole-file approach would be WRONG for the
// JavaScript and Go files this plan also touches (Task 2), which are full
// of dashes inside code comments that are not user-visible and are
// correctly out of scope — those get the narrowly targeted positive
// assertion below instead.
//
// Honest limit of this test's claim: it proves the shipped template bytes
// carry no long-dash characters. It cannot prove the rewritten sentences
// read well — that is a human judgement, recorded in this plan's
// human-check, not something static text scanning can ever verify.
func TestUserVisibleCopyUsesPlainPunctuation(t *testing.T) {
	const emDash = "—" // —
	const enDash = "–" // –

	templates := templatesWithPlainPunctuationCoverage(t)
	if len(templates) < 5 {
		t.Fatalf("expected at least 5 embedded .tmpl files under templates/, found %d — the premise "+
			"this test is built on (a template set derived from TemplatesFS) may have silently "+
			"evaporated", len(templates))
	}

	for path, content := range templates {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(line, emDash) || strings.Contains(line, enDash) {
				t.Errorf("%s:%d: found a long dash character in user-visible copy — line: %q",
					path, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// agreedRateLimitSentence is the wording agreed in this plan's Task 1 and
// reused character for character by Task 2 in three Go files
// (internal/ratelimit/perip.go's tooManyRequestsMessage,
// internal/ratelimit/perip_test.go's expected envelope, and
// internal/api/handlers/auth.go's inline literal). Four files, one
// sentence, one wording — recorded here as the single source the auth.js
// assertion below checks against, so a future edit to any of the four
// copies that drifts from the others fails this build rather than shipping
// silently.
const agreedRateLimitSentence = "Too many requests. Try again in a minute."

// TestUserVisibleCopyUsesPlainPunctuation cannot whole-file scan auth.js —
// SECURITY comments in that file legitimately use dashes (see the file's
// own header comment) — so this is a narrowly targeted POSITIVE assertion
// on the new wording instead of a negative scan for the character. A
// negative scan would fail on the file's own comments, which are correct
// as they are.
func TestUserVisibleCopyUsesPlainPunctuationAuthJS(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/auth.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/auth.js: %v", err)
	}
	content := string(raw)

	count := strings.Count(content, agreedRateLimitSentence)
	if count != 1 {
		t.Errorf("expected the agreed rate-limit sentence %q to appear exactly once in auth.js, found %d",
			agreedRateLimitSentence, count)
	}
}

// TestNoPillShapedControls is the standing gate for design rule 2: no
// pill-shaped (fully rounded) buttons. It reuses this package's own
// parseCSSRules/declsOf/ruleBySelector helpers (web/css_contract_test.go)
// rather than a second parser.
//
// This test cannot be a simple "the fully-rounded value appears zero
// times" count: the severity slider's two track pseudo-elements
// legitimately keep it (a slider track is not a pill-shaped button, and
// site-design-rules.md says so explicitly), so a bare count would fail on
// correct code. Instead it distinguishes WHICH selectors are allowed to be
// fully rounded — the severity slider only — from every other rule, which
// must not be.
//
// Honest limit of this test's claim, in the same spirit as this package's
// other CSS contract tests: it proves the declared radii in the shipped
// stylesheet. It cannot prove the rendered controls look right in a real
// browser — that is the human check's job.
func TestNoPillShapedControls(t *testing.T) {
	const fullyRoundedRadius = "999px"
	const interimRadius = "8px"
	const viewToggleSelector = ".view-toggle"
	const toastSelector = "#toast"
	const severitySliderPrefix = ".severity-slider"

	raw, err := fs.ReadFile(StaticFS, "static/css/main.css")
	if err != nil {
		t.Fatalf("failed to read embedded static/css/main.css: %v", err)
	}
	rules := parseCSSRules(string(raw))

	// The view toggle has two rules sharing the same exact selector head:
	// a base display rule and a responsive rule that alone carries a
	// radius. Collect ALL of them — fatal if fewer than two are found,
	// since a silently vanished premise is worse than no test at all.
	var viewToggleRules []cssRule
	for _, r := range rules {
		if r.selectorHead == viewToggleSelector {
			viewToggleRules = append(viewToggleRules, r)
		}
	}
	if len(viewToggleRules) < 2 {
		t.Fatalf("expected at least 2 rules with exact selector head %q (a base display rule and a "+
			"responsive rounded rule), found %d — the premise this test is built on may have silently "+
			"evaporated", viewToggleSelector, len(viewToggleRules))
	}
	for _, r := range viewToggleRules {
		decls := declsOf(r.declBody)
		radius, ok := decls["border-radius"]
		if !ok {
			continue
		}
		if radius == fullyRoundedRadius {
			t.Errorf("%s declares a fully-rounded border-radius (%s) — site-design-rules.md forbids "+
				"pill-shaped buttons", viewToggleSelector, radius)
		} else if radius != interimRadius {
			t.Errorf("%s declares border-radius %q, want the interim %q", viewToggleSelector, radius, interimRadius)
		}
	}

	toastRule, ok := ruleBySelector(rules, toastSelector)
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/main.css", toastSelector)
	}
	toastDecls := declsOf(toastRule.declBody)
	if radius, ok := toastDecls["border-radius"]; !ok {
		t.Errorf("%s declares no border-radius", toastSelector)
	} else if radius == fullyRoundedRadius {
		t.Errorf("%s declares a fully-rounded border-radius (%s) — site-design-rules.md forbids "+
			"pill-shaped buttons", toastSelector, radius)
	} else if radius != interimRadius {
		t.Errorf("%s declares border-radius %q, want the interim %q", toastSelector, radius, interimRadius)
	}

	// Walk EVERY rule: whichever ones declare the fully-rounded radius
	// must name the severity slider. This is the assertion that
	// distinguishes an allowed selector from a forbidden one, rather than
	// banning the value outright.
	var foundSliderFullyRounded bool
	for _, r := range rules {
		decls := declsOf(r.declBody)
		radius, ok := decls["border-radius"]
		if !ok || radius != fullyRoundedRadius {
			continue
		}
		if !strings.Contains(r.selectorHead, severitySliderPrefix) {
			t.Errorf("rule %q declares a fully-rounded border-radius (%s) but its selector does not "+
				"name the severity slider — site-design-rules.md allows this radius only on the slider "+
				"tracks, every other control must not be a pill", r.selectorHead, radius)
			continue
		}
		foundSliderFullyRounded = true
	}
	if !foundSliderFullyRounded {
		t.Fatalf("expected at least one severity-slider rule to declare the fully-rounded border-radius "+
			"(%s) — if the slider rules were ever deleted, this check would otherwise pass vacuously",
			fullyRoundedRadius)
	}
}
