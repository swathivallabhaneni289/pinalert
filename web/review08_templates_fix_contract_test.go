package web

import (
	"strings"
	"testing"
)

// Phase 8 code review, templates part (08-REVIEW-templates.md): the pins
// added by the fix round, one test or more per finding.

// TestCommentsHeadingNoRing (WR-T03): after a keyboard Delete, focus moves to
// the Comments heading (tabindex -1, never a Tab stop), and UI-SPEC 7d says it
// draws no ring. Without the rule, :focus-visible draws one.
func TestCommentsHeadingNoRing(t *testing.T) {
	head := ".leaflet-container .report-page .comments .comments__heading:focus"
	r := p16Rule(t, commentRules(t, "static/css/comments.css"), "", head)
	p16MustDecl(t, head, r, map[string]string{"outline": "none"})
}

// r08tMust fails when text lacks any of the needles.
func r08tMust(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s lacks %q", label, n)
		}
	}
}
