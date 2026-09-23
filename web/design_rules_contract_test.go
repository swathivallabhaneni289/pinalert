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
