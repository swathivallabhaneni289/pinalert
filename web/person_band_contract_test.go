package web

// Quick 261108-fku (owner decision 2026-11-08, testing on the laptop) drew the
// profile page header as one band across the top from 900px. Amended
// 2026-11-13 by quick 261113-k9t (owner decision 2026-11-13, the centred
// profile): the band is replaced on both sizes by the centred header, and the
// pins below describe that. These tests read the shipped template and
// stylesheet from the embedded trees, never a copy. How the header looks in a
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

// fkuPhoneOnly keeps the rules inside the phone only block (below 900px).
func fkuPhoneOnly(rules []p16CSSRule) []p16CSSRule {
	var out []p16CSSRule
	for _, r := range rules {
		if strings.Contains(r.Media, "not all and (min-width: 900px)") {
			out = append(out, r)
		}
	}
	return out
}

// TestPersonProfileCentred pins quick 261113-k9t (owner decision 2026-11-13,
// the centred profile, option D of the mockup, "lets do the centered"). It
// replaces the TestPersonProfileBand pins of quick 261108-fku, 261108-hm7 and
// 261109-n25: the band, its record wrapper, the grid areas, the display
// contents wrappers, the band placement rules and the hm7 Tab order note are
// gone, and the header is one centred column on the laptop and on the phone.
// Amended 2026-11-13 by that decision.
func TestPersonProfileCentred(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	// The DOM order is the reading order (this ends the hm7 conflict between the
	// Tab order and the drawing): the letter, the username, the name, the action
	// row, the counts row, the details line, then the state line or Your rating,
	// then the action error line.
	fkuIDs(t, "the header order", tmpl,
		`id="person-header"`,
		`id="person-disc"`,
		`<h1 id="person-username"`,
		`class="person-username person-username--phone"`,
		`id="person-name"`,
		`id="person-actions"`,
		`id="person-edit"`,
		`id="person-follow"`,
		`id="person-unfollow"`,
		`id="person-bell"`,
		`id="person-more"`,
		`id="person-more-menu"`,
		`id="person-stats" class="person-stats"`,
		`id="person-stars"`,
		`id="person-followers-button"`,
		`id="person-following-button"`,
		`class="person-tag-group"`,
		`id="person-tag"`,
		`id="person-tag-sentence"`,
		`class="person-facts"`,
		`id="person-member-since"`,
		`id="person-home-area"`,
		`id="person-state-line"`,
		`id="person-rating"`,
		`id="person-action-error"`,
		`</header>`,
	)
	// The counts row holds the stars group (when present), the two count buttons
	// and the reporter cell, in that order, and nothing else.
	statsAt := strings.Index(tmpl, `id="person-stats"`)
	factsAt := strings.Index(tmpl, `class="person-facts"`)
	if statsAt < 0 || factsAt < statsAt {
		t.Fatalf("the counts row or the details line was not found")
	}
	stats := tmpl[statsAt:factsAt]
	for _, id := range []string{"person-stars", "person-followers-button", "person-following-button", "person-tag"} {
		if strings.Count(stats, `id="`+id+`"`) != 1 {
			t.Errorf("the counts row draws %s %d times, want once", id, strings.Count(stats, `id="`+id+`"`))
		}
	}
	// The details line has the clock and the map pin, both hidden from a screen reader.
	facts := tmpl[factsAt:strings.Index(tmpl, `id="person-state-line"`)]
	for _, want := range []string{`app-icon--clock`, `app-icon--map-pin`} {
		if !strings.Contains(facts, want) {
			t.Errorf("the details line lacks %s", want)
		}
	}
	if strings.Count(facts, `aria-hidden="true"`) < 2 {
		t.Errorf("the details line icons are not aria-hidden")
	}
	// No wrapper of the earlier layouts is left in the markup or the stylesheet.
	css := stripCSSComments(p421ReadStatic(t, "css/person.css"))
	for _, gone := range []string{"person-band", "person-record", "person-ident", "person-counts"} {
		if strings.Contains(tmpl, gone) {
			t.Errorf("person.html.tmpl still names %s", gone)
		}
		if strings.Contains(css, gone) {
			t.Errorf("person.css still names %s", gone)
		}
	}

	rules := p26Rules(t, "person.css")
	base := fkuBase(rules)
	laptop := fkuLaptop(rules)
	phoneOnly := fkuPhoneOnly(rules)
	forced := fkuForced(rules)
	if len(laptop) == 0 {
		t.Fatalf("person.css has no min-width: 900px block")
	}
	if len(phoneOnly) == 0 {
		t.Fatalf("person.css has no phone only block")
	}

	// The page: still a full width column (03.4 R6), never a grid.
	if v, ok := fkuDecl(laptop, ".person-page", "grid-template-columns"); ok {
		t.Errorf("the laptop .person-page declares grid-template-columns %q (one full width column, 03.4 R6)", v)
	}
	if v, _ := fkuDecl(laptop, ".person-page", "display"); v == "grid" {
		t.Errorf("the laptop .person-page is a grid")
	}
	fkuWant(t, "phone page", base, ".person-page", "display", "flex")
	fkuWant(t, "phone page", base, ".person-page", "flex-direction", "column")
	fkuWant(t, "phone page gap", base, ".person-page", "gap", "var(--space-xl)")
	fkuWant(t, "phone page", base, ".person-page", "padding", "var(--space-lg) var(--space-md) var(--space-md)")
	fkuWant(t, "laptop page gap", laptop, ".person-page", "gap", "var(--space-2xl)")
	fkuWant(t, "laptop page", laptop, ".person-page", "padding", "var(--space-2xl) var(--space-lg) var(--space-lg)")

	// The header is a centred column on both sizes.
	fkuWant(t, "header", base, ".person-header:not([hidden])", "display", "flex")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "flex-direction", "column")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "align-items", "center")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "text-align", "center")
	// The letter is 88px on the phone and 104px on the laptop, from tokens.
	fkuWant(t, "phone letter", base, ".person-disc", "width", "calc(var(--space-3xl) + var(--space-lg))")
	fkuWant(t, "phone letter", base, ".person-disc", "height", "calc(var(--space-3xl) + var(--space-lg))")
	fkuWant(t, "laptop letter", laptop, ".person-disc", "width", "calc(var(--space-3xl) + var(--space-xl) + var(--space-sm))")
	fkuWant(t, "laptop letter", laptop, ".person-disc", "height", "calc(var(--space-3xl) + var(--space-xl) + var(--space-sm))")
	// The username: heading size on the phone, display size on the laptop h1.
	fkuWant(t, "phone username", base, ".person-username", "font-size", "var(--font-size-heading)")
	fkuWant(t, "phone username", base, ".person-username", "font-weight", "var(--font-weight-semibold)")
	fkuWant(t, "laptop username", laptop, "h1.person-username", "font-size", "var(--font-size-display)")
	fkuWant(t, "laptop username", laptop, "h1.person-username", "display", "block")
	fkuWant(t, "phone h1", base, "h1.person-username", "display", "none")
	fkuWant(t, "laptop phone line", laptop, ".person-username--phone", "display", "none")
	fkuWant(t, "name", base, ".person-name", "color", "var(--color-text-muted)")

	// The action row is centred; the laptop buttons are small (quick 261109-n25
	// stays: 32px, the label font size, the radius of .btn left alone) and the
	// phone buttons keep the 44px of .btn and the bell.
	fkuWant(t, "action row", base, ".person-actions", "justify-content", "center")
	fkuWant(t, "small buttons", laptop, ".person-header .person-actions .btn", "min-height", "32px")
	fkuWant(t, "small buttons", laptop, ".person-header .person-actions .btn", "padding", "0 var(--space-12)")
	fkuWant(t, "small buttons", laptop, ".person-header .person-actions .btn", "font-size", "var(--font-size-label)")
	if v, ok := fkuDecl(laptop, ".person-header .person-actions .btn", "border-radius"); ok {
		t.Errorf("the small buttons change the radius: %s", v)
	}
	for _, prop := range []string{"width", "height", "min-width", "min-height"} {
		fkuWant(t, "small bell", laptop, ".person-header .person-bell:not([hidden])", prop, "32px")
		fkuWant(t, "phone bell", base, ".person-bell:not([hidden])", prop, "var(--touch-target-min)")
	}
	// The More menu hangs from the right edge of the row, under More, on both
	// sizes, so it stays inside the viewport at 320px.
	fkuWant(t, "menu", base, ".person-menu:not([hidden])", "right", "0")
	fkuWant(t, "menu", base, ".person-menu:not([hidden])", "left", "auto")
	fkuWant(t, "menu", base, ".person-menu:not([hidden])", "min-width", "200px")

	// The counts row: a flex row with no outer box, a 1px line between the cells,
	// the count buttons 44px or more tall with no box of their own.
	fkuWant(t, "counts row", base, ".person-stats", "display", "flex")
	fkuWant(t, "counts row", base, ".person-stats", "flex-wrap", "wrap")
	fkuWant(t, "counts row", base, ".person-stats", "justify-content", "center")
	for _, prop := range []string{"border", "border-width", "border-style", "border-color", "border-radius", "padding", "background", "background-color"} {
		for label, set := range map[string][]p16CSSRule{"base": base, "laptop": laptop} {
			if v, ok := fkuDecl(set, ".person-stats", prop); ok {
				t.Errorf("the %s .person-stats declares %s: %s (no outer box on the counts row)", label, prop, v)
			}
		}
	}
	fkuWant(t, "cell line", base, ".person-stats > * + *", "border-left", "1px solid var(--color-border)")
	fkuWant(t, "cell", base, ".person-stats > *", "flex-direction", "column")
	fkuWant(t, "cell", base, ".person-stats > *", "align-items", "center")
	fkuWant(t, "count button", base, ".person-count", "min-height", "var(--touch-target-min)")
	fkuWant(t, "count button", base, ".person-count", "border", "0")
	fkuWant(t, "count button", base, ".person-count", "background", "transparent")
	fkuWant(t, "count number", base, ".person-count__number", "font-weight", "var(--font-weight-semibold)")
	fkuWant(t, "count word", base, ".person-count__word", "color", "var(--color-text-muted)")
	if v, ok := fkuDecl(laptop, ".person-count", "min-height"); ok {
		t.Errorf("the laptop shrinks the counts' hit area: %s", v)
	}
	// The stars cell draws the star and the number with Stars under them.
	fkuWant(t, "stars cell", base, ".person-stars:not([hidden])", "display", "grid")
	fkuWant(t, "stars word", base, ".person-stars__word", "grid-column", "1 / -1")
	// The reporter cell: the chip with its sentence under it, the chip allowed to
	// wrap in a narrow phone cell and kept on one line on the laptop.
	fkuWant(t, "tag sentence", base, ".person-tag-sentence", "color", "var(--color-text-muted)")
	fkuWant(t, "tag chip", base, ".person-tag", "overflow-wrap", "anywhere")
	fkuWant(t, "laptop tag chip", laptop, ".person-tag", "white-space", "nowrap")
	// Laptop cells: at least 140px wide, the reporter cell at most 260px, one row.
	fkuWant(t, "laptop cells", laptop, ".person-stats > *", "min-width", "140px")
	fkuWant(t, "laptop cells", laptop, ".person-stats > *", "padding", "var(--space-sm) var(--space-lg)")
	fkuWant(t, "laptop reporter cell", laptop, ".person-stats > .person-tag-group", "max-width", "260px")
	fkuWant(t, "laptop counts row", laptop, ".person-stats", "flex-wrap", "nowrap")
	// Phone: the cells share the width; with the stars cell there are four cells
	// and the reporter cell takes a row of its own, with no line on its left.
	fkuWant(t, "phone cells", base, ".person-stats > *", "flex", "1 1 0")
	fkuWant(t, "phone four cells", phoneOnly, ".person-stats:has(> .person-stars) > .person-tag-group", "flex", "1 0 100%")
	fkuWant(t, "phone four cells", phoneOnly, ".person-stats:has(> .person-stars) > .person-tag-group", "border-left", "0")
	// The three cells of the phone share the width equally: the reporter cell has
	// no flex-grow override of its own, and the chip is drawn at the caption size
	// so it fits (the mockup draws it at 12px on the phone).
	for _, r := range phoneOnly {
		if _, ok := r.Decls["flex-grow"]; ok {
			t.Errorf("phone only rule %q declares flex-grow, the cells share the width equally", r.Head)
		}
	}
	fkuWant(t, "phone chip", phoneOnly, ".person-stats .person-tag", "font-size", "var(--font-size-caption)")
	fkuWant(t, "phone chip", phoneOnly, ".person-stats .person-tag", "line-height", "var(--line-height-caption)")

	// The details line: stacked on the phone, side by side on the laptop, muted.
	fkuWant(t, "phone details", base, ".person-facts", "flex-direction", "column")
	fkuWant(t, "laptop details", laptop, ".person-facts", "flex-direction", "row")
	fkuWant(t, "laptop details", laptop, ".person-facts", "justify-content", "center")
	fkuWant(t, "details", base, ".person-meta", "color", "var(--color-text-muted)")
	fkuWant(t, "clock icon", base, "main.person-page .app-icon--clock", "mask-image", `url("/static/icons/clock.svg")`)
	// The state line and Your rating are centred (the header centres its text).
	fkuWant(t, "state line", base, ".person-state-line", "color", "var(--color-text-muted)")
	fkuWant(t, "rating", base, ".person-rating", "align-items", "center")
	fkuWant(t, "rating row", base, ".person-rating .stars-control__row", "justify-content", "center")

	// Recent reports: its heading stays left aligned, the rows box is unchanged,
	// and the empty and the error state sit in a bordered box on both sizes.
	if v, ok := fkuDecl(base, ".person-reports__title", "text-align"); ok {
		t.Errorf("the Recent reports heading declares text-align %s, it stays left aligned", v)
	}
	fkuWant(t, "state box", base, ".person-reports__state", "border", "1px solid var(--color-border)")
	fkuWant(t, "state box", base, ".person-reports__state", "border-radius", "12px")
	fkuWant(t, "state box", base, ".person-reports__state", "padding", "var(--space-lg) var(--space-md)")
	fkuWant(t, "state box", base, ".person-reports__state", "text-align", "left")
	fkuWant(t, "state box", base, ".person-reports__state", "align-items", "flex-start")
	fkuWant(t, "state heading", base, ".person-reports__state h3", "color", "var(--color-text-muted)")
	fkuWant(t, "state heading", base, ".person-reports__state h3", "font-size", "var(--font-size-body)")
	fkuWant(t, "state heading", base, ".person-reports__state h3", "font-weight", "var(--font-weight-regular)")
	for _, prop := range []string{"border", "border-radius", "padding", "text-align"} {
		if v, ok := fkuDecl(laptop, ".person-reports__state", prop); ok {
			t.Errorf("the laptop .person-reports__state declares %s: %s (the box is the same on both sizes)", prop, v)
		}
	}
	fkuWant(t, "rows box", laptop, ".person-report-box", "border", "1px solid var(--color-border)")
	fkuWant(t, "rows box", laptop, ".person-report-box", "border-radius", "12px")

	// Forced colours keeps the lines between the cells, the box edge and the
	// button borders.
	if len(forced) == 0 {
		t.Errorf("person.css has no forced-colors block")
	}
	fkuWant(t, "forced cell line", forced, ".person-stats > * + *", "border-left", "1px solid CanvasText")
	fkuWant(t, "forced state box", forced, ".person-reports__state", "border", "1px solid CanvasText")
	fkuWant(t, "forced buttons", forced, ".person-actions .btn", "border", "1px solid ButtonText")
	fkuWant(t, "forced bell", forced, ".person-bell", "border", "1px solid ButtonText")

	// Whole file: no CSS order property, no hex colour outside the chip tint
	// block, no new custom property. A property is matched by its exact name,
	// never by substring, because border: contains order.
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`)
	for _, r := range rules {
		if _, ok := r.Decls["order"]; ok {
			t.Errorf("rule %q declares the order property", r.Head)
		}
		named := strings.Contains(r.Head, ".person-stats") || strings.Contains(r.Head, ".person-facts") || strings.Contains(r.Head, ".person-header")
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
	for _, m := range regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		if !strings.HasPrefix(m[1], "--p-chip-") {
			t.Errorf("person.css defines the new custom property %s", m[1])
		}
	}
}
