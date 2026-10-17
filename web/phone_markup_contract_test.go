package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the markup contract of review finding WR-W01 (Phase 4 web
// review). Phase 4 wrapped the account header in .top-chrome, and the phone
// rule that named it as a child of body stopped matching anything, silently:
// the trigger then covered Close on the open place search. The tests here
// read the two pages that load phone.css (the map page and the Activity
// page), expand their partials, and check that every parent and child pair a
// phone rule names with the child combinator exists in that markup.

// phoneMarkupPages are the pages that load phone.css.
var phoneMarkupPages = []string{"index.html.tmpl", "profile.html.tmpl"}

// phoneScriptPlacedChildren are child combinator pairs whose child a script
// places, so the template markup cannot show them. Each entry says which.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-16, D-17):
// the List and Map switch that feed.js appended to the shell is retired, so no
// child is placed by a script now and the table is empty. Every child
// combinator pair a phone rule names must be in the template markup. The Phase
// 3.1 and Phase 4 hands-on tests are walked on this layout after Phase 3.4 is
// built.
var phoneScriptPlacedChildren = map[string]string{}

type phoneMarkupElem struct {
	tag     string
	id      string
	classes map[string]bool
}

type phoneMarkupPair struct {
	parent, child phoneMarkupElem
}

var (
	phoneIncludeRE = regexp.MustCompile(`\{\{\s*template\s+"([^"]+)"\s*\.?\s*\}\}`)
	phoneActionRE  = regexp.MustCompile(`(?s)\{\{.*?\}\}`)
	phoneCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)
	phoneTagRE     = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)((?:[^>"']|"[^"]*"|'[^']*')*)>`)
	phoneClassRE   = regexp.MustCompile(`\bclass="([^"]*)"`)
	phoneIDRE      = regexp.MustCompile(`\bid="([^"]*)"`)
)

var phoneVoidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

// phoneExpandTemplate inlines the partials a page includes, two levels deep,
// then drops every remaining template action, so both branches of an if stay.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the two
// app bars are named blocks, app_topbar and app_bottombar, of one file,
// app_bar.html.tmpl, so an include of either name is expanded to its block of
// that file instead of a file of its own name. The Phase 3.1 and Phase 4
// hands-on tests are walked on this layout after Phase 3.4 is built.
func phoneExpandTemplate(t *testing.T, name string) string {
	t.Helper()
	text := p13ReadTemplate(t, name)
	for level := 0; level < 2; level++ {
		text = phoneIncludeRE.ReplaceAllStringFunc(text, func(m string) string {
			inc := phoneIncludeRE.FindStringSubmatch(m)[1]
			switch inc {
			// Amended 2026-10-17 by Phase 3.4 plan 26 (code review part 3 WR-05):
			// account_header.html.tmpl now defines account_header_menu, the menu
			// markup, and its own root is a header around one call to it. A
			// call to the menu expands to the defined block, and an include of
			// the file expands to its root only, so the markup appears once.
			case "account_header_menu":
				whole := p13ReadTemplate(t, "account_header.html.tmpl")
				_, rest, ok := strings.Cut(whole, `{{define "account_header_menu"}}`)
				if !ok {
					t.Fatalf("account_header.html.tmpl defines no account_header_menu")
				}
				body, _, _ := strings.Cut(rest, "{{end}}")
				return body
			case "account_header.html.tmpl":
				whole := p13ReadTemplate(t, "account_header.html.tmpl")
				_, root, ok := strings.Cut(whole, "{{end}}")
				if !ok {
					t.Fatalf("account_header.html.tmpl has no root after its menu block")
				}
				return root
			case "app_topbar", "app_bottombar":
				bar := p13ReadTemplate(t, "app_bar.html.tmpl")
				top, bottom, ok := strings.Cut(bar, `{{define "app_bottombar"}}`)
				if !ok {
					t.Fatalf("app_bar.html.tmpl has no app_bottombar block")
				}
				if inc == "app_bottombar" {
					return bottom
				}
				_, topBlock, ok := strings.Cut(top, `{{define "app_topbar"}}`)
				if !ok {
					t.Fatalf("app_bar.html.tmpl has no app_topbar block")
				}
				return topBlock
			}
			return p13ReadTemplate(t, inc)
		})
	}
	text = phoneCommentRE.ReplaceAllString(text, "")
	return phoneActionRE.ReplaceAllString(text, "")
}

// phoneMarkupPairs walks the expanded markup and returns every element with
// its direct parent.
func phoneMarkupPairs(markup string) []phoneMarkupPair {
	var pairs []phoneMarkupPair
	var stack []phoneMarkupElem
	for _, m := range phoneTagRE.FindAllStringSubmatch(markup, -1) {
		closing, tag, attrs := m[1] == "/", strings.ToLower(m[2]), m[3]
		if closing {
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag == tag {
					stack = stack[:i]
					break
				}
			}
			continue
		}
		el := phoneMarkupElem{tag: tag, classes: map[string]bool{}}
		if c := phoneClassRE.FindStringSubmatch(attrs); c != nil {
			for _, name := range strings.Fields(c[1]) {
				el.classes[name] = true
			}
		}
		if id := phoneIDRE.FindStringSubmatch(attrs); id != nil {
			el.id = id[1]
		}
		if len(stack) > 0 {
			pairs = append(pairs, phoneMarkupPair{parent: stack[len(stack)-1], child: el})
		}
		if !phoneVoidTags[tag] && !strings.HasSuffix(strings.TrimSpace(attrs), "/") {
			stack = append(stack, el)
		}
	}
	return pairs
}

// phoneCompound is one simple selector compound: a tag, an id and classes.
// Attribute tests and pseudo classes are dropped, so a compound matches every
// state of the element it names.
type phoneCompound struct {
	tag, id string
	classes []string
}

// phoneStripGroups removes every (...) and [...] group, nesting aware.
func phoneStripGroups(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(', '[':
			depth++
			continue
		case ')', ']':
			depth--
			continue
		}
		if depth == 0 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func phoneParseCompound(raw string) phoneCompound {
	s := phoneStripGroups(raw)
	// Drop pseudo classes and elements, whose groups are already gone.
	s = regexp.MustCompile(`::?[a-zA-Z-]+`).ReplaceAllString(s, "")
	var c phoneCompound
	for _, tok := range regexp.MustCompile(`[.#]?[a-zA-Z0-9_-]+`).FindAllString(s, -1) {
		switch tok[0] {
		case '.':
			c.classes = append(c.classes, tok[1:])
		case '#':
			c.id = tok[1:]
		default:
			c.tag = strings.ToLower(tok)
		}
	}
	return c
}

func (c phoneCompound) matches(el phoneMarkupElem) bool {
	if c.tag != "" && c.tag != el.tag {
		return false
	}
	if c.id != "" && c.id != el.id {
		return false
	}
	for _, name := range c.classes {
		if !el.classes[name] {
			return false
		}
	}
	return c.tag != "" || c.id != "" || len(c.classes) > 0
}

// phoneChildSteps splits one selector into its compounds and returns the
// raw text of every pair joined by the child combinator. Spaces and > inside
// a (...) or [...] group belong to the compound.
func phoneChildSteps(selector string) [][2]string {
	var compounds []string
	var combinators []string
	var cur strings.Builder
	depth := 0
	pending := ""
	flush := func() {
		if cur.Len() > 0 {
			if len(compounds) > 0 {
				if pending == "" {
					pending = " "
				}
				combinators = append(combinators, pending)
			}
			compounds = append(compounds, cur.String())
			cur.Reset()
			pending = ""
		}
	}
	for _, r := range selector {
		switch {
		case r == '(' || r == '[':
			depth++
			cur.WriteRune(r)
		case r == ')' || r == ']':
			depth--
			cur.WriteRune(r)
		case depth == 0 && (r == ' ' || r == '\n' || r == '\t'):
			flush()
		case depth == 0 && (r == '>' || r == '+' || r == '~'):
			flush()
			pending = string(r)
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	var steps [][2]string
	for i, comb := range combinators {
		if comb == ">" {
			steps = append(steps, [2]string{compounds[i], compounds[i+1]})
		}
	}
	return steps
}

// TestPhoneRuleHeadsMatchTheMarkup pins that every child combinator pair in
// a phone rule exists in the markup of a page that loads phone.css, whenever
// the parent is markup at all (a parent only a script builds, such as a
// Leaflet pane, is out of reach of a template check and skipped). It also
// pins the two rules the finding was about on both pages: the account
// header is a child of .top-chrome and the theme toggle a child of body.
func TestPhoneRuleHeadsMatchTheMarkup(t *testing.T) {
	pagePairs := map[string][]phoneMarkupPair{}
	var all []phoneMarkupPair
	for _, page := range phoneMarkupPages {
		pairs := phoneMarkupPairs(phoneExpandTemplate(t, page))
		if len(pairs) == 0 {
			t.Fatalf("%s: the markup walker found no elements", page)
		}
		pagePairs[page] = pairs
		all = append(all, pairs...)
	}
	parentInMarkup := func(c phoneCompound) bool {
		for _, p := range all {
			if c.matches(p.parent) {
				return true
			}
		}
		return false
	}
	pairIn := func(pairs []phoneMarkupPair, parent, child phoneCompound) bool {
		for _, p := range pairs {
			if parent.matches(p.parent) && child.matches(p.child) {
				return true
			}
		}
		return false
	}

	checked := 0
	for _, r := range phoneRules(t) {
		if r.Media != phoneBlockMedia {
			continue
		}
		for _, head := range p16HeadParts(r.Head) {
			for _, step := range phoneChildSteps(head) {
				parent, child := phoneParseCompound(step[0]), phoneParseCompound(step[1])
				if !parentInMarkup(parent) {
					continue
				}
				key := strings.TrimSpace(phoneStripGroups(step[0])) + " > " + strings.TrimSpace(phoneStripGroups(step[1]))
				if _, ok := phoneScriptPlacedChildren[key]; ok {
					continue
				}
				checked++
				if !pairIn(all, parent, child) {
					t.Errorf("phone.css head %q: no %q is a direct child of %q in %v, so the rule matches nothing",
						head, step[1], step[0], phoneMarkupPages)
				}
			}
		}
	}
	if checked < 5 {
		t.Errorf("only %d child combinator pairs were checked against the markup, the walker is broken", checked)
	}

	// The two rules of the finding must hold on both pages, and their exact
	// heads must be in the phone block.
	heads := map[string]bool{}
	for _, r := range phoneRules(t) {
		if r.Media == phoneBlockMedia {
			for _, part := range p16HeadParts(r.Head) {
				heads[part] = true
			}
		}
	}
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10 and
	// D-11): the account header is a child of the shared top bar, which is the
	// child of .top-chrome, so the pair of the finding is .app-topbar >
	// .app-header. The Phase 3.1 and Phase 4 hands-on tests are walked on this
	// layout after Phase 3.4 is built.
	//
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-08, UX-01
	// as amended): the light and dark button is a child of the shared top bar on
	// these pages (it floats as a child of body only on the Low data view and
	// the moderator screen, which do not load phone.css), so the second pair is
	// .app-topbar > .theme-toggle. The Phase 3.1 and Phase 4 hands-on tests are
	// walked on this layout after Phase 3.4 is built.
	for _, want := range [][2]string{{".app-topbar", ".app-header"}, {".app-topbar", ".theme-toggle"}} {
		head := want[0] + " > " + want[1]
		if !heads[head] {
			t.Errorf("phone.css has no %q rule", head)
		}
		for _, page := range phoneMarkupPages {
			if !pairIn(pagePairs[page], phoneParseCompound(want[0]), phoneParseCompound(want[1])) {
				t.Errorf("%s: %s is not a direct child of %s, so the phone rule %q matches nothing", page, want[1], want[0], head)
			}
		}
	}
}

// TestPhoneKeepsTheAccountTriggerAndDropsTheThemeToggle pins what the rules
// say (owner decision of 2026-10-15 for the toggle, Phase 4 P4-A4 for the
// trigger): the toggle is not displayed at all on a phone, so it also leaves
// the tab order, and the open search row keeps the collapsed row's right
// inset.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10 and D-11):
// the phone no longer draws the person button. The title bar holds the
// Settings gear (and the back arrow where a screen has one) instead, so the
// account header is hidden inside the top bar on a phone and the gear and
// back controls are shown. The theme toggle stays hidden on the phone (D-08).
// The Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
// Phase 3.4 is built.
func TestPhoneKeepsTheAccountTriggerAndDropsTheThemeToggle(t *testing.T) {
	var header, toggle, layer, gear *p16CSSRule
	for _, r := range phoneRules(t) {
		if r.Media != phoneBlockMedia {
			continue
		}
		rule := r
		for _, part := range p16HeadParts(r.Head) {
			switch part {
			case ".app-topbar > .app-header":
				header = &rule
			case ".app-topbar > .theme-toggle":
				toggle = &rule
			case ".pane--map > .map-tools":
				layer = &rule
			case ".app-topbar > .app-topbar__gear":
				gear = &rule
			}
			// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-11,
			// D-04): there is no open search row any more, so the check that no
			// open state rule moves the layer's right edge is replaced by one
			// that the old open state attribute is not used by any phone rule.
			// The Phase 3.1 and Phase 4 hands-on tests are walked on this layout
			// after Phase 3.4 is built.
			if strings.Contains(part, "data-search-open") {
				t.Errorf("phone.css %q uses the retired open search state", part)
			}
		}
	}
	if header == nil || header.Decls["display"] != "none" {
		t.Errorf("phone.css needs .app-topbar > .app-header { display: none } (the gear replaces the person button), got %v", header)
	}
	if gear == nil || gear.Decls["display"] != "flex" {
		t.Errorf("phone.css needs .app-topbar > .app-topbar__gear shown as a flex box, got %v", gear)
	}
	if toggle == nil || toggle.Decls["display"] != "none" {
		t.Errorf("phone.css needs .app-topbar > .theme-toggle { display: none } (Phase 3.4, D-08), got %v", toggle)
	}
	if layer == nil || p19NoSpace(layer.Decls["right"]) != p19NoSpace("calc(var(--space-md) + var(--touch-target-min) + var(--space-sm))") {
		t.Errorf("phone.css .pane--map > .map-tools right must keep its inset, got %v", layer)
	}
}

// TestPhoneFollowsTheDeviceTheme pins the script side of the owner decision:
// theme.js names the phone breakpoint, drops the root theme attribute there
// from inside applyMode, and persists nothing on a phone width.
func TestPhoneFollowsTheDeviceTheme(t *testing.T) {
	raw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading %s: %v", themeJSPath, err)
	}
	code := stripJSComments(string(raw))
	if !strings.Contains(code, "matchMedia('(max-width: 899.98px)')") {
		t.Errorf("theme.js does not watch the phone breakpoint")
	}
	apply := jsFunctionBody(t, code, "applyMode(mode)")
	if !strings.Contains(apply, "removeAttribute('data-theme')") || !strings.Contains(apply, "isPhone()") {
		t.Errorf("applyMode must drop the theme attribute on a phone width, got %q", apply)
	}
	if n := strings.Count(code, "removeAttribute('data-theme')"); n != 1 {
		t.Errorf("theme.js removes the theme attribute in %d places, want 1 (applyMode)", n)
	}
	if !strings.Contains(code, "if (!isPhone()) {\n    persistMode(currentMode);") {
		t.Errorf("theme.js must not persist the first visit mode on a phone width")
	}
}
