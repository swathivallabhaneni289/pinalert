// Plan 05-07 pins the sign in globe and the quiet row of the sign in screen
// (D-03, D-13, D-14, UI-SPEC Surface 4e and Surface 5, sketch 009). The tests
// read the shipped templates, stylesheets and scripts out of the embedded
// filesystems and render the real login page, never a copy of their text.
//
// The tests that need a running handler (the page header) live in
// internal/api/handlers, because that package imports this one.

package web

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"
	"testing"
)

// p507Render draws login_gate.html.tmpl the way LoginGate does, with the Low
// data device default on or off.
func p507Render(t *testing.T, lowData bool) string {
	t.Helper()
	tmpl, err := template.ParseFS(TemplatesFS, "templates/*.tmpl")
	if err != nil {
		t.Fatalf("parsing the templates: %v", err)
	}
	// Amended 2026-10-29 by Phase 7.1 plan 11: the view model of the new card
	// (loginGateViewModel) replaces Email, Sent, ResendCooldownSeconds and
	// LinkTTLSeconds. p711Vm holds the defaults of a fresh browser.
	vm := p711Vm()
	vm["LowDataDefault"] = lowData
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "login_gate.html.tmpl", vm); err != nil {
		t.Fatalf("rendering login_gate.html.tmpl (lowData=%v): %v", lowData, err)
	}
	return out.String()
}

var p507IntegrityRE = regexp.MustCompile(`integrity="([^"]+)"`)

// p507IndexPins reads the maplibre-gl 5.24.0 script and stylesheet addresses
// and their SRI hashes out of index.html.tmpl, the one place the map page
// keeps them, so the sign in page is held to the same files.
func p507IndexPins(t *testing.T) (jsURL, jsSRI, cssURL, cssSRI string) {
	t.Helper()
	index := p421ReadTemplate(t, "index.html.tmpl")
	jsURL = "https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js"
	cssURL = maplibreStylesheetHref
	jsSRI = p507IntegrityRE.FindStringSubmatch(findTagWindow(t, index, jsURL))[1]
	cssSRI = p507IntegrityRE.FindStringSubmatch(findTagWindow(t, index, cssURL))[1]
	if jsSRI == "" || cssSRI == "" {
		t.Fatalf("index.html.tmpl has no SRI on the maplibre-gl tags")
	}
	return
}

// TestLoginGlobeTags pins T-05-34 and T-05-36: the sign in page carries the
// same pinned maplibre-gl files as the map page, never as a tag in the head,
// and the loader runs after auth.js.
func TestLoginGlobeTags(t *testing.T) {
	jsURL, jsSRI, cssURL, cssSRI := p507IndexPins(t)
	page := p507Render(t, false)

	for _, want := range []string{jsURL, jsSRI, cssURL, cssSRI} {
		if n := strings.Count(page, want); n != 1 {
			t.Errorf("the sign in page holds %q %d times, want exactly 1", want, n)
		}
	}
	head := page[:strings.Index(page, "</head>")]
	for _, banned := range []string{"unpkg", "maplibre", "<script src=\"/static/js/login-globe.js"} {
		if strings.Contains(head, banned) {
			t.Errorf("the head of the sign in page holds %q; the libraries and the loader are never in the head", banned)
		}
	}
	// Neither maplibre address is written as a tag. They travel as data
	// attributes on the loader's own script, and the loader injects them.
	if strings.Contains(page, `src="`+jsURL) || strings.Contains(page, `href="`+cssURL) {
		t.Errorf("the page writes a maplibre-gl tag itself; the loader must inject it after load and idle")
	}

	authAt := strings.Index(page, `/static/js/auth.js`)
	globeAt := strings.Index(page, `/static/js/login-globe.js`)
	if authAt == -1 || globeAt == -1 {
		t.Fatalf("auth.js at %d, login-globe.js at %d; both must be on the page", authAt, globeAt)
	}
	if globeAt < authAt {
		t.Errorf("login-globe.js is written before auth.js; deferred scripts run in order and Send works only once auth.js has wired it")
	}
	tag := findTagWindow(t, page, `/static/js/login-globe.js`)
	if !strings.Contains(tag, " defer") {
		t.Errorf("the login-globe.js tag is not deferred: %s", tag)
	}
	if !strings.Contains(tag, `data-tiles="/globe/tiles/{z}/{y}/{x}.jpg"`) {
		t.Errorf("the login-globe.js tag does not carry data-tiles for our own tile route: %s", tag)
	}

	js := p421ReadStatic(t, "js/login-globe.js")
	for _, want := range []string{"'anonymous'", "'no-referrer'"} {
		if !strings.Contains(js, want) {
			t.Errorf("login-globe.js lacks %s", want)
		}
	}
	// Amended 2026-11-19 (owner on the 7.1 globe retest): the loader starts as
	// soon as the deferred script runs, never waiting for load or an idle moment.
	for _, banned := range []string{"requestIdleCallback", "addEventListener('load'"} {
		if strings.Contains(js, banned) {
			t.Errorf("login-globe.js still waits on %s", banned)
		}
	}
	p421InOrder(t, "login-globe.js start order", js, "function canDrawWebGL2", "function injectLibraries", "function startGlobe")
}

// TestLoginGlobeNotWhenLowData pins D-13 and UI-SPEC 5c: with the Low data view
// remembered, the page holds no globe markup, script, stylesheet or preconnect,
// and the panel words are exactly as they were.
func TestLoginGlobeNotWhenLowData(t *testing.T) {
	low := strings.ToLower(p507Render(t, true))
	for _, banned := range []string{"globe", "unpkg", "login-globe", "preconnect", "maplibre"} {
		if strings.Contains(low, banned) {
			t.Errorf("the Low data sign in page holds %q", banned)
		}
	}
	// Amended 2026-10-29 by Phase 7.1 plan 11 (UI-SPEC 1e, round 2 item 3 a): the
	// panel words are now the one status line above the row and the Turn off low
	// data view button; "Text only, no map" is gone from the normal page. The
	// no globe, no unpkg and no preconnect assertions above are unchanged.
	for _, want := range []string{"low data view is on. it opens after you sign in.", "turn off low data view"} {
		if !strings.Contains(low, want) {
			t.Errorf("the Low data sign in page lacks %q", want)
		}
	}
	normal := p507Render(t, false)
	for _, want := range []string{"Low data view", "login-globe.css", "login-globe.js"} {
		if !strings.Contains(normal, want) {
			t.Errorf("the normal sign in page lacks %q", want)
		}
	}
	if strings.Contains(normal, "Text only, no map") {
		t.Errorf("the normal sign in page still holds the hint line under Low data view")
	}
}

// TestLoginGlobePartialOrder pins D-14 and UI-SPEC 5d: the globe is its own
// partial after .login-column inside #login-gate, and the order heading, field,
// button, Low data panel, row of links is kept.
func TestLoginGlobePartialOrder(t *testing.T) {
	src := p421ReadTemplate(t, "login_gate.html.tmpl")
	// Amended 2026-10-29 by Phase 7.1 plan 11 (round 2 item 3 b): the old email
	// field, button and error ids became login-title, login-choice and
	// create-email, and the Low data link left for the row in the partial. The
	// order is now the heading, the choices, the fields, then the one row inside
	// .login-ways, with the globe include still after .login-column. Every globe
	// assertion below is kept.
	p421InOrder(t, "login_gate.html.tmpl order", src,
		`id="login-gate"`, `class="login-column"`, `id="login-form-panel"`, `id="login-title"`, `id="login-choice"`,
		`id="create-email"`, `class="login-ways"`, `{{template "login_ways_links" .}}`, `{{template "login_globe" .}}`,
		`/static/js/auth.js`, `{{template "login_globe_script" .}}`)
	for _, include := range []string{`{{template "login_globe" .}}`, `{{template "login_globe_script" .}}`, `{{template "login_globe_css" .}}`} {
		at := strings.Index(src, include)
		if at == -1 {
			t.Errorf("login_gate.html.tmpl lacks %s", include)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(src[strings.LastIndex(src[:at], "\n"):at]), "{{if not .LowDataDefault}}") {
			t.Errorf("%s is not written under {{if not .LowDataDefault}}", include)
		}
	}
	for _, id := range []string{`id="login-gate"`, `id="login-form-panel"`, `id="login-title"`, `id="login-choice"`, `id="create-email"`} {
		if n := strings.Count(src, id); n != 1 {
			t.Errorf("%s appears %d times in login_gate.html.tmpl, want 1", id, n)
		}
	}
	// The stylesheet is in the head, so the credit's space exists from the
	// first paint. The loader is never in the head.
	headEnd := strings.Index(src, "</head>")
	if at := strings.Index(src, `{{template "login_globe_css" .}}`); at == -1 || at > headEnd {
		t.Errorf("the login-globe.css include is not in the head")
	}
	if strings.Contains(src[:headEnd], "login_globe_script") || strings.Contains(src[:headEnd], `{{template "login_globe" .}}`) {
		t.Errorf("the globe markup or the loader is included in the head")
	}

	// Rendered: the globe markup comes after the whole login column.
	page := p507Render(t, false)
	columnAt := strings.Index(page, `class="login-column"`)
	waysAt := strings.Index(page, `class="login-ways"`)
	globeAt := strings.Index(page, `id="login-globe"`)
	if columnAt == -1 || waysAt == -1 || globeAt == -1 || !(columnAt < waysAt && waysAt < globeAt) {
		t.Errorf("rendered order is column %d, row wrapper %d, globe %d; the globe must come last in #login-gate", columnAt, waysAt, globeAt)
	}
	// Added 2026-10-29 by Phase 7.1 plan 11: the card closes before the wrapper of
	// the one row, the row sits inside the wrapper, and the globe is never inside
	// the card.
	cardAt := strings.Index(page, `id="login-form-panel"`)
	rowAt := strings.Index(page, `class="login-ways__row"`)
	if cardAt == -1 || rowAt == -1 || !(cardAt < waysAt && waysAt < rowAt && rowAt < globeAt) {
		t.Errorf("rendered order is card %d, wrapper %d, row %d, globe %d; want card, wrapper, row, globe", cardAt, waysAt, rowAt, globeAt)
	}
	if cardAt != -1 && waysAt > cardAt && strings.Contains(page[cardAt:waysAt], "login-globe") {
		t.Errorf("the globe markup sits inside the sign in card")
	}
	for _, want := range []string{`id="login-globe-map"`, `id="globe-credit"`, `aria-hidden="true"`, "NASA Blue Marble", "NASA GIBS", "Earth pictures:"} {
		if !strings.Contains(page, want) {
			t.Errorf("the rendered globe partial lacks %q", want)
		}
	}
	// The credit sits outside the faded, aria-hidden wrapper.
	partial := p421ReadTemplate(t, "login_globe.html.tmpl")
	wrapperStart := strings.Index(partial, `id="login-globe"`)
	creditStart := strings.Index(partial, `id="globe-credit"`)
	if wrapperStart == -1 || creditStart == -1 || wrapperStart > creditStart {
		t.Fatalf("login_globe.html.tmpl: wrapper at %d, credit at %d", wrapperStart, creditStart)
	}
	between := partial[wrapperStart:creditStart]
	// between starts inside the wrapper's own open tag, so the wrapper is closed
	// before the credit only when the closes outnumber the opens.
	if strings.Count(between, "</div>") <= strings.Count(between, "<div") {
		t.Errorf("the credit is inside the faded wrapper; it must be a sibling after it")
	}
	// The credit is written with the page, hidden by the stylesheet, never built by script.
	if !strings.Contains(partial, `href="https://visibleearth.nasa.gov/collection/1484/blue-marble"`) ||
		!strings.Contains(partial, `href="https://www.earthdata.nasa.gov/gibs"`) {
		t.Errorf("login_globe.html.tmpl lacks the two NASA links")
	}
}

// TestLoginGlobeStyleAndTiles pins the globe's source (D-13, sketch items 7, 8,
// 12): one map, the guarded probe, tiles from our own route, the zoom cap.
func TestLoginGlobeStyleAndTiles(t *testing.T) {
	raw := p421ReadStatic(t, "js/login-globe.js")
	code := stripJSComments(raw)

	for _, want := range []string{
		"webgl2", "/globe/tiles/", "projection: { type: 'globe' }", "interactive: false", "attributionControl: false",
		"contextType: 'webgl2'", "maxzoom: MAX_ZOOM", "webglcontextlost", "visibilitychange", "prefers-reduced-motion: reduce",
		"requestAnimationFrame", "createElement('link')", "createElement('script')",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("login-globe.js lacks %q", want)
		}
	}
	for pattern, name := range map[string]string{
		`var\s+TURN_SECONDS\s*=\s*120\b`:      "TURN_SECONDS = 120",
		`var\s+START_CENTRE\s*=\s*\[80, 18\]`: "START_CENTRE = [80, 18]",
		`var\s+GIVE_UP_MS\s*=\s*10000\b`:      "GIVE_UP_MS = 10000",
		`var\s+MAX_ZOOM\s*=\s*3\b`:            "MAX_ZOOM = 3",
	} {
		if !regexp.MustCompile(pattern).MatchString(code) {
			t.Errorf("login-globe.js lacks the constant %s", name)
		}
	}
	if n := strings.Count(code, "new maplibregl.Map("); n != 1 {
		t.Errorf("login-globe.js creates %d maps, want exactly 1", n)
	}
	// Banned anywhere in the file, comments included, so the acceptance greps hold.
	for _, banned := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "style.opacity", "100vh", "100dvh", "geolocation", "/api/", "fetch("} {
		if strings.Contains(raw, banned) {
			t.Errorf("login-globe.js holds %q", banned)
		}
	}
	// The probe runs before anything is injected.
	run := jsFunctionBody(t, code, "startGlobe()")
	p421InOrder(t, "startGlobe order", run, "canDrawWebGL2()", "injectLibraries(")

	// Only our own tile route is accepted; the template carries the address.
	partial := p421ReadTemplate(t, "login_globe.html.tmpl")
	if n := strings.Count(partial, "/globe/tiles/"); n != 1 {
		t.Errorf("login_globe.html.tmpl holds /globe/tiles/ %d times, want 1", n)
	}
	if strings.Contains(partial, "gibs.earthdata.nasa.gov") || strings.Contains(raw, "gibs.earthdata.nasa.gov/wmts") {
		t.Errorf("the sign in page names the NASA tile host; the pictures come from /globe/tiles/ only")
	}
}

// TestLoginGlobeNotInAuthCSSOrThemeJS pins sketch item 3: auth.css (which the
// Low data page links and counts) and theme.js (which every page loads) hold
// nothing of the globe.
func TestLoginGlobeNotInAuthCSSOrThemeJS(t *testing.T) {
	for _, file := range []string{"css/auth.css", "js/theme.js"} {
		text := strings.ToLower(p421ReadStatic(t, file))
		for _, banned := range []string{"globe", "maplibre", "@keyframes globe"} {
			if strings.Contains(text, banned) {
				t.Errorf("%s holds %q; the globe styles and script live in login-globe.css and login-globe.js", file, banned)
			}
		}
	}
	if _, err := StaticFS.ReadFile("static/img/globe.svg"); err == nil {
		t.Errorf("static/img/globe.svg still ships; the flat picture globe is gone (D-13)")
	}
	// Amended 2026-10-29 by Phase 7.1 plan 16: the check that the link landing page
	// holds no flat globe markup is removed, because that page is deleted (D-12).
}

// TestLoginRowTryTheDemo pins D-03 and UI-SPEC 4e: the quiet row reads Try the
// demo, Disclaimer, Policy, in its own partial, with the reserved video slot.
func TestLoginRowTryTheDemo(t *testing.T) {
	partial := p421ReadTemplate(t, "login_ways_links.html.tmpl")
	// Amended 2026-10-29 by Phase 7.1 plan 11 (UI-SPEC 1e): the row is now the one
	// row of four a.btn links (Low data view joins it as the second), so the
	// wrapper class is login-ways__row and each link carries class="btn". Try the
	// demo stays first, then Low data view, Disclaimer and Policy; the dot free
	// check and the reserved video slot below are unchanged.
	p421InOrder(t, "login_ways_links.html.tmpl", partial,
		`{{define "login_ways_links"}}`, `class="login-ways__row"`,
		`id="login-try-demo" href="/demo">Try the demo</a>`,
		`id="login-low"`, `href="/disclaimer">Disclaimer</a>`, `href="/policy">Policy</a>`)
	for _, banned := range []string{"·", "&middot;", "&#183;", "&bull;", "•"} {
		if strings.Contains(partial, banned) {
			t.Errorf("the row draws the separator %q between bordered buttons", banned)
		}
	}
	if !strings.Contains(partial, "explainer video") {
		t.Errorf("the partial does not mark the reserved explainer video slot")
	}
	for _, low := range []bool{false, true} {
		page := p507Render(t, low)
		if n := strings.Count(page, "Try the demo"); n != 1 {
			t.Errorf("lowData=%v: the sign in page holds Try the demo %d times, want 1", low, n)
		}
		p421InOrder(t, "rendered row", page, `Try the demo</a>`, `ow data view</a>`, `Disclaimer</a>`, `Policy</a>`)
	}
	if n := strings.Count(p421ReadTemplate(t, "login_gate.html.tmpl"), `{{template "login_ways_links" .}}`); n != 1 {
		t.Errorf("login_gate.html.tmpl includes the row %d times, want 1", n)
	}
}

// TestLoginRefererPolicy pins sketch item 15 and T-05-35 in the source: the
// sign in page sends Referrer-Policy same-origin, so MapLibre's tile requests
// never carry a page address (which can hold an email after Send) to another
// host. internal/api/handlers/login_referrer_test.go serves the page and checks
// the header itself.
func TestLoginRefererPolicy(t *testing.T) {
	src := goSourceText(t, "../internal/api/handlers/auth.go")
	start := strings.Index(src, "func LoginGate(")
	if start == -1 {
		t.Fatalf("auth.go has no LoginGate")
	}
	body := src[start:]
	if end := strings.Index(body, "\n}\n"); end != -1 {
		body = body[:end]
	}
	if !strings.Contains(body, `w.Header().Set("Referrer-Policy", "same-origin")`) {
		t.Errorf("LoginGate does not set Referrer-Policy: same-origin")
	}
	// It is set before the body is written.
	p421InOrder(t, "LoginGate header order", body, `"Referrer-Policy"`, "ExecuteTemplate(")
}

// ---------------------------------------------------------------------------
// Added 2026-10-29 by quick 261029-47j, owner feedback: the phone globe was cut
// off and the boxes hid the Earth. On a phone the whole Earth sits in a box the
// stylesheet reserves above the card, the Earth shows at full strength, and
// while the globe is on the sign in boxes are frosted glass with solid
// fallbacks. The laptop globe is untouched.
// ---------------------------------------------------------------------------

// p47jItem is one block of a stylesheet: the whitespace collapsed text before
// its opening brace, its body, and the offset of the prelude in the text.
type p47jItem struct {
	Prelude, Body string
	Start         int
}

// p47jItems splits comment stripped CSS into its top level items, with
// balanced braces. A nested block comes back as part of its parent's Body.
func p47jItems(s string) []p47jItem {
	var items []p47jItem
	pos := 0
	for {
		open := strings.Index(s[pos:], "{")
		if open == -1 {
			return items
		}
		open += pos
		depth := 1
		end := open + 1
		for end < len(s) && depth > 0 {
			switch s[end] {
			case '{':
				depth++
			case '}':
				depth--
			}
			end++
		}
		items = append(items, p47jItem{
			Prelude: strings.Join(strings.Fields(s[pos:open]), " "),
			Body:    s[open+1 : end-1],
			Start:   pos,
		})
		pos = end
	}
}

// p47jFindBlock returns the first block, nested ones included, whose prelude
// equals prelude exactly.
func p47jFindBlock(s, prelude string) (p47jItem, bool) {
	for _, it := range p47jItems(s) {
		if it.Prelude == prelude {
			return it, true
		}
		if strings.HasPrefix(it.Prelude, "@") {
			if inner, ok := p47jFindBlock(it.Body, prelude); ok {
				return inner, true
			}
		}
	}
	return p47jItem{}, false
}

// p47jRuleRaw returns the whitespace collapsed declaration text of the rule
// whose head is exactly head, among the direct items of body.
func p47jRuleRaw(t *testing.T, label, body, head string) string {
	t.Helper()
	for _, it := range p47jItems(body) {
		if it.Prelude == head {
			return strings.Join(strings.Fields(it.Body), " ")
		}
	}
	t.Errorf("%s: no rule with head %q", label, head)
	return ""
}

// p47jLeaves returns every rule without a nested block, at any depth.
func p47jLeaves(s string) []p47jItem {
	var out []p47jItem
	for _, it := range p47jItems(s) {
		if strings.Contains(it.Body, "{") {
			out = append(out, p47jLeaves(it.Body)...)
		} else {
			out = append(out, it)
		}
	}
	return out
}

const (
	p47jPhonePrelude = "@media (max-width: 899.98px)"
	p47jCombined     = "@media (max-width: 899.98px), (max-width: 1199.98px) and (max-height: 799.98px)"
	p47jPhoneQuery   = "(max-width: 899.98px)"
	p47jGateHead     = ".login-gate.has-globe"
	p47jClampVH      = "clamp(140px, min(100vw - 48px, (100vh - 404px) * 1.28), 300px)"
	p47jClampSVH     = "clamp(140px, min(100vw - 48px, (100svh - 404px) * 1.28), 300px)"
	p47jColumnHead   = ".login-gate.has-globe .login-column"
	// Amended 2026-10-29 by Phase 7.1 plan 11: the solid error backing moved from
	// the old .auth-error line to the shown .inline-error lines of the new card.
	p47jErrorHead = ".login-gate.has-globe .login-card .inline-error:not([hidden])"
)

// p47jBoxHeads is the one glass box; p47jButtonHeads are the four buttons of the
// row. Each is its own head, never a comma group.
//
// Amended 2026-10-29 by Phase 7.1 plan 11 (UI-SPEC 1e, rule 11): the row has no
// box, so the glass boxes are the card alone (#check-inbox is gone with the
// inbox partial, and .login-ways has no background), and the four buttons are
// one head, .login-ways__row .btn, replacing .login-ways__low, __off and
// __links a. Each button now carries its own blur.
var p47jBoxHeads = []string{
	".login-gate.has-globe .login-card",
}

var p47jButtonHeads = []string{
	".login-gate.has-globe .login-ways__row .btn:not(:hover)",
}

// TestLoginGlobePhoneBoxReserved pins the phone geometry: the whole Earth in a
// box the stylesheet reserves from the first paint, the card overlapping its
// lower fifth, and the script sizing the sphere from that box.
func TestLoginGlobePhoneBoxReserved(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/login-globe.css"))
	phone, ok := p47jFindBlock(css, p47jPhonePrelude)
	if !ok {
		t.Fatalf("login-globe.css has no block with the prelude %q", p47jPhonePrelude)
	}
	combined, ok := p47jFindBlock(css, p47jCombined)
	if !ok {
		t.Fatalf("login-globe.css lost the combined media block")
	}
	if phone.Start < combined.Start {
		t.Errorf("the phone block comes before the combined block; it must come after so it wins")
	}

	gate := p47jRuleRaw(t, "phone block", phone.Body, p47jGateHead)
	for _, want := range []string{"--g: " + p47jClampVH + ";", "justify-content: flex-start;"} {
		if !strings.Contains(gate, want) {
			t.Errorf("phone block %s lacks %q (has %q)", p47jGateHead, want, gate)
		}
	}
	supports, ok := p47jFindBlock(phone.Body, "@supports (height: 100svh)")
	if !ok {
		t.Fatalf("the phone block has no nested @supports (height: 100svh)")
	}
	svhGate := p47jRuleRaw(t, "phone svh", supports.Body, p47jGateHead)
	if !strings.Contains(svhGate, "--g: "+p47jClampSVH+";") {
		t.Errorf("the svh rule lacks the clamp with 100svh: %q", svhGate)
	}
	// A second --g line in the same rule would always win and collapse the box
	// where svh is unknown, so the vh rule holds exactly one.
	if n := strings.Count(gate, "--g:"); n != 1 {
		t.Errorf("the vh rule declares --g %d times, want 1", n)
	}

	wants := map[string][]string{
		".login-gate.has-globe .globe-area":    {"display: contents;"},
		".login-gate.has-globe .globe-bg":      {"position: relative;", "inset: auto;", "order: -1;", "flex: none;", "align-self: center;", "width: var(--g);", "height: var(--g);"},
		".login-gate.has-globe .globe-bg__map": {"left: 0;", "top: 0;", "width: 100%;", "height: 100%;"},
		p47jColumnHead:                         {"margin-top: calc(var(--g) * -0.22);"},
		".login-gate.has-globe .globe-credit":  {"order: 1;", "align-self: center;"},
	}
	for head, decls := range wants {
		raw := p47jRuleRaw(t, "phone block", phone.Body, head)
		for _, want := range decls {
			if !strings.Contains(raw, want) {
				t.Errorf("phone block %s lacks %q (has %q)", head, want, raw)
			}
		}
	}

	code := stripJSComments(p421ReadStatic(t, "js/login-globe.js"))
	if !regexp.MustCompile(`var\s+BOX\s*=\s*1\.06\b`).MatchString(code) {
		t.Errorf("login-globe.js lacks var BOX = 1.06")
	}
	m := regexp.MustCompile(`var\s+PHONE_QUERY\s*=\s*'([^']+)'`).FindStringSubmatch(code)
	if m == nil {
		t.Fatalf("login-globe.js lacks a PHONE_QUERY constant")
	}
	if m[1] != p47jPhoneQuery {
		t.Errorf("PHONE_QUERY is %q, want %q", m[1], p47jPhoneQuery)
	}
	if "@media "+m[1] != p47jPhonePrelude {
		t.Errorf("the script's query %q and the stylesheet's prelude %q disagree", m[1], p47jPhonePrelude)
	}
	size := jsFunctionBody(t, code, "sizeGlobe()")
	for _, want := range []string{"isPhone()", "wrapper.clientWidth", "/ BOX"} {
		if !strings.Contains(size, want) {
			t.Errorf("sizeGlobe() lacks %q", want)
		}
	}
	if !strings.Contains(jsFunctionBody(t, code, "isPhone()"), "matchMedia(PHONE_QUERY)") {
		t.Errorf("isPhone() does not use matchMedia(PHONE_QUERY)")
	}
}

// TestLoginGlassFallbacks pins the glass rules and the three ways back to a
// solid box: no backdrop-filter, no color-mix, and reduced transparency.
func TestLoginGlassFallbacks(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/login-globe.css"))
	top := p47jItems(css)
	topRule := func(head string) (p47jItem, bool) {
		for _, it := range top {
			if it.Prelude == head {
				return it, true
			}
		}
		return p47jItem{}, false
	}
	collapse := func(s string) string { return strings.Join(strings.Fields(s), " ") }

	glassStart := -1
	for _, head := range p47jBoxHeads {
		it, ok := topRule(head)
		if !ok {
			t.Errorf("login-globe.css has no top level rule %s", head)
			continue
		}
		if it.Start > glassStart {
			glassStart = it.Start
		}
		p421InOrder(t, head, collapse(it.Body),
			"background: var(--color-surface);",
			"background: color-mix(in srgb, var(--color-surface) 65%, transparent);",
			"-webkit-backdrop-filter: blur(4px);",
			"backdrop-filter: blur(4px);")
	}
	for _, head := range p47jButtonHeads {
		it, ok := topRule(head)
		if !ok {
			t.Errorf("login-globe.css has no top level rule %s", head)
			continue
		}
		if it.Start > glassStart {
			glassStart = it.Start
		}
		// Amended 2026-10-29 by Phase 7.1 plan 11: the buttons no longer sit inside
		// a blurred panel, so each carries its own blur of 4px (this used to assert
		// that they had none).
		p421InOrder(t, head, collapse(it.Body),
			"background: var(--color-bg);",
			"background: color-mix(in srgb, var(--color-bg) 65%, transparent);",
			"-webkit-backdrop-filter: blur(4px);",
			"backdrop-filter: blur(4px);")
	}
	if it, ok := topRule(p47jColumnHead); !ok {
		t.Errorf("login-globe.css has no top level rule %s", p47jColumnHead)
	} else if !strings.Contains(collapse(it.Body), "background: transparent;") {
		t.Errorf("%s does not declare background: transparent", p47jColumnHead)
	}
	if it, ok := topRule(p47jErrorHead); !ok {
		t.Errorf("login-globe.css has no top level rule %s", p47jErrorHead)
	} else {
		body := collapse(it.Body)
		if !strings.Contains(body, "background: var(--color-surface);") {
			t.Errorf("%s lacks the solid surface backing", p47jErrorHead)
		}
		if strings.Contains(body, "color-mix") || strings.Contains(body, "backdrop-filter") {
			t.Errorf("%s must stay solid: %q", p47jErrorHead, body)
		}
	}

	type fallback struct {
		prelude string
		noBlur  bool
	}
	for _, fb := range []fallback{
		{"@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px)))", false},
		{"@supports not (background: color-mix(in srgb, currentColor 65%, transparent))", true},
		{"@media (prefers-reduced-transparency: reduce)", true},
	} {
		block, ok := topRule(fb.prelude)
		if !ok {
			t.Errorf("login-globe.css lacks the fallback block %q", fb.prelude)
			continue
		}
		if block.Start < glassStart {
			t.Errorf("%q comes before the glass rules and would lose to them", fb.prelude)
		}
		for _, head := range p47jBoxHeads {
			raw := p47jRuleRaw(t, fb.prelude, block.Body, head)
			if !strings.Contains(raw, "background: var(--color-surface);") {
				t.Errorf("%q: %s does not restore the solid surface", fb.prelude, head)
			}
			if fb.noBlur {
				for _, want := range []string{"-webkit-backdrop-filter: none;", "backdrop-filter: none;"} {
					if !strings.Contains(raw, want) {
						t.Errorf("%q: %s lacks %q", fb.prelude, head, want)
					}
				}
			}
		}
		for _, head := range append([]string{p47jColumnHead}, p47jButtonHeads...) {
			raw := p47jRuleRaw(t, fb.prelude, block.Body, head)
			if !strings.Contains(raw, "background: var(--color-bg);") {
				t.Errorf("%q: %s does not restore the solid page colour", fb.prelude, head)
			}
			// Added 2026-10-29 by Phase 7.1 plan 11: a button with a blur of its own
			// drops the blur where the glass is switched off.
			if fb.noBlur && head != p47jColumnHead {
				for _, want := range []string{"-webkit-backdrop-filter: none;", "backdrop-filter: none;"} {
					if !strings.Contains(raw, want) {
						t.Errorf("%q: %s lacks %q", fb.prelude, head, want)
					}
				}
			}
		}
	}
	// The color-mix test value names no var(): a var() would make any browser
	// pass an @supports test, which is the point of the guard.
	if strings.Contains(css, "@supports not (background: color-mix(in srgb, var(") {
		t.Errorf("the color-mix guard tests a value with var(), which every browser passes")
	}
}

// TestLoginGlobeFullStrength pins the owner's change: the Earth shows at full
// strength in every theme, and the wrapper keeps the one allowed opacity.
func TestLoginGlobeFullStrength(t *testing.T) {
	rules := p16ParseCSS("static/css/login-globe.css", p16ReadStatic(t, "static/css/login-globe.css"))
	n := 0
	for _, r := range rules {
		if v, ok := r.Decls["--globe-dim"]; ok {
			n++
			if v != "1" {
				t.Errorf("%s (%s) sets --globe-dim to %q, want 1", r.Head, r.Media, v)
			}
		}
	}
	if n < 4 {
		t.Errorf("--globe-dim is declared %d times, want the four theme blocks", n)
	}
	for _, c := range []struct{ media, head string }{
		{"", ":root"},
		{"prefers-color-scheme: dark", ":root"},
		{"", `:root[data-theme="dark"]`},
		{"", `:root[data-theme="light"]`},
	} {
		r := p16Rule(t, rules, c.media, c.head)
		if r.Decls["--globe-dim"] != "1" {
			t.Errorf("%s (%q) does not declare --globe-dim: 1", c.head, c.media)
		}
	}
	if got := p16Rule(t, rules, "", ".globe-bg").Decls["opacity"]; got != "var(--globe-dim)" {
		t.Errorf(".globe-bg opacity is %q, want var(--globe-dim)", got)
	}
}

var p47jBackdropRE = regexp.MustCompile(`backdrop-filter:\s*([^;]+)`)

// TestLoginGlassOnlyWithGlobe pins that the glass exists only while the globe
// is on: every color-mix or blur rule is scoped to .login-gate.has-globe, the
// Low data page's stylesheet holds none, and that page writes no has-globe.
func TestLoginGlassOnlyWithGlobe(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/login-globe.css"))
	glassRules := 0
	for _, leaf := range p47jLeaves(css) {
		glass := strings.Contains(leaf.Body, "color-mix(")
		for _, m := range p47jBackdropRE.FindAllStringSubmatch(leaf.Body, -1) {
			if strings.TrimSpace(m[1]) != "none" {
				glass = true
			}
		}
		if !glass {
			continue
		}
		glassRules++
		for _, part := range p16HeadParts(leaf.Prelude) {
			if !strings.HasPrefix(part, ".login-gate.has-globe ") {
				t.Errorf("glass rule %q is not scoped under .login-gate.has-globe", part)
			}
		}
	}
	if glassRules == 0 {
		t.Errorf("login-globe.css holds no glass rule at all")
	}
	auth := p421ReadStatic(t, "css/auth.css")
	for _, banned := range []string{"backdrop-filter", "color-mix("} {
		if strings.Contains(auth, banned) {
			t.Errorf("auth.css holds %q; the Low data page keeps a solid card", banned)
		}
	}
	low := p507Render(t, true)
	for _, banned := range []string{"has-globe", "login-globe.css"} {
		if strings.Contains(low, banned) {
			t.Errorf("the Low data sign in page holds %q", banned)
		}
	}
}

// TestLoginGlassTextContrast pins the owner's "5 to 1" claim for the text on
// the glass, and the error line's backing. The glass is the surface or page
// colour at 65 percent over the Earth, which is anything between black and
// white; the 35 percent of the backdrop is checked at both ends. The computed
// minimum is about 5.0 (the dark card over pure white), so the floor is AA 4.5,
// because integer rounding in the mix can land at 4.99.
func TestLoginGlassTextContrast(t *testing.T) {
	main := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))
	themes := map[string]map[string]string{
		"light": p16Rule(t, main, "", ":root").Decls,
		"dark":  p16Rule(t, main, "", `:root[data-theme="dark"]`).Decls,
	}
	for name, tok := range themes {
		for _, glass := range []string{"--color-surface", "--color-bg"} {
			for _, behind := range []string{"#000000", "#FFFFFF"} {
				ctx := name + " " + glass + " over " + behind
				mixed := mixHexSRGB(t, tok[glass], 65, behind, 35, ctx)
				ratio := wcagContrastRatio(t, tok["--color-text"], mixed, ctx)
				t.Logf("%s: %s gives %.2f to 1", ctx, mixed, ratio)
				if ratio < 4.5 {
					t.Errorf("%s: text contrast %.2f is under 4.5", ctx, ratio)
				}
			}
		}
		if got := wcagContrastRatio(t, tok["--color-severity-critical"], tok["--color-surface"], name+" error line"); got < 4.5 {
			t.Errorf("%s: the error line on the solid surface is %.2f, want at least 4.5", name, got)
		}
	}
	css := stripCSSComments(p421ReadStatic(t, "css/login-globe.css"))
	found := false
	for _, it := range p47jItems(css) {
		if it.Prelude == p47jErrorHead && strings.Contains(it.Body, "background: var(--color-surface)") {
			found = true
		}
	}
	if !found {
		t.Errorf("no %s rule gives the error line a solid surface backing", p47jErrorHead)
	}

	// Amended 2026-10-29 by the Phase 5 review fix (WR-T04): the red on surface
	// pair above is kept, but it is not what the page paints. #auth-error and
	// #link-expiry-notice are p elements inside .login-card and #check-inbox,
	// so ".login-card p, #check-inbox p" (0,1,1 and 1,0,1) beats ".auth-error"
	// (0,1,0) and draws the line in the normal text colour, at 16px; colour is
	// reserved for severity, so that stays. This pins the winning rule and the
	// painted pair, and that the losing rule no longer claims a colour or size.
	//
	// Amended 2026-10-29 by Phase 7.1 plan 11: the old #auth-error and
	// #link-expiry-notice lines are gone from the page (the inbox partial is not
	// included any more). The error lines are p.inline-error elements inside the
	// card now. They are painted in --color-text through the shared .inline-error
	// rule of chrome.css, and the card scoped rule in auth.css wins over
	// ".login-card p" for size (Label 14px) and sets no colour of its own, so the
	// painted pair is the same 15.20 and 16.15 to 1 pinned below.
	gate := p421ReadTemplate(t, "login_gate.html.tmpl")
	card := strings.Index(gate, `id="login-form-panel"`)
	ways := strings.Index(gate, `class="login-ways"`)
	errAt := strings.Index(gate, `<p class="inline-error" id="create-error"`)
	if card == -1 || ways == -1 || errAt == -1 || errAt < card || errAt > ways {
		t.Errorf("create-error is no longer a p.inline-error inside .login-card; the painted colour pinned here would change")
	}
	authCSS := "static/css/auth.css"
	winner := p31mRule(t, authCSS, "", ".login-card p")
	if winner.Decls["color"] != "var(--color-text)" {
		t.Errorf(".login-card p colour = %q, want var(--color-text), the colour the error line falls back to", winner.Decls["color"])
	}
	cardError := p31mRule(t, authCSS, "", ".login-card .inline-error:not([hidden])")
	if v, ok := cardError.Decls["color"]; ok && v != "var(--color-text)" {
		t.Errorf("the card scoped error rule sets color %q; it must paint --color-text or set none", v)
	}
	if p31mRule(t, "static/css/chrome.css", "", ".inline-error:not([hidden])").Decls["color"] != "var(--color-text)" {
		t.Errorf("the shared .inline-error rule no longer paints the normal text colour")
	}
	// Amended 2026-10-29 by the Phase 7.1 review fix (templates): IN-T03 removed
	// the dead .auth-error rules with the rest of the old emailed link sign in, so
	// instead of reading .auth-error's declarations this pins that no rule for it
	// is left to claim a colour or a size.
	for _, r := range p16ParseCSS(authCSS, p16ReadStatic(t, authCSS)) {
		if strings.Contains(r.Head, ".auth-error") {
			t.Errorf("auth.css still has a .auth-error rule (%q); nothing draws that line any more", r.Head)
		}
	}
	for name, tok := range themes {
		got := wcagContrastRatio(t, tok["--color-text"], tok["--color-surface"], name+" painted error line")
		t.Logf("%s: the painted error line (text on the solid surface) is %.2f to 1", name, got)
		if got < 4.5 {
			t.Errorf("%s: the painted error line on the solid surface is %.2f, want at least 4.5", name, got)
		}
	}
	rawGlobe := p421ReadStatic(t, "css/login-globe.css")
	if strings.Contains(rawGlobe, "4.87 light and 5.46 dark") || strings.Contains(rawGlobe, "The red error line") {
		t.Errorf("login-globe.css still describes the error line as red at 4.87 and 5.46; it is painted in --color-text")
	}
}
