package web

import (
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file guards the owner's rules for the app layout across every screen
// (Phase 3.4, plan 16, 03.4-CONTEXT D-01 to D-09 and the three mockup
// overrides D-29, O-1 and O-2). The per screen contract tests pin what each
// screen draws; the tests here walk every stylesheet, template and script, so
// a later change on any screen trips the same rule.
//
// Every guard reads the embedded trees (StaticFS and TemplatesFS) and finds
// rules by selector, never by line position or by grid columns, so a layout
// rearrangement (the laptop Nearby columns of plan 20) cannot trip them. Each
// exception is an entry in a named allow list, and every entry carries a
// written reason. The only way past a guard is a new entry with a reason.
//
// Honest limit of these claims: they prove the declared CSS and the shipped
// bytes. They cannot prove how a screen looks in a browser; the owner's
// viewing session and the phase hands-on tests do that.

// layoutAllow is one allow list entry: the stylesheet, one whole part of a
// selector list, and the written reason the rule is allowed. The reason names
// the owner decision or the UI-SPEC line behind it.
type layoutAllow struct {
	file   string
	head   string
	reason string
}

// layoutAllowed reports whether a rule part is on a list.
func layoutAllowed(list []layoutAllow, file, part string) bool {
	for _, a := range list {
		if a.file == file && a.head == part {
			return true
		}
	}
	return false
}

// layoutCheckReasons fails when an entry has no written reason, so an allow
// list cannot hold a bare exception.
func layoutCheckReasons(t *testing.T, name string, list []layoutAllow) {
	t.Helper()
	for _, a := range list {
		if strings.TrimSpace(a.reason) == "" || a.file == "" || a.head == "" {
			t.Errorf("%s: allow list entry %q in %q has no written reason", name, a.head, a.file)
		}
	}
}

// layoutStylesheets returns every stylesheet under static/css, in name order.
func layoutStylesheets(t *testing.T) []string {
	t.Helper()
	files, err := fs.Glob(StaticFS, "static/css/*.css")
	if err != nil || len(files) < 10 {
		t.Fatalf("expected the embedded stylesheets under static/css, got %d (%v)", len(files), err)
	}
	sort.Strings(files)
	return files
}

// layoutRules parses every stylesheet into rules with the shared parser.
func layoutRules(t *testing.T) []p16CSSRule {
	t.Helper()
	var all []p16CSSRule
	for _, file := range layoutStylesheets(t) {
		all = append(all, p16ParseCSS(file, p16ReadStatic(t, file))...)
	}
	return all
}

// layoutWalk calls fn for every embedded file under root with one of the
// wanted suffixes.
func layoutWalk(t *testing.T, fsys fs.FS, root string, suffixes []string, fn func(path, content string)) {
	t.Helper()
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		for _, suffix := range suffixes {
			if strings.HasSuffix(path, suffix) {
				raw, readErr := fs.ReadFile(fsys, path)
				if readErr != nil {
					return readErr
				}
				fn(path, string(raw))
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
}

// layoutStripNot removes every :not( ... ) group from a selector part, with
// balanced parentheses, so a rule that EXCLUDES a state is not read as a rule
// for that state.
func layoutStripNot(part string) string {
	for {
		i := strings.Index(part, ":not(")
		if i == -1 {
			return part
		}
		depth := 0
		end := -1
		for j := i + len(":not"); j < len(part); j++ {
			switch part[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					end = j
				}
			}
			if end != -1 {
				break
			}
		}
		if end == -1 {
			return part[:i]
		}
		part = part[:i] + part[end+1:]
	}
}

// ---------------------------------------------------------------------------
// D-07: no opacity on text, rows, cards or pins
// ---------------------------------------------------------------------------

// opacityAllowed lists the only rules that may declare an opacity other than 1
// outside an animation keyframe. A faded element is never text, a row, a card,
// a chip or a pin.
var opacityAllowed = []layoutAllow{
	{"static/css/auth.css", ".globe-bg",
		"the decorative globe behind the login screen: a mask picture with no text, drawn at 35% so it stays a quiet background (Phase 2 login gate). It is not text, a row, a card or a pin."},
}

// TestNoOpacityText pins D-07 and UI-SPEC R7: nothing is faded. No rule
// declares an opacity between 0 and 1 and none declares 0 outside a keyframe
// (a hidden element uses the hidden attribute), the filter opacity() function
// is not used, no template styles opacity inline and no script writes it.
func TestNoOpacityText(t *testing.T) {
	layoutCheckReasons(t, "opacityAllowed", opacityAllowed)

	checked := 0
	for _, r := range layoutRules(t) {
		value, ok := r.Decls["opacity"]
		if ok {
			checked++
			if strings.HasPrefix(r.Media, "@keyframes") {
				continue
			}
			number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err == nil && number == 1 {
				continue
			}
			parts := p16HeadParts(r.Head)
			allowed := len(parts) > 0
			for _, part := range parts {
				if !layoutAllowed(opacityAllowed, r.File, part) {
					allowed = false
				}
			}
			if !allowed {
				t.Errorf("%s: rule %q declares opacity %q; D-07 forbids faded text, rows, cards and pins "+
					"(mark a disputed or ended item with a grey icon and a status tag, never a fade)", r.File, r.Head, value)
			}
		}
		for prop, v := range r.Decls {
			if strings.Contains(v, "opacity(") {
				t.Errorf("%s: rule %q uses the opacity() function in %s: %q", r.File, r.Head, prop, v)
			}
		}
	}
	if checked < 3 {
		t.Fatalf("only %d opacity declarations were seen; the walk may have stopped reading the stylesheets", checked)
	}

	layoutWalk(t, TemplatesFS, "templates", []string{".tmpl"}, func(path, content string) {
		if strings.Contains(content, "opacity") {
			t.Errorf("%s names opacity; no template may fade anything", path)
		}
	})

	codeOpacity := regexp.MustCompile(`(?i)style\.opacity|setProperty\(\s*['"]opacity|['"]opacity['"]\s*:`)
	layoutWalk(t, StaticFS, "static/js", []string{".js"}, func(path, content string) {
		for i, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
				continue
			}
			if codeOpacity.MatchString(line) {
				t.Errorf("%s:%d writes opacity from script: %q", path, i+1, trimmed)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// D-04: a chosen option never gets a dark box, a dark outline or a heavy border
// ---------------------------------------------------------------------------

// chosenStateMarkers are the selector fragments that name a chosen, pressed,
// selected or current state. A :not( ... ) group is removed first, so a rule
// for the OTHER state is not read as a chosen rule.
var chosenStateMarkers = []string{
	`[aria-pressed="true"]`, `[aria-checked="true"]`, `[aria-selected="true"]`,
	`[aria-current`, `--selected`, `--chosen`, `:checked`, `map-pin--chosen`,
}

// chosenDarkAllowed lists the chosen rules that keep a dark look, each with its
// reason. D-04 is about chosen options (filters, tiles, cards, picks, boxes and
// pins); these are the rules that are not that.
var chosenDarkAllowed = []layoutAllow{
	{"static/css/main.css", ".category-tile--selected",
		"the Low data view's own chosen tile keeps its built dark look: that screen is kept as built and does not carry .post-window (UI-SPEC 5e, post_window_contract_test.go TestPostWindowChosenTileSoftFill). The Post a report window draws the soft grey fill."},
	{"static/css/app-bar.css", ".app-topbar__link[aria-current=\"page\"]",
		"the laptop top bar's current place is a short bottom underline in the text colour on a bar, not a box or an outline around an option (UI-SPEC 1c)."},
	{"static/css/phone.css", "body > .app-bar > .app-bar__item[aria-current=\"page\"]::before",
		"Allowed 2026-10-17 (plan 03.4-16): the phone bottom bar's current place is a 4px bar drawn above the item, in the text colour, never a box or a ring around it (UI-SPEC 1b). Forced colours draws it as CanvasText."},
	{"static/css/trust.css", ".vote-btn--confirm[aria-pressed=\"true\"]",
		"Allowed 2026-10-17 (plan 03.4-16): the Confirm button's pressed state is the Phase 3 inverted neutral fill (03-UI-SPEC A13). The report popup and detail sheet are kept as built in 3.4 (UI-SPEC: Not designed here); the owner has not asked to change the vote buttons. Open owner question: should a pressed vote also be a soft fill, like every other chosen item?"},
	{"static/css/trust.css", ".vote-btn--dispute[aria-pressed=\"true\"]",
		"Allowed 2026-10-17 (plan 03.4-16): the Dispute button's pressed state, identical to Confirm (03-UI-SPEC A13); same reason as the Confirm entry."},
	{"static/css/trust.css", ".vote-btn[aria-pressed=\"true\"]:disabled:not([hidden])",
		"Allowed 2026-10-17 (plan 03.4-16): a pressed vote button while busy keeps the inverted neutral fill of A13; same reason as the Confirm entry."},
	{"static/css/trust.css", ".vote-btn[aria-pressed=\"true\"][aria-disabled=\"true\"]:not([hidden])",
		"Allowed 2026-10-17 (plan 03.4-16): a pressed vote button that is too far keeps the inverted neutral fill of A13; same reason as the Confirm entry."},
	{"static/css/phone.css", ".leaflet-container .leaflet-detail-pane .vote-btn[aria-pressed=\"true\"]:not([hidden])",
		"Allowed 2026-10-17 (plan 03.4-16): the phone detail sheet's pressed vote button, the A13 inverted neutral fill; same reason as the Confirm entry."},
}

// heavyBorder is true for a border width of 2px or more in a declaration value.
var heavyBorder = regexp.MustCompile(`(^|[^0-9.])([2-9]|[1-9][0-9]+)(\.[0-9]+)?px`)

// TestNoDarkBorderOnChosenItems pins D-04 and UI-SPEC R4: no rule for a chosen
// state declares a border of 2px or more, any border, outline or shadow in the
// text colour, or the text colour as its fill. Chosen severity is a soft tint
// and every other chosen item is the soft grey fill (--color-surface).
func TestNoDarkBorderOnChosenItems(t *testing.T) {
	layoutCheckReasons(t, "chosenDarkAllowed", chosenDarkAllowed)

	seen := 0
	for _, r := range layoutRules(t) {
		// Forced colours mode replaces the app's colours with the reader's own
		// system palette and drops fills, so a chosen item can only show by a
		// border in a system colour there. The width rule is relaxed in that
		// block only; the text colour token is still refused everywhere.
		// Finding 2026-10-17: UI-SPEC says chosen items add 1px solid
		// Highlight, and feed.css draws the chosen chip as 2px solid CanvasText.
		// It is a forced colours fallback of Phase 3.1, left as built.
		forced := strings.Contains(r.Media, "forced-colors")
		for _, raw := range p16HeadParts(r.Head) {
			part := layoutStripNot(raw)
			chosen := false
			for _, marker := range chosenStateMarkers {
				if strings.Contains(part, marker) {
					chosen = true
				}
			}
			if !chosen {
				continue
			}
			seen++
			var problems []string
			for prop, value := range r.Decls {
				switch {
				case strings.HasPrefix(prop, "border") && prop != "border-radius":
					if strings.Contains(value, "var(--color-text)") {
						problems = append(problems, prop+": "+value+" (dark border)")
					}
					if !forced && heavyBorder.MatchString(value) {
						problems = append(problems, prop+": "+value+" (2px or wider border)")
					}
				case prop == "outline" || prop == "outline-color" || prop == "box-shadow":
					if strings.Contains(value, "var(--color-text)") {
						problems = append(problems, prop+": "+value+" (dark outline or ring)")
					}
				case prop == "background" || prop == "background-color":
					if strings.TrimSpace(value) == "var(--color-text)" {
						problems = append(problems, prop+": "+value+" (dark inverted fill)")
					}
				}
			}
			if len(problems) > 0 && !layoutAllowed(chosenDarkAllowed, r.File, raw) {
				sort.Strings(problems)
				t.Errorf("%s: chosen state rule %q: %s; D-04 allows only the soft grey fill or the severity tint "+
					"(or add an allow list entry with the owner's reason)", r.File, raw, strings.Join(problems, "; "))
			}
		}
	}
	if seen < 15 {
		t.Fatalf("only %d chosen state selector parts were seen; the markers or the walk may have stopped matching", seen)
	}
}

// ---------------------------------------------------------------------------
// D-03: every new control is at least 44px
// ---------------------------------------------------------------------------

// touchControls are the controls this phase added or rebuilt, as the last
// compound of their selector. Each must reach 44px in height through a rule
// that names it.
var touchControls = []string{
	// the bottom bar and the top bars
	".app-bar__item", ".app-topbar__link", ".app-topbar__post", ".app-topbar__bell",
	".app-topbar__gear", ".app-topbar__back", ".theme-toggle", ".account-trigger",
	// the bell list
	".bell-list__close",
	// Search a place and the filter chips
	".map-search__input", ".map-search__submit", ".map-chip",
	// Show and the sheet
	".show-button", ".show-menu__item", ".sheet__handle",
	// Post a report
	".post-window__close", ".category-tile", ".severity-box", ".happening-pick", ".call-box__button",
	// Settings, the windows, the reader and the first-time flow
	".place-row", ".app-window__close", ".app-window__quiet", ".app-window__option",
	".legal-close", ".btn",
	// My activity
	".activity-hub__row",
	// Amended 2026-10-17 by Phase 3.4 plan 26 (code review part 3 IN-02): the
	// notice strip's Read the disclaimer link is a standalone link of about
	// 20px, and UI-SPEC Spacing says every link is 44px. It was missing from
	// this list, so the guard did not see it. Added; no check weakened.
	".notice-strip__link",
}

// touchValue reads a height or min-height value as pixels: a px length, a
// spacing token, or a calc of those with plus and minus.
func touchValue(value string, tokens map[string]float64) float64 {
	value = strings.TrimSpace(value)
	if value == "var(--app-topbar-height)" {
		return 52
	}
	// calc( A + B ) and calc( A - B ) of px lengths and var() tokens.
	if inner, ok := strings.CutPrefix(value, "calc("); ok {
		inner = strings.TrimSuffix(inner, ")")
		sign := 1.0
		total := 0.0
		for _, field := range strings.Fields(inner) {
			switch field {
			case "+":
				sign = 1
			case "-":
				sign = -1
			default:
				total += sign * touchValue(field, tokens)
			}
		}
		return total
	}
	if name, ok := strings.CutPrefix(value, "var("); ok {
		name = strings.TrimSuffix(name, ")")
		return tokens[name]
	}
	if strings.HasSuffix(value, "px") {
		if n, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64); err == nil {
			return n
		}
	}
	return 0
}

// layoutLengthTokens reads the px custom properties declared on :root in
// main.css (the spacing scale and --touch-target-min).
func layoutLengthTokens(t *testing.T) map[string]float64 {
	t.Helper()
	tokens := map[string]float64{}
	for _, r := range p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css")) {
		if r.Head != ":root" {
			continue
		}
		for name, value := range r.Decls {
			if strings.HasPrefix(name, "--space-") || name == "--touch-target-min" {
				if n, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64); err == nil {
					tokens[name] = n
				}
			}
		}
	}
	if tokens["--touch-target-min"] != 44 {
		t.Fatalf("--touch-target-min resolves to %v, want 44", tokens["--touch-target-min"])
	}
	return tokens
}

// layoutLastCompound is the last compound selector of one selector part, with
// pseudo elements marked as not a control box.
func layoutLastCompound(part string) (string, bool) {
	part = layoutStripNot(part)
	if strings.Contains(part, "::") {
		return "", false
	}
	cut := strings.LastIndexAny(part, " >+~")
	return part[cut+1:], true
}

// touchRuleNames is true when the last compound is the control itself or the
// control with an added class or attribute (not a hover or focus state).
func touchRuleNames(compound, control string) bool {
	if compound == control {
		return true
	}
	if !strings.HasPrefix(compound, control) {
		return false
	}
	switch compound[len(control)] {
	case '.', '[':
		return true
	}
	return false
}

// TestNewControlsMeetTouchTarget pins D-03 and UI-SPEC D8: every control the
// phase added or rebuilt declares a height or min-height of 44px or more.
func TestNewControlsMeetTouchTarget(t *testing.T) {
	rules := layoutRules(t)
	tokens := layoutLengthTokens(t)
	for _, control := range touchControls {
		best := 0.0
		named := 0
		for _, r := range rules {
			if strings.HasPrefix(r.Media, "@keyframes") || strings.Contains(r.Media, "forced-colors") {
				continue
			}
			for _, raw := range p16HeadParts(r.Head) {
				compound, ok := layoutLastCompound(raw)
				if !ok || !touchRuleNames(compound, control) {
					continue
				}
				named++
				for _, prop := range []string{"min-height", "height"} {
					if value, has := r.Decls[prop]; has {
						if px := touchValue(value, tokens); px > best {
							best = px
						}
					}
				}
			}
		}
		if named == 0 {
			t.Errorf("%s: no rule names this control; rename the entry in touchControls if the class changed", control)
			continue
		}
		if best < 44 {
			t.Errorf("%s: the largest height or min-height declared is %.0fpx, want 44px or more (D-03, use --touch-target-min)", control, best)
		}
	}
}

// ---------------------------------------------------------------------------
// House rule: plain punctuation in the phase's new scripts
// ---------------------------------------------------------------------------

// phaseScripts are the scripts this phase added.
var phaseScripts = []string{
	"app-bar.js", "settings.js", "reader.js", "welcome.js", "position.js",
	"show-menu.js", "ended-cards.js", "ended-nearby.js", "sheet.js",
}

// TestNewScriptsUsePlainPunctuation holds the phase's new scripts to the house
// rule: no em dash and no en dash anywhere, comments included, because a
// comment is read by the owner too. The templates are guarded whole by
// TestUserVisibleCopyUsesPlainPunctuation.
func TestNewScriptsUsePlainPunctuation(t *testing.T) {
	for _, name := range phaseScripts {
		raw, err := fs.ReadFile(StaticFS, "static/js/"+name)
		if err != nil {
			t.Errorf("static/js/%s is missing: %v", name, err)
			continue
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.ContainsFunc(line, func(r rune) bool { return r == 0x2013 || r == 0x2014 }) {
				t.Errorf("static/js/%s:%d holds an em or en dash: %q", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// D-01: an element shown on one size only is named, with the owner's reason
// ---------------------------------------------------------------------------

// sizeOnlyNamed lists every selector part that the stylesheets hide on one
// size. The reason names the owner decision or UI-SPEC line that makes it size
// specific, or says why the rule is a state of one screen and not a feature
// that one size lacks (R1: the same features on both sizes, only the
// arrangement differs). A new size only element without an entry fails.
var sizeOnlyNamed = map[string]string{
	// Phone only, hidden at 900px and up (phoneBaseHideHeads and min-width rules).
	".feed-heading":      "phone only: the phone Nearby heading above the sheet list; the laptop draws its own list head (UI-SPEC 2a, D-06 laptop headings).",
	".app-bar":           "phone only: the four item bottom bar needs a phone's thumb reach; the laptop has the same places in its top bar (D-10, D-11, UI-SPEC 1b).",
	".app-topbar__title": "phone only: the title in the phone top bar; the laptop shows the same place name in the nav and the page heading (UI-SPEC 1b).",
	".app-topbar__back":  "phone only: the back arrow of a sub screen; the laptop uses the nav links (UI-SPEC 1b).",
	".app-topbar__gear":  "phone only: the Settings gear; the laptop reaches Settings from the person menu (D-11, UI-SPEC 1b).",
	".sheet__handle-row": "phone only: the phone list sheet and its handle; the laptop shows the list beside the map (D-16, D-17).",
	// Amended 2026-10-19 by Phase 3.2 plan 12 (D-23, UI-SPEC D3): the phone only class moved from the
	// Account section to its Log out row, because the section now draws on both sizes.
	".place-row-form--phone": "phone only: the Log out row of Settings; the laptop has Log out in the person menu (D-25, R2 once per size); the Account section draws on both sizes since Phase 3.2 (UI-SPEC D3).",
	".welcome__progress":     "phone only: the progress line of the first-time flow; the laptop shows the same steps in the side panel (D-06 whole screen on a laptop, UI-SPEC Surface 7).",
	".welcome__steps-item[data-state=\"done\"] .welcome__dot-num": "laptop side panel detail: a finished step shows a tick in place of its number; the phone shows the same progress in its progress line.",
	".welcome__dot-tick": "the tick on a finished step of the laptop side panel, hidden until the step is done and on the phone, which draws the progress line.",

	// Laptop only, hidden under 900px.
	"main.profile-page > .activity-heading": "laptop only: the My activity page heading in the left column; the phone has the top bar title and the hub (D-06, D-19).",
	"main.profile-page > .activity-hub":     "phone only: the My activity hub list of five parts; the laptop stacks the same five parts on one page (D-19, D-21).",
	".place-head":                           "laptop only: the page heading of Settings and Official alerts in the left column; the phone has the top bar title (D-06, UI-SPEC 1b).",
	".welcome__side":                        "laptop only: the dark side panel of the first-time window with the steps; the phone has the progress line (D-27, UI-SPEC Surface 7).",
	"body > .theme-toggle":                  "laptop only: the light and dark button; the phone follows the device setting (D-08, R8).",
	".app-topbar > .theme-toggle":           "laptop only: the light and dark button in the top bar; the phone follows the device setting (D-08, R8).",
	".app-topbar > .app-topbar__brand":      "laptop only: the Pinalert name at the left of the top bar; the phone bar carries the title (UI-SPEC 1c).",
	".app-topbar > .app-topbar__nav":        "laptop only: the nav links of the top bar; the phone has the bottom bar with the same places (D-10, UI-SPEC 1c).",
	".app-topbar > .app-topbar__post":       "laptop only: the Post a report link of the top bar; the phone has Report in the bottom bar (D-10, UI-SPEC 1c).",
	".app-topbar > .app-header":             "laptop only: the person button and menu in the top bar; the phone reaches the same items through Settings (D-11, D-25, UI-SPEC 1c).",
	"body > .notice-strip":                  "laptop only: the dark Unofficial strip on Nearby; the phone has no strip and no call box on its main screens (D-29, D-30).",

	// Rules that swap one element for another on one screen, or that hide a part of
	// one phone screen while another part shows. The feature stays on both sizes.
	// Amended 2026-10-17 by Phase 3.4 plan 26 (code review part 3 WR-04): this title is no longer
	// display none on any size. app-bar.css hides it by clipping so Nearby keeps its one h1 for screen
	// readers; the entry stays so a later display none on it would still be named and explained.
	".app-topbar:has(> .map-search) > .app-topbar__title":                                           "phone Nearby: Search a place takes the place of the title in the top bar (D-10, app_bar.html.tmpl); the title is clipped, not removed, so the h1 stays.",
	".app-topbar:has(> .map-search) > .app-topbar__spacer":                                          "phone Nearby: the spacer is not needed when Search a place fills the bar (D-10).",
	"#map .icon-badge--pin .icon-glyph":                                                             "phone: the pin is a 20px dot without a glyph; the laptop pin is the 44px badge with it (UI-SPEC D16, pins stay the built markers).",
	"main.profile-page:not([data-part=\"\"]):not([data-part=\"posted\"]) > .queue-strip":            "phone My activity: the waiting reports strip belongs to the Posted part only; the laptop stack shows it with that part (D-21).",
	"main.profile-page > .activity-part":                                                            "phone My activity: one part at a time, the hub chooses; the laptop stacks all five parts (D-19, D-21).",
	"main.profile-page > .activity-part .activity-part__title":                                      "phone My activity: the part title repeats the top bar title on the phone (R2 nothing repeated); the laptop stack needs it to tell the parts apart (D-21).",
	"main.profile-page > .activity-part .activity-cols":                                             "phone My activity: the column head row is for the aligned laptop table; phone cards carry their own labels (D-21).",
	".leaflet-container .leaflet-detail-pane .leaflet-popup-tip-container":                          "phone detail sheet: a sheet has no pointer tip; the laptop popup keeps it (kept as built, UI-SPEC Not designed here).",
	".leaflet-container .leaflet-detail-pane .popup-card__pills > .icon-badge":                      "phone detail sheet: the badge repeats the heading's glyph; kept as built (UI-SPEC Not designed here).",
	".leaflet-container .leaflet-detail-pane .popup-card__meta > .icon-glyph":                       "phone detail sheet: the meta glyph is dropped at sheet width; kept as built (UI-SPEC Not designed here).",
	"body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open=\"true\"]) .leaflet-control-zoom": "phone: an open detail sheet covers the map controls (R3 nothing under something floating); the laptop popup does not cover them.",
	"body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open=\"true\"]) .map-tools":            "phone: an open detail sheet covers the map tools; same reason as the zoom control.",
	"body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open=\"true\"]) .map-label":            "phone: an open detail sheet covers the map label; same reason as the zoom control.",
	"body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open=\"true\"]) #map-search-results":   "phone: an open detail sheet covers the search results; same reason as the zoom control.",
}

// sizeMedia matches the two media preludes that split the phone from the laptop.
var sizeMedia = regexp.MustCompile(`max-width:\s*899\.98px|min-width:\s*900px`)

// TestSizeOnlyElementsAreNamed pins D-01 and UI-SPEC R1: phone and laptop are
// the same product, so an element that one size hides must be on a named list
// with the reason. It reads every stylesheet for a display none inside a size
// media block, for a base display none that a size block turns back on, and for
// the base hide list of phone.css (phoneBaseHideHeads).
func TestSizeOnlyElementsAreNamed(t *testing.T) {
	for part, reason := range sizeOnlyNamed {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("sizeOnlyNamed entry %q has no written reason", part)
		}
	}

	rules := layoutRules(t)

	// Parts that a size block shows (display other than none).
	shown := map[string]bool{}
	for _, r := range rules {
		if !sizeMedia.MatchString(r.Media) {
			continue
		}
		if d, ok := r.Decls["display"]; ok && d != "none" {
			for _, part := range p16HeadParts(r.Head) {
				shown[part] = true
			}
		}
	}

	found := map[string]string{}
	for _, r := range rules {
		if r.Decls["display"] != "none" || strings.HasPrefix(r.Media, "@keyframes") {
			continue
		}
		for _, part := range p16HeadParts(r.Head) {
			if strings.Contains(part, "[hidden]") || strings.Contains(part, "visually-hidden") || strings.Contains(part, "::") {
				continue
			}
			switch {
			case sizeMedia.MatchString(r.Media):
				found[part] = r.File + " in " + r.Media
			case r.Media == "" && shown[part]:
				found[part] = r.File + " (base hide, shown by a size block)"
			}
		}
	}
	for _, head := range phoneBaseHideHeads {
		found[head] = "phone.css phoneBaseHideHeads"
	}
	if len(found) < 15 {
		t.Fatalf("only %d size specific hide rules were found; the walk may have stopped matching", len(found))
	}

	var missing []string
	for part, where := range found {
		if _, named := sizeOnlyNamed[part]; !named {
			missing = append(missing, part+"  ["+where+"]")
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("%s is hidden on one size and is not named in sizeOnlyNamed; D-01 says phone and laptop "+
			"are the same product, so name the owner decision that makes it size specific", m)
	}
}

// ---------------------------------------------------------------------------
// The three mockup overrides, pinned together (D-29, O-1, O-2)
// ---------------------------------------------------------------------------

// TestMockupOverrides pins the three places where the owner's answer departs
// from the mockup, so none of them can drift back to the picture:
//  1. the notice strip is included by the Nearby template only (D-29);
//  2. the mockup's footer line under Post report (the "Not confirmed yet"
//     sentence) is in no template and no script, on either size (O-1);
//  3. the chosen category tile in the Post a report window is the soft grey fill
//     with no dark border (O-2, D-04).
func TestMockupOverrides(t *testing.T) {
	t.Run("strip only on Nearby", func(t *testing.T) {
		var including []string
		layoutWalk(t, TemplatesFS, "templates", []string{".tmpl"}, func(path, content string) {
			if strings.Contains(content, "notice_strip.html.tmpl") && path != "templates/notice_strip.html.tmpl" {
				including = append(including, path)
			}
		})
		if len(including) != 1 || including[0] != "templates/index.html.tmpl" {
			t.Errorf("templates including the strip = %v, want only templates/index.html.tmpl (D-29)", including)
		}
	})

	t.Run("no Not confirmed yet footer line", func(t *testing.T) {
		// The mockup's sentence ends "until people nearby confirm it"; the
		// tail is enough to find it in any wording around it.
		const tail = "until people nearby confirm it"
		check := func(path, content string) {
			if strings.Contains(content, tail) {
				t.Errorf("%s holds the mockup's footer line about Not confirmed yet; it is built on neither size (O-1)", path)
			}
		}
		layoutWalk(t, TemplatesFS, "templates", []string{".tmpl"}, check)
		layoutWalk(t, StaticFS, "static", []string{".js", ".css"}, check)
	})

	t.Run("chosen category tile is the soft grey fill", func(t *testing.T) {
		const head = ".post-window #category-grid .category-tile--selected"
		rule := p31mRule(t, "static/css/modal.css", "", head)
		if got := rule.Decls["background"]; got != "var(--color-surface)" {
			t.Errorf("%s background is %q, want var(--color-surface) (O-2)", head, got)
		}
		for prop, value := range rule.Decls {
			if strings.HasPrefix(prop, "border") && prop != "border-radius" {
				if strings.Contains(value, "var(--color-text)") || heavyBorder.MatchString(value) {
					t.Errorf("%s declares %s: %s; no dark and no 2px border on the chosen tile (O-2)", head, prop, value)
				}
			}
			if strings.HasPrefix(prop, "outline") || prop == "box-shadow" {
				t.Errorf("%s declares %s; the chosen tile is a fill only (O-2)", head, prop)
			}
		}
	})
}
