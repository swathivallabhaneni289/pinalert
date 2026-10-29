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

// TestSharePageNamesItsIcon (WR-T02): the public page names the app icon in
// its head like every other page. With no icon link a browser asks for
// /favicon.ico, a path that is not in the session skip set, and that answer
// would set a cookie the page promises not to set. /icons/app.svg skips it.
func TestSharePageNamesItsIcon(t *testing.T) {
	page := p13ReadTemplate(t, "share_status.html.tmpl")
	head := page
	if i := strings.Index(page, "</head>"); i >= 0 {
		head = page[:i]
	}
	r08tMust(t, "share_status.html.tmpl head", head, `<link rel="icon" href="/icons/app.svg" type="image/svg+xml">`)
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
