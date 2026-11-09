package web

// Quick 261108-fku (owner decision 2026-11-08, testing on the laptop): the
// profile page header is one band across the top from 900px. These tests read
// the shipped template and stylesheet from the embedded trees, never a copy.
// The phone is proved unchanged by the base rule pins below and by the before
// and after screenshots the quick summary records; how the band looks in a
// forced colours theme is pinned here by the declared rules only, because
// WebKit cannot emulate forced colours.

import (
	"regexp"
	"strings"
	"testing"
)

// fkuBase keeps the rules that sit at the top level of the sheet (the phone).
func fkuBase(rules []p16CSSRule) []p16CSSRule {
	var out []p16CSSRule
	for _, r := range rules {
		if r.Media == "" {
			out = append(out, r)
		}
	}
	return out
}

// fkuLaptop keeps the rules inside a min-width: 900px block.
func fkuLaptop(rules []p16CSSRule) []p16CSSRule {
	var out []p16CSSRule
	for _, r := range rules {
		if strings.Contains(r.Media, "min-width: 900px") {
			out = append(out, r)
		}
	}
	return out
}

// fkuForced keeps the rules inside a forced-colors block (review fix IN-05), so
// the three forced colours pins below cannot be met by a rule in the base or the
// laptop block.
func fkuForced(rules []p16CSSRule) []p16CSSRule {
	var out []p16CSSRule
	for _, r := range rules {
		if strings.Contains(r.Media, "forced-colors") {
			out = append(out, r)
		}
	}
	return out
}

// fkuDecl returns the value of prop in the first rule of rules whose head has
// the exact selector part, with every run of white space collapsed to one
// space, and whether it was declared.
func fkuDecl(rules []p16CSSRule, part, prop string) (string, bool) {
	for _, r := range p26Find(rules, "", part) {
		if v, ok := r.Decls[prop]; ok {
			return strings.Join(strings.Fields(v), " "), true
		}
	}
	return "", false
}

func fkuWant(t *testing.T, label string, rules []p16CSSRule, part, prop, want string) {
	t.Helper()
	got, ok := fkuDecl(rules, part, prop)
	if !ok {
		t.Errorf("%s: no rule for %s declares %s", label, part, prop)
		return
	}
	if got != want {
		t.Errorf("%s: %s %s is %q, want %q", label, part, prop, got, want)
	}
}

// fkuIDs reads, in order, the first occurrence of each marker.
func fkuIDs(t *testing.T, label, text string, parts ...string) {
	t.Helper()
	p421InOrder(t, label, text, parts...)
}

func TestPersonProfileBand(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	// Quick 261108-fku (C1, C2): the wrappers sit inside the header and the DOM
	// keeps the phone's reading order. Tab on the laptop runs Followers,
	// Following, the action row, then the rating control.
	fkuIDs(t, "the band order", tmpl,
		`id="person-header"`,
		`id="person-band" class="person-band"`,
		`id="person-disc"`,
		`<h1 id="person-username"`,
		`id="person-name"`,
		`id="person-record" class="person-record"`,
		`id="person-stars"`,
		`id="person-tag"`,
		`id="person-tag-sentence"`,
		`id="person-member-since"`,
		`id="person-home-area"`,
		`id="person-followers-button"`,
		`id="person-following-button"`,
		`id="person-actions"`,
		`id="person-edit"`,
		`id="person-follow"`,
		`id="person-unfollow"`,
		`id="person-bell"`,
		`id="person-more"`,
		`id="person-more-menu"`,
		`id="person-rating"`,
		`id="person-action-error"`,
		`id="person-state-line"`,
		`</header>`,
	)
	for _, id := range []string{"person-band", "person-record"} {
		if n := strings.Count(tmpl, `id="`+id+`"`); n != 1 {
			t.Errorf("person.html.tmpl draws %s %d times, want once", id, n)
		}
	}
	// The state line is outside the band: the band's closing div comes between
	// the action error line and the state line.
	errAt := strings.Index(tmpl, `id="person-action-error"`)
	stateAt := strings.Index(tmpl, `id="person-state-line"`)
	if errAt < 0 || stateAt < errAt || !strings.Contains(tmpl[errAt:stateAt], "</div>") {
		t.Errorf("the state line is not after the band's closing div")
	}

	rules := p26Rules(t, "person.css")
	base := fkuBase(rules)
	laptop := fkuLaptop(rules)
	forced := fkuForced(rules)
	if len(laptop) == 0 {
		t.Fatalf("person.css has no min-width: 900px block")
	}

	// Laptop: one full width column, the band drawn as a grid.
	if v, ok := fkuDecl(laptop, ".person-page", "grid-template-columns"); ok {
		t.Errorf("the laptop .person-page still declares grid-template-columns %q (one full width column, 03.4 R6)", v)
	}
	if v, _ := fkuDecl(laptop, ".person-page", "display"); v == "grid" {
		t.Errorf("the laptop .person-page is still a grid")
	}
	fkuWant(t, "band", laptop, ".person-band", "display", "grid")
	fkuWant(t, "band", laptop, ".person-band", "border", "1px solid var(--color-border)")
	fkuWant(t, "band", laptop, ".person-band", "border-radius", "12px")
	fkuWant(t, "band", laptop, ".person-band", "padding", "var(--space-lg)")
	// Quick 261108-hm7 (owner decision 2026-11-08, all beside your letter; amended
	// 2026-11-08): the disc takes the first column and every other part the second,
	// so the areas are a disc column beside user and actions, name, counts, record.
	// Amended 2026-11-08 (261108-hm7): the fku areas "ident counts", "ident actions"
	// and "record record" are replaced by the rows below.
	fkuWant(t, "band", laptop, ".person-band", "grid-template-columns", "auto auto minmax(0, 1fr)")
	areas, _ := fkuDecl(laptop, ".person-band", "grid-template-areas")
	for _, row := range []string{`"disc user actions"`, `"disc name name"`, `"disc counts counts"`, `"disc record record"`} {
		if !strings.Contains(areas, row) {
			t.Errorf("the band grid areas %q lack the row %s", areas, row)
		}
	}
	for _, old := range []string{`"ident counts"`, `"ident actions"`, `"record record"`} {
		if strings.Contains(areas, old) {
			t.Errorf("the band grid areas %q still hold the 261108-fku row %s", areas, old)
		}
	}
	if _, ok := fkuDecl(laptop, ".person-band", "overflow"); ok {
		t.Errorf("the laptop band declares overflow, which would clip the More menu")
	}
	// The identity wrappers give their children to the grid, so the disc, the h1
	// and the name are grid items.
	fkuWant(t, "ident wrapper", laptop, ".person-band .person-ident", "display", "contents")
	fkuWant(t, "ident text wrapper", laptop, ".person-band .person-ident__text", "display", "contents")
	fkuWant(t, "disc", laptop, ".person-band .person-disc", "grid-area", "disc")
	fkuWant(t, "disc", laptop, ".person-band .person-disc", "align-self", "start")
	fkuWant(t, "username", laptop, ".person-band h1.person-username", "grid-area", "user")
	fkuWant(t, "username", laptop, ".person-band h1.person-username", "align-self", "center")
	fkuWant(t, "name", laptop, ".person-band .person-name", "grid-area", "name")
	// The action row sits right after the username on the same line, left aligned
	// and centred on it. Nothing is pushed to the far right.
	fkuWant(t, "actions", laptop, ".person-band .person-actions", "grid-area", "actions")
	fkuWant(t, "actions", laptop, ".person-band .person-actions", "justify-self", "start")
	fkuWant(t, "actions", laptop, ".person-band .person-actions", "align-self", "center")
	fkuWant(t, "counts", laptop, ".person-band .person-counts", "grid-area", "counts")
	fkuWant(t, "counts", laptop, ".person-band .person-counts", "justify-self", "start")
	fkuWant(t, "counts", laptop, ".person-band .person-counts", "margin-inline-start", "calc((var(--space-12) + 1px) * -1)")
	for _, r := range laptop {
		for prop, v := range r.Decls {
			if prop == "justify-self" && v == "end" || prop == "justify-content" && v == "flex-end" {
				t.Errorf("laptop rule %q pushes to the far right with %s: %s", r.Head, prop, v)
			}
		}
	}
	// D4: the counts read as counts. C5: the border stays so nothing moves.
	fkuWant(t, "counts in the band", laptop, ".person-band .person-count", "border-color", "transparent")
	// The line across the box is gone: the record keeps no border and no padding
	// at the laptop size, and it is the record area of the grid.
	fkuWant(t, "record", laptop, ".person-record", "grid-area", "record")
	for _, prop := range []string{"border-top", "border", "padding", "margin"} {
		if v, ok := fkuDecl(laptop, ".person-record", prop); ok {
			t.Errorf("the laptop .person-record still declares %s: %s (no line across the box)", prop, v)
		}
	}
	// The rating and the error line take the second column after the record.
	fkuWant(t, "rating", laptop, ".person-band .person-rating", "grid-column", "2 / -1")
	fkuWant(t, "tag row", laptop, ".person-tag-group", "flex-direction", "row")
	// D6: the empty and error states sit in a box like the rows box.
	fkuWant(t, "state box", laptop, ".person-reports__state", "border", "1px solid var(--color-border)")
	fkuWant(t, "state box", laptop, ".person-reports__state", "border-radius", "12px")
	// C4: the menu hangs from the right edge of the action row.
	fkuWant(t, "menu", laptop, ".person-band .person-menu:not([hidden])", "right", "0")

	// Phone base: the only base rule that names a wrapper is display contents.
	sawWrapper := false
	for _, r := range base {
		for _, part := range p16HeadParts(r.Head) {
			if part == ".person-band" || part == ".person-record" {
				sawWrapper = true
				if len(r.Decls) != 1 || r.Decls["display"] != "contents" {
					t.Errorf("base rule %q declares %v, want exactly display: contents", r.Head, r.Decls)
				}
			}
		}
	}
	if !sawWrapper {
		t.Errorf("the phone base has no display: contents rule for the band and record wrappers")
	}
	fkuWant(t, "phone page", base, ".person-page", "display", "flex")
	fkuWant(t, "phone page", base, ".person-page", "flex-direction", "column")
	fkuWant(t, "phone page", base, ".person-page", "gap", "var(--space-lg)")
	fkuWant(t, "phone page", base, ".person-page", "padding", "var(--space-md)")
	fkuWant(t, "phone header", base, ".person-header:not([hidden])", "display", "flex")
	fkuWant(t, "phone header", base, ".person-header:not([hidden])", "flex-direction", "column")
	fkuWant(t, "phone header", base, ".person-header:not([hidden])", "gap", "var(--space-md)")
	fkuWant(t, "phone count", base, ".person-count", "border", "1px solid var(--color-border)")
	fkuWant(t, "phone count", base, ".person-count", "border-radius", "8px")
	fkuWant(t, "phone count", base, ".person-count", "background", "var(--color-bg)")
	fkuWant(t, "phone count", base, ".person-count", "min-height", "var(--touch-target-min)")
	fkuWant(t, "phone tag group", base, ".person-tag-group", "flex-direction", "column")
	fkuWant(t, "phone menu", base, ".person-menu:not([hidden])", "left", "0")
	fkuWant(t, "phone state", base, ".person-reports__state", "padding", "var(--space-xl) var(--space-lg) 0")
	for _, r := range base {
		if _, ok := r.Decls["grid-template-areas"]; ok && strings.Contains(r.Head, ".person-band") {
			t.Errorf("a phone base rule declares grid-template-areas for the band")
		}
	}

	// C7: forced colours keeps the edges of the band and the state box.
	if len(forced) == 0 {
		t.Errorf("person.css has no forced-colors block")
	}
	fkuWant(t, "forced band", forced, ".person-band", "border-color", "CanvasText")
	// Amended 2026-11-08 (261108-hm7): the line across the band is gone, so the
	// record has no edge to keep in a forced colours theme.
	if v, ok := fkuDecl(forced, ".person-record", "border-top-color"); ok {
		t.Errorf("the forced colours block still colours a record line: %s", v)
	}
	fkuWant(t, "forced state", forced, ".person-reports__state", "border-color", "CanvasText")

	// Whole file: no CSS order property, no hex colour in the new rules, no new
	// custom property (D7, D8). A property is matched by its exact name, never
	// by substring, because border: contains order.
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`)
	for _, r := range rules {
		if _, ok := r.Decls["order"]; ok {
			t.Errorf("rule %q declares the order property", r.Head)
		}
		named := strings.Contains(r.Head, ".person-band") || strings.Contains(r.Head, ".person-record")
		for prop, v := range r.Decls {
			if named && hex.MatchString(v) {
				t.Errorf("rule %q holds a hex colour in %s", r.Head, prop)
			}
			if named && strings.HasPrefix(prop, "--") {
				t.Errorf("rule %q defines the custom property %s", r.Head, prop)
			}
		}
	}
	// The only custom properties person.css defines are the six chip tints that
	// were there before.
	css := stripCSSComments(p421ReadStatic(t, "css/person.css"))
	for _, m := range regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		if !strings.HasPrefix(m[1], "--p-chip-") {
			t.Errorf("person.css defines the new custom property %s", m[1])
		}
	}
}
