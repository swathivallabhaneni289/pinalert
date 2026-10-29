package web

import (
	"os"
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

// TestTrustModelSharingRemovedReport (WR-T06): the public page answers a
// removed report with "This report was removed" (D-06), so TRUST-MODEL's
// Sharing section must not say a removed report answers like an unknown one.
func TestTrustModelSharingRemovedReport(t *testing.T) {
	doc := r08tReadRepo(t, "../docs/TRUST-MODEL.md")
	if strings.Contains(doc, "held, removed or too old report answers like an unknown one") {
		t.Errorf("TRUST-MODEL.md says a removed report answers like an unknown one; it says it was removed")
	}
	r08tMust(t, "TRUST-MODEL.md", doc,
		"token for a held or too old report answers like an unknown one",
		"a removed one says it was removed (D-06)")
}

// TestPolicyCommentCheckKeptForEvery (WR-T04): every checked comment keeps
// the check's verdict, its scores and the model with it, clear or borderline,
// so the Policy must not say only a borderline one does.
func TestPolicyCommentCheckKeptForEvery(t *testing.T) {
	keeps := r71tPolicySection(t, "What Pinalert keeps")
	if strings.Contains(keeps, "for a borderline one the check") {
		t.Errorf("the Policy says only a borderline comment keeps its check result")
	}
	r08tMust(t, "the Policy's keeps list", keeps,
		"A comment is checked by the same service as a report. For every comment the check's result, its scores and which model gave them are kept with the comment.")
}

// r08tReadRepo reads a file of the repository, relative to web/.
func r08tReadRepo(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
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
