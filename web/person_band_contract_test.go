package web

// Quick 261108-fku (owner decision 2026-11-08, testing on the laptop) drew the
// profile page header as one band across the top from 900px. Amended
// 2026-11-13 by quick 261113-k9t (owner decision 2026-11-13, the centred
// profile): the band is replaced on both sizes by the centred header. Amended
// again 2026-11-13 by quick 261113-t4y (the compact profile): the centred header
// keeps its shape and takes about half the height, and the pins below describe
// that. These tests read the shipped template and
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

// TestPersonProfileCompact pins quick 261113-t4y (owner decision 2026-11-13,
// the compact profile, mockup profile-compact: "instead of taking more than
// half of the page for just personal info we can reduce it"). It replaces the
// TestPersonProfileCentred pins of quick 261113-k9t: the header stays one
// centred column on the laptop and on the phone, but it is about half as tall:
// a 64px letter, a 20px username, one bordered group of three counts, the
// reporter tag and its sentence on one line under it, Member since and the home
// area on one line joined by a dot, and the action row last. Amended
// 2026-11-13 by that decision.
func TestPersonProfileCompact(t *testing.T) {
	tmpl := p421ReadTemplate(t, "person.html.tmpl")
	// The DOM order is the reading order: the letter, the username, the name, the
	// edit panel (your own page), the counts group, the tag line, the details line,
	// the private line, the action row, Your rating, then the action error line.
	fkuIDs(t, "the header order", tmpl,
		`id="person-header"`,
		`id="person-disc"`,
		`<h1 id="person-username"`,
		`class="person-username person-username--phone"`,
		`id="person-name"`,
		`id="person-edit-panel"`,
		`id="person-stats" class="person-stats"`,
		`id="person-stars"`,
		`id="person-followers-button"`,
		`id="person-following-button"`,
		`id="person-tag-line" class="person-tag-line"`,
		`id="person-tag"`,
		`id="person-tag-sentence"`,
		`id="person-facts" class="person-facts"`,
		`id="person-member-since"`,
		`id="person-home-area"`,
		`id="person-state-line"`,
		`id="person-actions"`,
		`id="person-edit"`,
		`id="person-follow"`,
		`id="person-unfollow"`,
		`id="person-bell"`,
		`id="person-more"`,
		`id="person-more-menu"`,
		`id="person-rating"`,
		`id="person-action-error"`,
		`</header>`,
	)
	// The counts group holds the stars group (when present) and the two count
	// buttons, and nothing else: the reporter tag has its own line under it.
	statsAt := strings.Index(tmpl, `id="person-stats"`)
	tagLineAt := strings.Index(tmpl, `id="person-tag-line"`)
	if statsAt < 0 || tagLineAt < statsAt {
		t.Fatalf("the counts group or the tag line was not found")
	}
	stats := tmpl[statsAt:tagLineAt]
	for _, id := range []string{"person-stars", "person-followers-button", "person-following-button"} {
		if strings.Count(stats, `id="`+id+`"`) != 1 {
			t.Errorf("the counts group draws %s %d times, want once", id, strings.Count(stats, `id="`+id+`"`))
		}
	}
	if strings.Contains(stats, `id="person-tag"`) {
		t.Errorf("the reporter tag is inside the counts group, want its own line under it")
	}
	// The details line joins the two facts by a dot and draws no icon.
	factsAt := strings.Index(tmpl, `id="person-facts"`)
	factsEnd := strings.Index(tmpl[factsAt:], "</p>")
	if factsAt < 0 || factsEnd < 0 {
		t.Fatalf("the details line was not found")
	}
	facts := tmpl[factsAt : factsAt+factsEnd]
	for _, want := range []string{`id="person-member-since"`, `id="person-facts-dot"`, `id="person-home-area"`, "·"} {
		if !strings.Contains(facts, want) {
			t.Errorf("the details line lacks %q", want)
		}
	}
	if strings.Contains(facts, "app-icon") {
		t.Errorf("the details line draws an icon, want none (the mockup has none)")
	}
	if !strings.Contains(facts, `id="person-facts-dot" class="person-facts__dot" aria-hidden="true"`) {
		t.Errorf("the dot between the two facts is not aria-hidden")
	}
	// No wrapper of the earlier layouts is left in the markup or the stylesheet.
	css := stripCSSComments(p421ReadStatic(t, "css/person.css"))
	for _, gone := range []string{"person-band", "person-record", "person-ident", "person-counts", "person-tag-group", "person-meta__icon"} {
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
	forced := fkuForced(rules)
	if len(laptop) == 0 {
		t.Fatalf("person.css has no min-width: 900px block")
	}

	// The page: still a full width column (03.4 R6), never a grid. 20px above the
	// letter, 16px between the header's line and Recent reports.
	if v, ok := fkuDecl(laptop, ".person-page", "grid-template-columns"); ok {
		t.Errorf("the laptop .person-page declares grid-template-columns %q (one full width column, 03.4 R6)", v)
	}
	if v, _ := fkuDecl(laptop, ".person-page", "display"); v == "grid" {
		t.Errorf("the laptop .person-page is a grid")
	}
	fkuWant(t, "phone page", base, ".person-page", "display", "flex")
	fkuWant(t, "phone page", base, ".person-page", "flex-direction", "column")
	fkuWant(t, "page gap", base, ".person-page", "gap", "var(--space-md)")
	fkuWant(t, "phone page", base, ".person-page", "padding", "calc(var(--space-md) + var(--space-xs)) var(--space-md) var(--space-md)")
	fkuWant(t, "laptop page", laptop, ".person-page", "padding", "calc(var(--space-md) + var(--space-xs)) var(--space-lg) var(--space-lg)")
	if v, ok := fkuDecl(laptop, ".person-page", "gap"); ok {
		t.Errorf("the laptop .person-page declares gap %q, the gap is the same on both sizes", v)
	}

	// The header is a centred column on both sizes, with a line under it that runs
	// to the page edges (negative side margins equal to the page padding).
	fkuWant(t, "header", base, ".person-header:not([hidden])", "display", "flex")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "flex-direction", "column")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "align-items", "center")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "text-align", "center")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "border-bottom", "1px solid var(--color-border)")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "padding", "0 var(--space-md) var(--space-md)")
	fkuWant(t, "header", base, ".person-header:not([hidden])", "margin", "0 calc(var(--space-md) * -1)")
	fkuWant(t, "laptop header", laptop, ".person-header:not([hidden])", "padding", "0 var(--space-lg) var(--space-md)")
	fkuWant(t, "laptop header", laptop, ".person-header:not([hidden])", "margin", "0 calc(var(--space-lg) * -1)")
	// The letter is 64px with 8px under it on both sizes; the username is 20px.
	fkuWant(t, "letter", base, ".person-disc", "width", "var(--space-3xl)")
	fkuWant(t, "letter", base, ".person-disc", "height", "var(--space-3xl)")
	fkuWant(t, "letter", base, ".person-disc", "margin-bottom", "var(--space-sm)")
	for _, prop := range []string{"width", "height"} {
		if v, ok := fkuDecl(laptop, ".person-disc", prop); ok {
			t.Errorf("the laptop letter declares %s %q, the letter is 64px on both sizes", prop, v)
		}
	}
	fkuWant(t, "username", base, ".person-username", "font-size", "1.25rem")
	fkuWant(t, "username", base, ".person-username", "font-weight", "var(--font-weight-semibold)")
	if v, ok := fkuDecl(laptop, "h1.person-username", "font-size"); ok {
		t.Errorf("the laptop h1 declares font-size %q, the username is 20px on both sizes", v)
	}
	fkuWant(t, "laptop username", laptop, "h1.person-username", "display", "block")
	fkuWant(t, "phone h1", base, "h1.person-username", "display", "none")
	fkuWant(t, "laptop phone line", laptop, ".person-username--phone", "display", "none")
	fkuWant(t, "name", base, ".person-name", "color", "var(--color-text-muted)")

	// The action row is centred; the laptop buttons are small (quick 261109-n25
	// stays: 32px, the label font size, the radius of .btn left alone) and the
	// phone buttons keep the 44px of .btn and the bell.
	fkuWant(t, "action row", base, ".person-actions", "justify-content", "center")
	fkuWant(t, "action row", base, ".person-actions", "margin-top", "var(--space-12)")
	fkuWant(t, "small buttons", laptop, ".person-header .person-actions .btn", "min-height", "32px")
	fkuWant(t, "small buttons", laptop, ".person-header .person-actions .btn", "padding", "0 var(--space-12)")
	fkuWant(t, "small buttons", laptop, ".person-header .person-actions .btn", "font-size", "var(--font-size-label)")
	if v, ok := fkuDecl(laptop, ".person-header .person-actions .btn", "border-radius"); ok {
		t.Errorf("the small buttons change the radius: %s", v)
	}
	fkuWant(t, "small done", laptop, ".person-header .person-edit__done", "min-height", "32px")
	for _, prop := range []string{"width", "height", "min-width", "min-height"} {
		fkuWant(t, "small bell", laptop, ".person-header .person-bell:not([hidden])", prop, "32px")
		fkuWant(t, "phone bell", base, ".person-bell:not([hidden])", prop, "var(--touch-target-min)")
	}
	// The More menu hangs from the right edge of the row, under More, on both
	// sizes, so it stays inside the viewport at 320px.
	fkuWant(t, "menu", base, ".person-menu:not([hidden])", "right", "0")
	fkuWant(t, "menu", base, ".person-menu:not([hidden])", "left", "auto")
	fkuWant(t, "menu", base, ".person-menu:not([hidden])", "min-width", "200px")

	// The counts group: one bordered group of three cells (a 10px radius, never a
	// pill), each at least 44px tall, a 1px line between the cells, the number
	// above its word, the whole group centred.
	fkuWant(t, "counts group", base, ".person-stats", "display", "inline-flex")
	fkuWant(t, "counts group", base, ".person-stats", "align-self", "center")
	fkuWant(t, "counts group", base, ".person-stats", "border", "1px solid var(--color-border)")
	fkuWant(t, "counts group", base, ".person-stats", "border-radius", "10px")
	fkuWant(t, "counts group", base, ".person-stats", "margin-top", "var(--space-12)")
	fkuWant(t, "cell line", base, ".person-stats > * + *", "border-left", "1px solid var(--color-border)")
	fkuWant(t, "cell", base, ".person-stats > *", "flex-direction", "column")
	fkuWant(t, "cell", base, ".person-stats > *", "align-items", "center")
	fkuWant(t, "cell", base, ".person-stats > *", "min-height", "var(--touch-target-min)")
	fkuWant(t, "count button", base, ".person-count", "min-height", "var(--touch-target-min)")
	fkuWant(t, "count button", base, ".person-count", "border", "0")
	fkuWant(t, "count button", base, ".person-count", "background", "transparent")
	fkuWant(t, "count button focus", base, ".person-count:focus-visible", "outline-offset", "-2px")
	fkuWant(t, "count number", base, ".person-count__number", "font-weight", "var(--font-weight-semibold)")
	fkuWant(t, "count word", base, ".person-count__word", "color", "var(--color-text-muted)")
	if v, ok := fkuDecl(laptop, ".person-count", "min-height"); ok {
		t.Errorf("the laptop shrinks the counts' hit area: %s", v)
	}
	// The stars cell draws the star and the number with Stars under them.
	fkuWant(t, "stars cell", base, ".person-stars:not([hidden])", "display", "grid")
	fkuWant(t, "stars word", base, ".person-stars__word", "grid-column", "1 / -1")
	// The tag line: the chip and its sentence, 10px under the counts group. The
	// sentence wraps under the chip on the phone and sits beside it on the laptop.
	fkuWant(t, "tag line", base, ".person-tag-line", "margin-top", "10px")
	fkuWant(t, "tag sentence", base, ".person-tag-sentence", "color", "var(--color-text-muted)")
	fkuWant(t, "tag chip", base, ".person-tag", "overflow-wrap", "anywhere")
	fkuWant(t, "laptop tag chip", laptop, ".person-tag", "white-space", "nowrap")
	fkuWant(t, "laptop tag sentence", laptop, ".person-tag-sentence", "display", "inline")

	// The details line: one muted line, 10px under the tag line, no icons.
	fkuWant(t, "details", base, ".person-facts", "color", "var(--color-text-muted)")
	fkuWant(t, "details", base, ".person-facts", "margin", "10px 0 0")
	// The state line and Your rating are centred (the header centres its text).
	fkuWant(t, "state line", base, ".person-state-line", "color", "var(--color-text-muted)")
	fkuWant(t, "rating", base, ".person-rating", "align-items", "center")
	fkuWant(t, "rating row", base, ".person-rating .stars-control__row", "justify-content", "center")

	// The edit panel: a bordered list of three rows and Done, 480px at most, and
	// while it is open the counts, the tag line, the details, the private line,
	// the action row and Your rating are not drawn.
	fkuWant(t, "edit panel", base, ".person-edit:not([hidden])", "display", "flex")
	fkuWant(t, "edit panel", base, ".person-edit:not([hidden])", "flex-direction", "column")
	fkuWant(t, "edit box", base, ".person-edit__box", "max-width", "480px")
	fkuWant(t, "edit row", base, ".person-edit .place-row", "min-height", "48px")
	fkuWant(t, "edit row line", base, ".person-edit .place-box > * + *", "border-top", "1px solid var(--color-border)")
	for _, part := range []string{".person-stats", ".person-tag-line", ".person-facts", ".person-state-line", ".person-actions", ".person-rating"} {
		fkuWant(t, "editing", base, ".person-header--editing > "+part, "display", "none")
	}

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

	// Forced colours keeps the lines between the cells, the group edge, the box
	// edge and the button borders.
	if len(forced) == 0 {
		t.Errorf("person.css has no forced-colors block")
	}
	fkuWant(t, "forced cell line", forced, ".person-stats > * + *", "border-left", "1px solid CanvasText")
	fkuWant(t, "forced group edge", forced, ".person-stats", "border-color", "CanvasText")
	fkuWant(t, "forced header line", forced, ".person-header:not([hidden])", "border-color", "CanvasText")
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
