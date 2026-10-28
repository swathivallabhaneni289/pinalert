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
	vm := map[string]any{
		"AssetVersion":          "test",
		"Email":                 "",
		"Sent":                  false,
		"ResendCooldownSeconds": 45,
		"LinkTTLSeconds":        300,
		"LowDataDefault":        lowData,
		"DoneLine":              "",
	}
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
	for _, want := range []string{"'anonymous'", "'no-referrer'", "requestIdleCallback", "'load'"} {
		if !strings.Contains(js, want) {
			t.Errorf("login-globe.js lacks %s", want)
		}
	}
	// The loader injects the libraries after the page has loaded and gone idle.
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
	for _, want := range []string{"low data view is on for this device. it opens after you sign in.", "turn off"} {
		if !strings.Contains(low, want) {
			t.Errorf("the Low data sign in page lacks %q", want)
		}
	}
	normal := p507Render(t, false)
	for _, want := range []string{"Low data view", "Text only, no map", "login-globe.css", "login-globe.js"} {
		if !strings.Contains(normal, want) {
			t.Errorf("the normal sign in page lacks %q", want)
		}
	}
}

// TestLoginGlobePartialOrder pins D-14 and UI-SPEC 5d: the globe is its own
// partial after .login-column inside #login-gate, and the order heading, field,
// button, Low data panel, row of links is kept.
func TestLoginGlobePartialOrder(t *testing.T) {
	src := p421ReadTemplate(t, "login_gate.html.tmpl")
	p421InOrder(t, "login_gate.html.tmpl order", src,
		`id="login-gate"`, `class="login-column"`, `id="login-form-panel"`, `id="login-email"`, `id="request-link"`,
		`href="/login?low=1"`, `{{template "login_ways_links" .}}`, `{{template "login_globe" .}}`,
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
	for _, id := range []string{`id="login-gate"`, `id="login-form-panel"`, `id="login-email"`, `id="request-link"`, `id="auth-error"`} {
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
		t.Errorf("rendered order is column %d, Low data panel %d, globe %d; the globe must come last in #login-gate", columnAt, waysAt, globeAt)
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
		`var\s+TURN_SECONDS\s*=\s*120\b`:     "TURN_SECONDS = 120",
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
	if strings.Contains(p421ReadTemplate(t, "verify_outcome.html.tmpl"), "globe") {
		t.Errorf("verify_outcome.html.tmpl still holds the flat globe markup")
	}
}

// TestLoginRowTryTheDemo pins D-03 and UI-SPEC 4e: the quiet row reads Try the
// demo, Disclaimer, Policy, in its own partial, with the reserved video slot.
func TestLoginRowTryTheDemo(t *testing.T) {
	partial := p421ReadTemplate(t, "login_ways_links.html.tmpl")
	p421InOrder(t, "login_ways_links.html.tmpl", partial,
		`{{define "login_ways_links"}}`, `class="login-ways__links"`,
		`id="login-try-demo" href="/demo">Try the demo</a>`,
		`<a href="/disclaimer">Disclaimer</a>`, `<a href="/policy">Policy</a>`)
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
		p421InOrder(t, "rendered row", page, `Try the demo</a>`, `Disclaimer</a>`, `Policy</a>`)
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
