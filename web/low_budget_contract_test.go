package web_test

// Byte budget for the Low data view (plan 04-27, D-14, P4-A14). The test renders
// the real /low page and sums the raw bytes of the page plus every local
// stylesheet, script and icon file it links, then holds the sum under a budget
// that was measured once, after plans 04-24 and 04-25 made their last Low data
// edits. The test lives in package web_test, beside the package web files,
// because internal/api/handlers imports pinalert/web and a package web file
// could not import it back. Every helper starts with p427.

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/api/handlers"
	"pinalert/web"
)

// The measured figure and the budget, set together in one commit.
//
// Measured on 2026-10-16 after plans 04-24 and 04-25 made their last Low data
// edits: 509094 raw bytes (html 10338, css 178817, js 308607, icons 11332,
// other 0; gzip level 5 total 153048 bytes). The budget is 585728 bytes: the
// measured total plus 15 percent, rounded up to the next 1024 bytes (one KB here
// means 1024 bytes), computed by p427BudgetFor. Raw bytes are tested although the UI-SPEC
// described a compressed budget (orchestrator decision 2): the router now
// compresses text, but the transfer size depends on the client, so raw bytes are
// the conservative bound. No icon file was dropped to meet the budget (P4-A14):
// the icons are counted inside the budget, which is set from the measurement,
// and dropping one would need a low.css rule that plan 04-27 does not own.
//
// To change the budget after an intended change: re-measure with
// go test ./web/ -run TestLowDataByteBudget -v, set both constants together in
// one commit and update the Low data view size budget section of
// docs/MODERATION.md.
const (
	lowDataMeasuredBytes = 509094
	lowDataByteBudget    = 585728
)

type p427Rule struct {
	head string
	body string
}

var (
	p427LinkAttr  = regexp.MustCompile(`(?:src|href)="(/static/[^"]*)"`)
	p427URLValue  = regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)`)
	p427ImportAt  = regexp.MustCompile(`@import\b`)
	p427BlockCmnt = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// p427RenderLowPage serves GET /low through the real handler with fixed inputs
// so the figure is reproducible, and returns the body.
func p427RenderLowPage(t *testing.T) []byte {
	t.Helper()
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}
	cfg := handlers.PageConfig{FallbackLat: 12.9716, FallbackLon: 77.5946, DefaultRadiusKm: 10, VoteRadiusKm: 1, AssetVersion: "v427"}
	req := httptest.NewRequest(http.MethodGet, handlers.LowDataPath, nil)
	req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 42, Email: "reader@example.com"}))
	rec := httptest.NewRecorder()
	handlers.LowPage(tmpl, cfg)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", handlers.LowDataPath, rec.Code)
	}
	return rec.Body.Bytes()
}

// p427LinkedStaticPaths returns every src or href value that starts with
// /static/, with its query string and fragment removed, deduplicated, in first
// seen order.
func p427LinkedStaticPaths(html string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range p427LinkAttr.FindAllStringSubmatch(html, -1) {
		p := m[1]
		if i := strings.IndexAny(p, "?#"); i >= 0 {
			p = p[:i]
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// p427CSSRules takes every innermost brace pair as one rule, which also reaches
// rules nested in media blocks. Block comments are removed first.
func p427CSSRules(css string) []p427Rule {
	css = p427BlockCmnt.ReplaceAllString(css, "")
	var rules []p427Rule
	start := 0
	lastOpen := -1
	for i := 0; i < len(css); i++ {
		switch css[i] {
		case '{':
			// An opening brace right after another opening brace (a media
			// block) moves the head start up to the new rule.
			if lastOpen >= 0 {
				start = lastOpen + 1
			}
			lastOpen = i
		case '}':
			if lastOpen >= 0 {
				rules = append(rules, p427Rule{head: strings.TrimSpace(css[start:lastOpen]), body: css[lastOpen+1 : i]})
				lastOpen = -1
			}
			start = i + 1
		}
	}
	return rules
}

// p427CSSURLTargets returns the url targets of every rule whose head does not
// contain .globe-bg__mask (the one named inert exception of the asset policy:
// that element never renders on this page). Quotes are stripped and data: values
// are skipped because their bytes are already inside the stylesheet.
func p427CSSURLTargets(css string) []string {
	var out []string
	for _, r := range p427CSSRules(css) {
		if strings.Contains(r.head, ".globe-bg__mask") {
			continue
		}
		for _, m := range p427URLValue.FindAllStringSubmatch(r.body, -1) {
			target := strings.TrimSpace(m[1])
			if strings.HasPrefix(target, "data:") {
				continue
			}
			out = append(out, target)
		}
	}
	return out
}

// p427BudgetFor is the measured total plus 15 percent, rounded up to the next
// 1024 bytes.
func p427BudgetFor(measured int) int {
	withMargin := (measured*115 + 99) / 100
	return (withMargin + 1023) / 1024 * 1024
}

// p427GzipSize is the length after gzip level 5, the level the router uses.
// Informational only, never asserted.
func p427GzipSize(b []byte) int {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, 5)
	if err != nil {
		return -1
	}
	_, _ = zw.Write(b)
	_ = zw.Close()
	return buf.Len()
}

func TestP427HelpersSelfCheck(t *testing.T) {
	if got := p427BudgetFor(1); got != 1024 {
		t.Errorf("p427BudgetFor(1) = %d, want 1024", got)
	}
	if got := p427BudgetFor(1000); got != 2048 {
		t.Errorf("p427BudgetFor(1000) = %d, want 2048", got)
	}
	if got := p427BudgetFor(100000); got != 115712 {
		t.Errorf("p427BudgetFor(100000) = %d, want 115712", got)
	}
	css := `.a { background: url("/static/icons/x.svg"); }
@media (min-width: 600px) { .b { mask: url(/static/icons/y.svg); } }
.globe-bg__mask { mask-image: url("/static/img/globe.svg"); }
.c { background: url(data:image/svg+xml;base64,AAAA); }`
	got := p427CSSURLTargets(css)
	if strings.Join(got, ",") != "/static/icons/x.svg,/static/icons/y.svg" {
		t.Errorf("p427CSSURLTargets = %v", got)
	}
	html := `<link href="/static/css/a.css?v=1"><script src="/static/js/b.js"></script><link href="/static/css/a.css?v=2"><a href="/other">x</a>`
	if got := p427LinkedStaticPaths(html); strings.Join(got, ",") != "/static/css/a.css,/static/js/b.js" {
		t.Errorf("p427LinkedStaticPaths = %v", got)
	}
}

func TestLowDataByteBudget(t *testing.T) {
	html := p427RenderLowPage(t)

	type counted struct {
		path string
		size int
		kind string
	}
	files := []counted{{path: "(rendered " + handlers.LowDataPath + " page)", size: len(html), kind: "html"}}
	gzipTotal := p427GzipSize(html)
	have := map[string]bool{}
	iconSeen := map[string]bool{}

	read := func(urlPath string) []byte {
		t.Helper()
		raw, err := fs.ReadFile(web.StaticFS, strings.TrimPrefix(urlPath, "/"))
		if err != nil {
			t.Errorf("%s is linked or used by url() but is not in the embedded static files: %v", urlPath, err)
			return nil
		}
		return raw
	}

	for _, p := range p427LinkedStaticPaths(string(html)) {
		raw := read(p)
		if raw == nil {
			continue
		}
		have[p] = true
		kind := "other"
		switch {
		case strings.HasSuffix(p, ".css"):
			kind = "css"
		case strings.HasSuffix(p, ".js"):
			kind = "js"
		case strings.HasPrefix(p, "/static/icons/"):
			kind = "icon"
			iconSeen[p] = true
		}
		files = append(files, counted{path: p, size: len(raw), kind: kind})
		gzipTotal += p427GzipSize(raw)

		if kind != "css" {
			continue
		}
		css := string(raw)
		if p427ImportAt.MatchString(p427BlockCmnt.ReplaceAllString(css, "")) {
			t.Errorf("%s holds an @import, which would fetch a file this test cannot count", p)
		}
		for _, target := range p427CSSURLTargets(css) {
			if i := strings.IndexAny(target, "?#"); i >= 0 {
				target = target[:i]
			}
			if !strings.HasPrefix(target, "/static/") {
				t.Errorf("%s: url target %q is neither under /static/ nor a data: value", p, target)
				continue
			}
			if iconSeen[target] || have[target] {
				continue
			}
			iraw := read(target)
			if iraw == nil {
				continue
			}
			iconSeen[target] = true
			files = append(files, counted{path: target, size: len(iraw), kind: "icon"})
			gzipTotal += p427GzipSize(iraw)
		}
	}

	totals := map[string]int{}
	total := 0
	for _, f := range files {
		totals[f.kind] += f.size
		total += f.size
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].size > files[j].size })

	t.Logf("low data total: %d bytes (html %d, css %d, js %d, icons %d, other %d); gzip level 5: %d bytes",
		total, totals["html"], totals["css"], totals["js"], totals["icon"], totals["other"], gzipTotal)
	for _, f := range files {
		t.Logf("  %7d  %s", f.size, f.path)
	}

	// Required members. Provenance: the 7 stylesheets are the UI-SPEC "Size
	// budget as a UI constraint" row "Same origin, deferred"; the 10 scripts are
	// the outline "Script order" for the Low data view; the 5 icons are the
	// P4-A14 shell and alert icons.
	required := []string{
		"/static/css/main.css", "/static/css/feed.css", "/static/css/trust.css", "/static/css/auth.css",
		"/static/css/chrome.css", "/static/css/low.css", "/static/css/modal.css",
		"/static/js/theme.js", "/static/js/app.js", "/static/js/votes.js", "/static/js/visibility.js",
		"/static/js/trust.js", "/static/js/queue.js", "/static/js/flag.js", "/static/js/report-form.js",
		"/static/js/low.js", "/static/js/account-menu.js",
		"/static/icons/user.svg", "/static/icons/sun.svg", "/static/icons/moon.svg",
		"/static/icons/log-out.svg", "/static/icons/circle-alert.svg",
	}
	counted2 := map[string]bool{}
	for _, f := range files {
		counted2[f.path] = true
	}
	for _, r := range required {
		if !counted2[r] {
			t.Errorf("required file %s is not among the counted files", r)
		}
	}

	// Provenance of the floor: RESEARCH "Size budget (D-14)" measured ten of
	// these shipped files at 142,084 raw bytes before Phases 3 and 4 added to
	// them; the floor leaves room for 04-10 moving the map rules out of
	// modal.css, and proves the constant was set from a measurement.
	if lowDataMeasuredBytes < 100000 {
		t.Errorf("lowDataMeasuredBytes = %d, want at least 100000 (set it from the logged total)", lowDataMeasuredBytes)
	}
	if want := p427BudgetFor(lowDataMeasuredBytes); lowDataByteBudget != want {
		t.Errorf("lowDataByteBudget = %d, want p427BudgetFor(%d) = %d", lowDataByteBudget, lowDataMeasuredBytes, want)
	}
	if total > lowDataByteBudget {
		var largest []string
		for i, f := range files {
			if i == 5 {
				break
			}
			largest = append(largest, f.path)
		}
		t.Errorf("the Low data view is %d raw bytes, over the budget of %d; the five largest files are %s. "+
			"If the growth is intended, re-measure, set lowDataMeasuredBytes and lowDataByteBudget together in one commit and update docs/MODERATION.md",
			total, lowDataByteBudget, strings.Join(largest, ", "))
	}
}
