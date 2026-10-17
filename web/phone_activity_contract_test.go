package web

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

// phoneActivityContainerMedia is the Media the CSS parser of this package gives
// the rules of the large text query in phone.css: the parser keeps only the
// innermost at-rule prelude, so the query nested in the 899.98px block is
// recorded as the container query alone.
const phoneActivityContainerMedia = "@container activity-list (max-width: 17em)"

func phoneAreas(r p16CSSRule) string {
	return strings.Join(strings.Fields(r.Decls["grid-template-areas"]), " ")
}

// TestPhoneActivityCardReflowsAtLargeText pins Phase 3.4 plan 22 (plan 17
// finding 5): on the phone My activity card the last row is "what distance"
// at normal text, and only when the text is enlarged does the distance drop to
// its own line under What happened, so the still current label keeps room for
// its longest word. The query is a container query in em (the harness and the
// type size tokens scale the font size, not the viewport), on the report list,
// inside phone.css's one narrow media block.
func TestPhoneActivityCardReflowsAtLargeText(t *testing.T) {
	raw := p16ReadStatic(t, "static/css/phone.css")
	text := stripCSSComments(raw)
	rules := phoneRules(t)

	// The normal text layout is untouched: the same two columns and the same
	// last row, in the phone block.
	const card = "main.profile-page .profile-report-list > .activity-card"
	const dist = card + " > .activity-card__distance"
	base := wm5CSSRule(t, rules, phoneBlockMedia, card)
	if got := base.Decls["grid-template-columns"]; got != "minmax(0, 1fr) auto" {
		t.Errorf("the phone card columns are %q, want minmax(0, 1fr) auto at normal text", got)
	}
	const normalAreas = `"disc status" "report report" "when when" "what distance"`
	if got := phoneAreas(base); got != normalAreas {
		t.Errorf("the phone card areas are %q, want %q at normal text", got, normalAreas)
	}
	baseDist := wm5CSSRule(t, rules, phoneBlockMedia, dist)
	if baseDist.Decls["padding"] != "10px 0 0 var(--space-md)" || baseDist.Decls["border-top"] != "1px solid var(--color-border)" ||
		baseDist.Decls["text-align"] != "right" {
		t.Errorf("the phone distance cell is %v, want the drawn padding, hairline and right alignment", baseDist.Decls)
	}

	// The list is the size container, named, on the inline axis only.
	list := wm5CSSRule(t, rules, phoneBlockMedia, "main.profile-page > .activity-part .profile-report-list")
	if got := list.Decls["container"]; got != "activity-list / inline-size" {
		t.Errorf("the report list declares container %q, want %q", got, "activity-list / inline-size")
	}

	// The query: its own rules, in the one place, and nothing else in it.
	var inQuery []p16CSSRule
	for _, r := range rules {
		if r.Media == phoneActivityContainerMedia {
			inQuery = append(inQuery, r)
		}
	}
	if len(inQuery) != 2 {
		t.Fatalf("the large text query holds %d rules, want 2 (the card areas and the distance cell)", len(inQuery))
	}
	reflow := wm5CSSRule(t, rules, phoneActivityContainerMedia, card)
	const stackedAreas = `"disc status" "report report" "when when" "what what" "distance distance"`
	if got := phoneAreas(reflow); got != stackedAreas {
		t.Errorf("the large text card areas are %q, want %q", got, stackedAreas)
	}
	if len(reflow.Decls) != 1 {
		t.Errorf("the large text card rule declares %v, want only grid-template-areas", reflow.Decls)
	}
	stackedDist := wm5CSSRule(t, rules, phoneActivityContainerMedia, dist)
	if stackedDist.Decls["padding"] != "0" || stackedDist.Decls["border-top"] != "0" || len(stackedDist.Decls) != 2 {
		t.Errorf("the large text distance cell declares %v, want padding 0 and border-top 0 only", stackedDist.Decls)
	}
	// Same specificity as the rules it overrides, so it must come after them.
	if reflow.Index <= base.Index || stackedDist.Index <= baseDist.Index {
		t.Errorf("the large text rules (%d, %d) must follow the card rules they override (%d, %d)",
			reflow.Index, stackedDist.Index, base.Index, baseDist.Index)
	}

	// The threshold is in em, because a container query reads em from the
	// container's own font size and the type size tokens scale that. It must
	// stay below the narrowest list (320px screen, 16px page padding each side,
	// so 288px) at normal text size, which is 18em at 16px, or the 100 percent
	// look would move, and it must leave room for text to grow before it acts.
	const prefix = "@container activity-list (max-width: "
	if !strings.HasPrefix(phoneActivityContainerMedia, prefix) || !strings.HasSuffix(phoneActivityContainerMedia, "em)") {
		t.Fatalf("the query %q is not a max-width threshold in em", phoneActivityContainerMedia)
	}
	em, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(phoneActivityContainerMedia, prefix), "em)"), 64)
	if err != nil {
		t.Fatalf("the threshold of %q is not a number of em: %v", phoneActivityContainerMedia, err)
	}
	const narrowestList, normalFontPx = 288.0, 16.0
	if em*normalFontPx >= narrowestList {
		t.Errorf("the threshold %vem is %vpx at normal text, not under the %vpx list of a 320px screen: the drawn look would move",
			em, em*normalFontPx, narrowestList)
	}
	if em*normalFontPx < narrowestList*0.9 {
		t.Errorf("the threshold %vem is %vpx at normal text, far under the %vpx list of a 320px screen: the label would break first",
			em, em*normalFontPx, narrowestList)
	}

	// The query sits inside the 899.98px block: between its opening brace and
	// its closing brace in the raw text, exactly once in the file.
	if n := strings.Count(text, phoneActivityContainerMedia); n != 1 {
		t.Fatalf("phone.css holds the large text query %d times, want 1", n)
	}
	open := strings.Index(text, phoneBlockMedia+" {")
	if open == -1 {
		t.Fatalf("phone.css has no %s block", phoneBlockMedia)
	}
	depth, end := 0, -1
	for i := open + len(phoneBlockMedia); i < len(text) && end == -1; i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
	}
	at := strings.Index(text, phoneActivityContainerMedia)
	if end == -1 || at < open || at > end {
		t.Errorf("the large text query (at %d) is not inside the first %s block (%d to %d)", at, phoneBlockMedia, open, end)
	}

	// Nothing else in the app uses a container: the laptop stack, the map
	// popup and the feed rows are as they were.
	err = fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, werr error) error {
		if werr != nil || d.IsDir() || !strings.HasSuffix(path, ".css") || path == "static/css/phone.css" {
			return werr
		}
		css := stripCSSComments(p16ReadStatic(t, path))
		for _, bad := range []string{"@container", "container-type", "container-name", "container:"} {
			if strings.Contains(css, bad) {
				t.Errorf("%s uses %q, the only container is the My activity list in phone.css", path, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk static/css: %v", err)
	}
}
