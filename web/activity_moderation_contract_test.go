// activity_moderation_contract_test.go: plan 04-16's contract tests for the
// Activity rows of a held or removed report (P4-A9, P4-A10, R-01, D-02, D-08).
// They read the embedded template, scripts and stylesheet, so they need no
// database and no server.
package web

import (
	"io/fs"
	"strings"
	"testing"
)

// p416ReadStatic returns one embedded static file as text.
func p416ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return string(raw)
}

// p416ReadTemplate returns one embedded template as text.
func p416ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("failed to read embedded templates/%s: %v", name, err)
	}
	return string(raw)
}

// p416InOrder fails unless every needle occurs in text, each after the one
// before it.
func p416InOrder(t *testing.T, where, text string, needles ...string) {
	t.Helper()
	at := 0
	for _, needle := range needles {
		idx := strings.Index(text[at:], needle)
		if idx == -1 {
			t.Errorf("%s: %q is missing or out of order", where, needle)
			return
		}
		at += idx + len(needle)
	}
}

func TestDisplayStateModeration(t *testing.T) {
	raw := p416ReadStatic(t, "static/js/visibility.js")
	code := stripJSComments(raw)

	body := jsFunctionBody(t, code, "displayState(report)")
	p416InOrder(t, "displayState", body,
		"=== 'removed'", "=== 'held'", "report.expired !== true", "'retracted'", "report.expired === true")

	for _, want := range []string{
		"held: 'Waiting for a text check'",
		"removed: 'Removed after review'",
		"var VISIBILITY_STATES = ['live', 'provisional', 'hidden', 'retracted'];",
		"displayState: displayState",
		"stateWord: stateWord",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("visibility.js does not contain %s", want)
		}
	}
	if state := jsFunctionBody(t, code, "visibilityState(report)"); strings.Contains(state, "moderation") {
		t.Errorf("the visibilityState body mentions moderation, it must stay at the four resolver values")
	}
}

func TestActivityModerationRowsOmitTrust(t *testing.T) {
	tmpl := p416ReadTemplate(t, "profile.html.tmpl")
	liStart := strings.Index(tmpl, `<li class="report-row`)
	if liStart == -1 {
		t.Fatal("profile.html.tmpl has no report row li")
	}
	liEnd := strings.Index(tmpl[liStart:], ">")
	if liEnd == -1 {
		t.Fatal("the report row li has no closing angle bracket")
	}
	opening := tmpl[liStart : liStart+liEnd]
	for _, want := range []string{
		`{{if not .Moderation}}data-trust="{{.TrustJSON}}"{{end}}`,
		`{{if .Moderation}}data-moderation="{{.Moderation}}"{{end}}`,
	} {
		if !strings.Contains(opening, want) {
			t.Errorf("the report row opening tag does not contain %s", want)
		}
	}
	const note = `<p class="report-row__moderation-note">`
	if n := strings.Count(tmpl, note); n != 1 {
		t.Errorf("the template writes the moderation note paragraph %d times, want 1", n)
	}
	metaAt := strings.Index(tmpl, "report-row__meta")
	noteAt := strings.Index(tmpl, note)
	if metaAt == -1 || noteAt < metaAt {
		t.Errorf("the moderation note does not come after the first report-row__meta")
	}

	raw := p416ReadStatic(t, "static/js/activity.js")
	code := stripJSComments(raw)

	if n := strings.Count(code, "moderation: row.dataset.moderation"); n != 1 {
		t.Errorf("activity.js reads the row moderation %d times, want 1", n)
	}
	p416InOrder(t, "activity.js report object", code,
		"expired: row.dataset.expired === 'true'",
		"moderation: row.dataset.moderation",
		"PinalertVisibility.updateVisibilityTag(tag, report)")

	const applyCall = "PinalertVisibility.applyVisibilityClass(row, report)"
	from := strings.Index(code, applyCall)
	to := strings.Index(code, "PinalertTrust.createTrustBlock()")
	if from == -1 || to == -1 || to < from {
		t.Fatalf("activity.js does not call applyVisibilityClass and then createTrustBlock")
	}
	if !strings.Contains(code[from:to], "!report.moderation") {
		t.Errorf("the trust block mount is not guarded by !report.moderation")
	}
	for _, want := range []string{"insertBefore(tag", "report-row__moderation-note"} {
		if !strings.Contains(code, want) {
			t.Errorf("activity.js does not contain %s", want)
		}
	}
	if reopen := jsFunctionBody(t, code, "onReopenClick(row, block)"); strings.Contains(reopen, "moderation") {
		t.Errorf("onReopenClick mentions moderation, a moderated row never offers Reopen")
	}

	// The raw file, comments included, types no new state class and no label
	// sentence: those words belong to visibility.js and Go only.
	for _, banned := range []string{"vis-held", "vis-removed", "Waiting for a text check", "Removed after review"} {
		if strings.Contains(raw, banned) {
			t.Errorf("activity.js contains %q", banned)
		}
	}
	if strings.Contains(code, "vis-expired") {
		t.Errorf("activity.js names the expired class on a code line")
	}
}

func TestHeldRemovedRowTreatmentMatchesExpired(t *testing.T) {
	rules := parseCSSRules(p416ReadStatic(t, "static/css/trust.css"))

	expired, ok := ruleBySelector(rules, ".vis-expired")
	if !ok {
		t.Fatal("trust.css has no .vis-expired rule")
	}
	expiredDecls := declsOf(expired.declBody)

	var expiredBadge cssRule
	for _, r := range rules {
		if strings.Contains(r.selectorHead, "vis-expired") && strings.Contains(r.declBody, "background") {
			expiredBadge = r
		}
	}
	if expiredBadge.selectorHead == "" {
		t.Fatal("trust.css has no expired badge rule")
	}
	badgeDecls := declsOf(expiredBadge.declBody)

	sameDecls := func(label string, got, want map[string]string) {
		for name, value := range want {
			if g, has := got[name]; !has || g != value {
				t.Errorf("%s %s is %q, the expired rule has %q", label, name, g, value)
			}
		}
		for name := range got {
			if _, has := want[name]; !has {
				t.Errorf("%s declares %s, which the expired rule does not", label, name)
			}
		}
	}

	for _, state := range []string{"vis-held", "vis-removed"} {
		r, found := ruleBySelector(rules, "."+state)
		if !found {
			t.Errorf("trust.css has no .%s rule", state)
			continue
		}
		sameDecls("."+state, declsOf(r.declBody), expiredDecls)

		badgeRules := 0
		for _, br := range rules {
			if !strings.Contains(br.selectorHead, state) || !strings.Contains(br.declBody, "background") {
				continue
			}
			badgeRules++
			if !strings.Contains(br.selectorHead, ".icon-badge."+state) || !strings.Contains(br.selectorHead, "."+state+" .icon-badge") {
				t.Errorf("the %s badge rule head %q must hold the compound and the descendant form", state, br.selectorHead)
			}
			sameDecls("the "+state+" badge rule", declsOf(br.declBody), badgeDecls)
		}
		if badgeRules != 1 {
			t.Errorf("found %d %s rules with a background, want exactly 1", badgeRules, state)
		}
	}

	for _, r := range rules {
		for _, state := range []string{"vis-held", "vis-removed"} {
			if !strings.Contains(r.selectorHead, state) {
				continue
			}
			for _, other := range []string{"vis-expired", "vis-retracted", "vis-hidden", "leaflet-popup"} {
				if strings.Contains(r.selectorHead, other) {
					t.Errorf("rule head %q names %s together with %s", r.selectorHead, state, other)
				}
			}
			otherNew := "vis-removed"
			if state == "vis-removed" {
				otherNew = "vis-held"
			}
			if strings.Contains(r.selectorHead, otherNew) {
				t.Errorf("rule head %q names both new states", r.selectorHead)
			}
		}
	}
}

func TestActivityModerationNoteStyle(t *testing.T) {
	rules := parseCSSRules(p416ReadStatic(t, "static/css/trust.css"))

	note, ok := ruleBySelector(rules, ".report-row__moderation-note")
	if !ok {
		t.Fatal("trust.css has no .report-row__moderation-note rule")
	}
	noteDecls := declsOf(note.declBody)
	for name, want := range map[string]string{
		"margin":        "var(--space-xs) 0 0",
		"font-size":     "var(--font-size-label)",
		"line-height":   "var(--line-height-label)",
		"font-weight":   "var(--font-weight-regular)",
		"color":         "var(--color-text)",
		"overflow-wrap": "anywhere",
	} {
		if got := noteDecls[name]; got != want {
			t.Errorf(".report-row__moderation-note %s is %q, want %q", name, got, want)
		}
	}
	if _, has := noteDecls["display"]; has {
		t.Errorf(".report-row__moderation-note declares display, it needs no hidden guard")
	}

	link, ok := ruleBySelector(rules, ".report-row__moderation-note a")
	if !ok {
		t.Fatal("trust.css has no .report-row__moderation-note a rule")
	}
	linkDecls := declsOf(link.declBody)
	if linkDecls["color"] != "var(--color-text)" || linkDecls["text-decoration"] != "underline" {
		t.Errorf(".report-row__moderation-note a declares %v, want the text colour and underline", linkDecls)
	}

	for _, r := range []cssRule{note, link} {
		if strings.Contains(r.declBody, "--color-text-muted") || strings.Contains(r.declBody, "--color-severity") {
			t.Errorf("rule %q uses a muted or severity colour", r.selectorHead)
		}
	}
}
