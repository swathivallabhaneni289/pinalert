package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file pins the "See this reporter" row of plan 09-09 (UI-SPEC 7d, P9-C19)
// in its own file, so it never edits the shared contract lists. Nothing here
// needs a database or a server. Helpers carry the tpl prefix.

// tplRuleBlocks returns the body of every CSS rule whose selector list names
// the class, so a selector list of two rules is checked on each body.
func tplRuleBlocks(t *testing.T, css, class string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?s)([^{}]*` + regexp.QuoteMeta(class) + `[^{}]*)\{([^}]*)\}`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(css, -1) {
		out = append(out, m[1]+"{"+m[2]+"}")
	}
	return out
}

func TestTrustProfileLinkRow(t *testing.T) {
	css := p13ReadStatic(t, "static/css/trust.css")
	rest := tplRuleBlocks(t, css, "a.trust-panel__profile")
	if len(rest) == 0 {
		t.Fatalf("trust.css has no rule for a.trust-panel__profile")
	}
	var base string
	for _, rule := range rest {
		if strings.Contains(rule, "min-height") {
			base = rule
		}
	}
	if base == "" {
		t.Fatalf("no a.trust-panel__profile rule sets min-height")
	}
	p13MustContain(t, "the profile row rule", base,
		"min-height: var(--touch-target-min)", "text-decoration: none", "border-top: 1px solid var(--color-border)")
	// Hover fills with the surface token and never underlines, and the focus ring
	// is drawn inset.
	p13MustContain(t, "trust.css", css,
		"a.trust-panel__profile:hover", "background: var(--color-surface)", "a.trust-panel__profile:focus-visible", "outline-offset: -2px")
	for _, rule := range rest {
		if regexp.MustCompile(`text-decoration:\s*underline`).MatchString(rule) {
			t.Errorf("a profile row rule underlines: %s", rule)
		}
	}

	js := p13ReadStatic(t, "static/js/trust.js")
	if n := strings.Count(js, "See this reporter"); n != 1 {
		t.Errorf("trust.js has %d copies of \"See this reporter\", want exactly 1", n)
	}
	p13MustContain(t, "trust.js", js, "setProfileLink: setProfileLink", "trust-panel__profile")
	body := jsFunctionBody(t, js, "setProfileLink(target, path)")
	p13MustContain(t, "setProfileLink", body, "PROFILE_PATH.test(path)", "Pinalert.setText(")
	p13MustNotContain(t, "setProfileLink", body, "innerHTML", "insertAdjacentHTML", "textContent =")

	// The two surfaces that show a trust object call it, and the Activity card
	// carries the path to the page as a data attribute.
	p13MustContain(t, "map.js", p13ReadStatic(t, "static/js/map.js"),
		"PinalertTrust.setProfileLink(card.trust, report.reporter_profile_path)")
	p13MustContain(t, "activity.js", p13ReadStatic(t, "static/js/activity.js"),
		"PinalertTrust.setProfileLink(trustBlock, row.dataset.reporterProfilePath)")
	p13MustContain(t, "profile.html.tmpl", p13ReadTemplate(t, "profile.html.tmpl"),
		`data-reporter-profile-path="{{.ReporterProfilePath}}"`)
}
