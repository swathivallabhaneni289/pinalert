package handlers

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"pinalert/web"
)

// chromeLookup builds a fake env lookup from a map. A key present in the
// map, even with an empty value, counts as set.
func chromeLookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadEmergencyNumbers(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		wantPrimary   string
		wantOlder     []string
		wantState     string
		wantDistrict  string
		wantHasState  bool
		wantOlderList string
	}{
		{
			name:        "all unset gives the defaults",
			env:         map[string]string{},
			wantPrimary: "112", wantOlder: []string{"100", "101", "108"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 101, 108",
		},
		{
			name:        "primary override",
			env:         map[string]string{"EMERGENCY_PRIMARY": "999"},
			wantPrimary: "999", wantOlder: []string{"100", "101", "108"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 101, 108",
		},
		{
			name:        "primary empty falls back",
			env:         map[string]string{"EMERGENCY_PRIMARY": ""},
			wantPrimary: "112", wantOlder: []string{"100", "101", "108"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 101, 108",
		},
		{
			name:        "primary blank falls back",
			env:         map[string]string{"EMERGENCY_PRIMARY": "   "},
			wantPrimary: "112", wantOlder: []string{"100", "101", "108"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 101, 108",
		},
		{
			name:        "primary invalid falls back",
			env:         map[string]string{"EMERGENCY_PRIMARY": "11a"},
			wantPrimary: "112", wantOlder: []string{"100", "101", "108"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 101, 108",
		},
		{
			name:        "older set but empty means none",
			env:         map[string]string{"EMERGENCY_OLDER": ""},
			wantPrimary: "112", wantOlder: nil,
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "",
		},
		{
			name:        "older trimmed",
			env:         map[string]string{"EMERGENCY_OLDER": " 100 , 101 "},
			wantPrimary: "112", wantOlder: []string{"100", "101"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 101",
		},
		{
			name:        "older drops empty and invalid items",
			env:         map[string]string{"EMERGENCY_OLDER": "100,,abc,108"},
			wantPrimary: "112", wantOlder: []string{"100", "108"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 108",
		},
		{
			name:        "state lines set but empty drops both",
			env:         map[string]string{"EMERGENCY_STATE_LINES": ""},
			wantPrimary: "112", wantOlder: []string{"100", "101", "108"},
			wantState: "", wantDistrict: "", wantHasState: false,
			wantOlderList: "100, 101, 108",
		},
		{
			name:        "state lines explicit pair",
			env:         map[string]string{"EMERGENCY_STATE_LINES": "1070,1077"},
			wantPrimary: "112", wantOlder: []string{"100", "101", "108"},
			wantState: "1070", wantDistrict: "1077", wantHasState: true,
			wantOlderList: "100, 101, 108",
		},
		{
			name:        "state lines half pair is not a pair",
			env:         map[string]string{"EMERGENCY_STATE_LINES": "1070"},
			wantPrimary: "112", wantOlder: []string{"100", "101", "108"},
			wantState: "1070", wantDistrict: "", wantHasState: false,
			wantOlderList: "100, 101, 108",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LoadEmergencyNumbers(chromeLookup(tc.env))
			if got.Primary != tc.wantPrimary {
				t.Errorf("Primary = %q, want %q", got.Primary, tc.wantPrimary)
			}
			if len(got.Older) != len(tc.wantOlder) || (len(tc.wantOlder) > 0 && !reflect.DeepEqual(got.Older, tc.wantOlder)) {
				t.Errorf("Older = %v, want %v", got.Older, tc.wantOlder)
			}
			if got.HasOlder() != (len(tc.wantOlder) > 0) {
				t.Errorf("HasOlder = %v, want %v", got.HasOlder(), len(tc.wantOlder) > 0)
			}
			if got.OlderList() != tc.wantOlderList {
				t.Errorf("OlderList = %q, want %q", got.OlderList(), tc.wantOlderList)
			}
			if got.StateHelpline != tc.wantState || got.DistrictHelpline != tc.wantDistrict {
				t.Errorf("state lines = %q,%q want %q,%q", got.StateHelpline, got.DistrictHelpline, tc.wantState, tc.wantDistrict)
			}
			if got.HasStateLines() != tc.wantHasState {
				t.Errorf("HasStateLines = %v, want %v", got.HasStateLines(), tc.wantHasState)
			}
		})
	}
}

func TestChromeWithDefaults(t *testing.T) {
	t.Run("zero chrome gets the default numbers", func(t *testing.T) {
		got := Chrome{}.withDefaults()
		if !reflect.DeepEqual(got.Numbers, DefaultEmergencyNumbers()) {
			t.Errorf("Numbers = %+v, want %+v", got.Numbers, DefaultEmergencyNumbers())
		}
		if got.Numbers.Primary != "112" {
			t.Errorf("Primary = %q, want 112", got.Numbers.Primary)
		}
	})

	t.Run("a set primary with no older list is kept as is", func(t *testing.T) {
		in := Chrome{Numbers: EmergencyNumbers{Primary: "999"}}
		got := in.withDefaults()
		if got.Numbers.Primary != "999" || got.Numbers.HasOlder() {
			t.Errorf("Numbers = %+v, want primary 999 and no older numbers", got.Numbers)
		}
	})

	t.Run("an older list the operator cleared stays empty", func(t *testing.T) {
		n := LoadEmergencyNumbers(chromeLookup(map[string]string{"EMERGENCY_OLDER": ""}))
		got := Chrome{Numbers: n}.withDefaults()
		if got.Numbers.HasOlder() {
			t.Errorf("Older = %v, want none", got.Numbers.Older)
		}
	})

	t.Run("other fields pass through untouched", func(t *testing.T) {
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
		// D-31): the bar on every page is replaced by the laptop strip on Nearby and
		// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
		// every page) is retired as worded; Disclaimer and Policy are reached from the
		// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
		// first-time flow, the login screen and the one reader. The Phase 3.1 and
		// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
		// The field that marked the Disclaimer link of the bar as current is gone.
		in := Chrome{ContactEmail: "a@example.com", LowDataPage: true}
		got := in.withDefaults()
		if got.ContactEmail != "a@example.com" || !got.LowDataPage {
			t.Errorf("got %+v, want fields untouched", got)
		}
	})

	t.Run("default older slice is fresh on every call", func(t *testing.T) {
		a := DefaultEmergencyNumbers()
		a.Older = append(a.Older[:0], "555")
		b := DefaultEmergencyNumbers()
		if !reflect.DeepEqual(b.Older, []string{"100", "101", "108"}) {
			t.Errorf("second call Older = %v, want the defaults", b.Older)
		}
	})

	t.Run("TelHref typed value", func(t *testing.T) {
		if got := DefaultEmergencyNumbers().TelHref(); got != template.URL("tel:112") {
			t.Errorf("TelHref = %q, want tel:112", got)
		}
		if got := (EmergencyNumbers{Primary: "999"}).TelHref(); got != template.URL("tel:999") {
			t.Errorf("TelHref = %q, want tel:999", got)
		}
	})

	t.Run("TelHref renders in an anchor without ZgotmplZ", func(t *testing.T) {
		tmpl := template.Must(template.New("tel").Parse(`<a href="{{.TelHref}}">x</a>`))
		cases := map[string]EmergencyNumbers{
			`<a href="tel:112">x</a>`: DefaultEmergencyNumbers(),
			`<a href="tel:999">x</a>`: {Primary: "999"},
		}
		for want, n := range cases {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, n); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if strings.Contains(buf.String(), "ZgotmplZ") || buf.String() != want {
				t.Errorf("rendered %q, want %q", buf.String(), want)
			}
		}
	})

	t.Run("OlderList on the defaults", func(t *testing.T) {
		if got := DefaultEmergencyNumbers().OlderList(); got != "100, 101, 108" {
			t.Errorf("OlderList = %q", got)
		}
	})
}

// chromeDefaultSentence is the exact D-29 strip text with the default number.
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
// The older numbers parenthetical of the bar is not repeated in the strip.
const chromeDefaultSentence = "Unofficial, not a government service. Pinalert cannot send help, it shows what people near you are seeing. Reports can sometimes be wrong. In danger, call 112. Read the disclaimer"

var (
	chromeBarPattern   = regexp.MustCompile(`(?s)<aside class="notice-strip".*?</aside>`)
	chromeTagPattern   = regexp.MustCompile(`(?s)<[^>]*>`)
	chromeSpacePattern = regexp.MustCompile(`\s+`)
)

// chromeBarRegion returns the one bar element of a rendered page, failing the
// test when the page holds none or more than one.
func chromeBarRegion(t *testing.T, body string) string {
	t.Helper()
	found := chromeBarPattern.FindAllString(body, -1)
	if len(found) != 1 {
		t.Fatalf("page holds %d bar elements, want exactly 1", len(found))
	}
	return found[0]
}

// chromeBarText returns the bar text with tags stripped and whitespace runs
// collapsed to one space, never deleting whitespace.
func chromeBarText(t *testing.T, body string) string {
	t.Helper()
	text := chromeTagPattern.ReplaceAllString(chromeBarRegion(t, body), "")
	return strings.TrimSpace(chromeSpacePattern.ReplaceAllString(text, " "))
}

func chromeParse(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}
	return tmpl
}

// chromeServeLogin renders the login page through the handler.
func chromeServeLogin(t *testing.T, tmpl *template.Template, cfg AuthConfig) string {
	t.Helper()
	rec := httptest.NewRecorder()
	LoginGate(tmpl, cfg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// chromeServeMap renders the map page through the handler.
func chromeServeMap(t *testing.T, tmpl *template.Template, cfg PageConfig) string {
	t.Helper()
	rec := httptest.NewRecorder()
	Page(tmpl, cfg).ServeHTTP(rec, newPageTestRequest())
	if rec.Code != http.StatusOK {
		t.Fatalf("map status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// chromeExecute runs one named template against a view model.
func chromeExecute(t *testing.T, tmpl *template.Template, name string, vm any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, vm); err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	return buf.String()
}

// chromeNoStrip fails the test when a rendered page holds the notice strip.
func chromeNoStrip(t *testing.T, name, body string) {
	t.Helper()
	if strings.Contains(body, "notice-strip") {
		t.Errorf("%s: the notice strip is for the Nearby page only", name)
	}
}

// TestDisclaimerBarSentenceIsExact is the strip's exact sentence on the page
// that carries it, and the login page without it.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
func TestDisclaimerBarSentenceIsExact(t *testing.T) {
	tmpl := chromeParse(t)
	chromeNoStrip(t, "login", chromeServeLogin(t, tmpl, AuthConfig{}))
	if got := chromeBarText(t, chromeServeMap(t, tmpl, PageConfig{})); got != chromeDefaultSentence {
		t.Errorf("map strip = %q, want %q", got, chromeDefaultSentence)
	}
}

// TestDisclaimerBarDropsEmptyNumbers keeps the property that the number is the
// configured one and that nothing else of the older list is printed.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
// The strip prints the primary number only; the older numbers parenthetical is
// left to the Disclaimer.
func TestDisclaimerBarDropsEmptyNumbers(t *testing.T) {
	tmpl := chromeParse(t)
	cases := []struct {
		name   string
		chrome Chrome
		want   string
	}{
		{
			"no older numbers",
			Chrome{Numbers: EmergencyNumbers{Primary: "999"}},
			"Unofficial, not a government service. Pinalert cannot send help, it shows what people near you are seeing. Reports can sometimes be wrong. In danger, call 999. Read the disclaimer",
		},
		{
			"one older number is not repeated",
			Chrome{Numbers: EmergencyNumbers{Primary: "112", Older: []string{"100"}}},
			chromeDefaultSentence,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := chromeBarText(t, chromeServeMap(t, tmpl, PageConfig{Chrome: tc.chrome})); got != tc.want {
				t.Errorf("map strip = %q, want %q", got, tc.want)
			}
			chromeNoStrip(t, "login", chromeServeLogin(t, tmpl, AuthConfig{Chrome: tc.chrome}))
		})
	}
}

// TestDisclaimerBarHasNoTelLink: a laptop cannot dial, so the strip holds the
// number as plain text and one plain link to the Disclaimer.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
func TestDisclaimerBarHasNoTelLink(t *testing.T) {
	tmpl := chromeParse(t)

	raw, err := fs.ReadFile(web.TemplatesFS, "templates/notice_strip.html.tmpl")
	if err != nil {
		t.Fatalf("reading the partial: %v", err)
	}
	if strings.Contains(string(raw), "tel:") || strings.Contains(string(raw), "TelHref") {
		t.Error("the strip partial must hold no telephone link")
	}

	region := chromeBarRegion(t, chromeServeMap(t, tmpl, PageConfig{}))
	if strings.Contains(region, "tel:") {
		t.Error("map: strip holds a tel link")
	}
	if got := strings.Count(region, "<a "); got != 1 {
		t.Errorf("map: strip holds %d links, want 1", got)
	}
	if got := strings.Count(region, "<button"); got != 0 {
		t.Errorf("map: strip holds %d buttons, want none", got)
	}
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): the link keeps its plain
	// href and also carries the reader opener, so a click opens the in app
	// reader and a visitor without script still reaches /disclaimer.
	if !strings.Contains(region, `<a class="notice-strip__link" href="/disclaimer" data-open-reader="disclaimer">Read the disclaimer</a>`) {
		t.Error("map: strip link is not a link to /disclaimer that opens the reader")
	}
	if strings.Contains(region, "aria-current") {
		t.Error("map: default render must not carry aria-current")
	}
	for _, line := range []string{"1070", "1077"} {
		if strings.Contains(region, line) {
			t.Errorf("map: strip shows the state line %s", line)
		}
	}
}

// TestExistingPagesRenderWithChrome renders every page that used to carry the
// bar and checks where the strip and the .top-chrome wrapper now are.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
func TestExistingPagesRenderWithChrome(t *testing.T) {
	tmpl := chromeParse(t)
	chrome := Chrome{}.withDefaults()

	// check wants the strip (or none) and the number of top-chrome wrappers.
	check := func(t *testing.T, body string, strip bool, wrappers int) {
		t.Helper()
		if strip {
			if got := chromeBarText(t, body); got != chromeDefaultSentence {
				t.Errorf("strip = %q, want %q", got, chromeDefaultSentence)
			}
		} else {
			chromeNoStrip(t, "page", body)
		}
		if got := strings.Count(body, `class="top-chrome"`); got != wrappers {
			t.Errorf("top-chrome appears %d times, want %d", got, wrappers)
		}
	}

	t.Run("map", func(t *testing.T) { check(t, chromeServeMap(t, tmpl, PageConfig{}), true, 1) })
	t.Run("login", func(t *testing.T) { check(t, chromeServeLogin(t, tmpl, AuthConfig{}), false, 0) })
	t.Run("verify", func(t *testing.T) {
		for _, outcome := range []string{"verified", "expired", "used", "malformed", "conflict"} {
			vm := verifyOutcomeViewModel{Outcome: outcome, Email: "a@example.com", CurrentEmail: "b@example.com", Chrome: chrome}
			check(t, chromeExecute(t, tmpl, verifyOutcomeTemplateName, vm), false, 0)
		}
	})
	t.Run("profile", func(t *testing.T) {
		check(t, chromeExecute(t, tmpl, profileTemplateName, profileViewModel{Email: "a@example.com", Chrome: chrome}), false, 1)
	})
	t.Run("zero_chrome", func(t *testing.T) {
		check(t, chromeServeMap(t, tmpl, PageConfig{}), true, 1)
		check(t, chromeServeLogin(t, tmpl, AuthConfig{}), false, 0)
	})
}

// chromeStubTemplates builds a template set that prints only the primary
// number, so the test needs no real template.
func chromeStubTemplates(t *testing.T) *template.Template {
	t.Helper()
	root := template.New("stub")
	for _, name := range []string{loginGateTemplateName, templateName} {
		template.Must(root.New(name).Parse(`{{.Chrome.Numbers.Primary}}`))
	}
	return root
}

func TestChromeReachesViewModels(t *testing.T) {
	stub := chromeStubTemplates(t)
	set := Chrome{Numbers: EmergencyNumbers{Primary: "999"}}

	loginReq := func() *http.Request { return httptest.NewRequest(http.MethodGet, "/login", nil) }

	cases := []struct {
		name string
		h    http.HandlerFunc
		req  *http.Request
		want string
	}{
		{"login with chrome", LoginGate(stub, AuthConfig{Chrome: set}), loginReq(), "999"},
		{"map with chrome", Page(stub, PageConfig{Chrome: set}), newPageTestRequest(), "999"},
		{"login zero config", LoginGate(stub, AuthConfig{}), loginReq(), "112"},
		{"map zero config", Page(stub, PageConfig{}), newPageTestRequest(), "112"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.h.ServeHTTP(rec, tc.req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if rec.Body.String() != tc.want {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.want)
			}
		})
	}
}
