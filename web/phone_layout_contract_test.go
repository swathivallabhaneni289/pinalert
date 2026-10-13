package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of quick task 261013-lrw, the
// phone layout under 900px. Every helper and test name carries the phone
// prefix so a later change can add its own without a clash. Nothing here
// needs a database or a server: the tests read the embedded static and
// template trees and inspect the shipped text, like the other contract files.
//
// The departures from the owner's pictures and from the house rules are
// written in .planning/sketches/005-phone-layout/README.md.

const phoneBlockMedia = "@media (max-width: 899.98px)"

// phoneBaseHideHeads are the five phone only classes that phone.css hides at
// every width outside its media block. The rule declares nothing else.
var phoneBaseHideHeads = []string{
	".feed-heading", ".bottom-nav", ".report-row__away", ".profile-logout", ".profile-record",
}

// phoneRules parses phone.css alone.
func phoneRules(t *testing.T) []p16CSSRule {
	t.Helper()
	return p16ParseCSS("static/css/phone.css", p16ReadStatic(t, "static/css/phone.css"))
}

// TestPhoneStylesheetIsLinkedLast pins the load order: phone.css follows every
// other app stylesheet on the feed and on the Activity page, with the same
// AssetVersion query the others carry, so it can override main.css's narrow
// rules without a specificity trick.
func TestPhoneStylesheetIsLinkedLast(t *testing.T) {
	phone := `/static/css/phone.css?v={{.AssetVersion}}`

	index := p13ReadTemplate(t, "index.html.tmpl")
	p13Ordered(t, index, "index.html.tmpl",
		`/static/css/main.css?v={{.AssetVersion}}`,
		`/static/css/modal.css?v={{.AssetVersion}}`,
		`/static/css/feed.css?v={{.AssetVersion}}`,
		`/static/css/trust.css?v={{.AssetVersion}}`,
		`/static/css/auth.css?v={{.AssetVersion}}`,
		phone,
	)
	if n := strings.Count(index, phone); n != 1 {
		t.Errorf("index.html.tmpl links phone.css %d times, want exactly 1", n)
	}

	profile := p13ReadTemplate(t, "profile.html.tmpl")
	p13Ordered(t, profile, "profile.html.tmpl",
		`/static/css/main.css?v={{.AssetVersion}}`,
		`/static/css/feed.css?v={{.AssetVersion}}`,
		`/static/css/trust.css?v={{.AssetVersion}}`,
		`/static/css/auth.css?v={{.AssetVersion}}`,
		phone,
	)
	if n := strings.Count(profile, phone); n != 1 {
		t.Errorf("profile.html.tmpl links phone.css %d times, want exactly 1", n)
	}
}

// TestPhoneRulesLiveInTheNarrowBlock pins that nothing in phone.css can reach
// a screen of 900px or more, except the base rule that hides the phone only
// elements there. Colours, sizes and weights are tokens, and the shapes keep
// the house corner limit.
func TestPhoneRulesLiveInTheNarrowBlock(t *testing.T) {
	rules := phoneRules(t)
	if len(rules) == 0 {
		t.Fatal("phone.css holds no rules")
	}

	baseRules := 0
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`)
	for _, r := range rules {
		switch r.Media {
		case "":
			baseRules++
			if len(r.Decls) != 1 || r.Decls["display"] != "none" {
				t.Errorf("base rule %q declares %v, want display none only", r.Head, r.Decls)
			}
			for _, part := range p16HeadParts(r.Head) {
				found := false
				for _, want := range phoneBaseHideHeads {
					if part == want {
						found = true
					}
				}
				if !found {
					t.Errorf("base rule head %q is not one of the five phone only classes", part)
				}
			}
		case phoneBlockMedia:
		default:
			t.Errorf("rule %q sits in %q, want only the %s block", r.Head, r.Media, phoneBlockMedia)
		}

		for prop, value := range r.Decls {
			if hex.MatchString(value) {
				t.Errorf("%q %s: %q uses a raw hex colour, tokens only", r.Head, prop, value)
			}
			switch prop {
			case "font-size":
				if !strings.HasPrefix(value, "var(--font-size-") {
					t.Errorf("%q font-size %q is not a type size token", r.Head, value)
				}
			case "font-weight":
				if value != "var(--font-weight-regular)" && value != "var(--font-weight-semibold)" {
					t.Errorf("%q font-weight %q is not one of the two weight tokens", r.Head, value)
				}
			case "border-radius":
				for _, component := range strings.Fields(value) {
					ok := component == "0" || component == "8px" || component == "50%" || component == "0px"
					if !ok {
						t.Errorf("%q border-radius %q is more than the 8px house corner", r.Head, value)
					}
				}
			}
			if strings.Contains(value, "!important") {
				t.Errorf("%q %s: the important flag is not allowed here", r.Head, prop)
			}
		}
	}
	if baseRules != 1 {
		t.Errorf("phone.css has %d base rules, want exactly 1 (the grouped hide rule)", baseRules)
	}
}

// TestPhoneHeadsNeverEqualAnExactLookupHead pins that no phone.css rule
// carries a head another contract test looks up by exact text in main.css,
// feed.css, trust.css or auth.css, so those tests keep reading the base rules.
func TestPhoneHeadsNeverEqualAnExactLookupHead(t *testing.T) {
	forbidden := map[string]bool{
		".fab": true, ".theme-toggle": true, ".app-header": true, ".account-trigger": true,
		".profile-page": true, "#toast": true, ".icon-badge": true, ".icon-badge--pin": true,
		".view-toggle": true, ".report-row": true, ".app-shell": true, ".pane": true,
		".pane--map": true, ".pane--list": true, "#map": true, "#report-list": true,
		".profile-back-link": true, ".profile-email": true, ".profile-activity h2": true,
	}
	for _, r := range phoneRules(t) {
		for _, part := range p16HeadParts(r.Head) {
			// The five phone only classes are new, so a base hide rule is fine.
			if r.Media == "" {
				continue
			}
			if forbidden[part] {
				t.Errorf("phone.css head %q equals a head other tests read by exact text, add a container prefix", part)
			}
		}
	}
}

// TestPhoneBottomNavMarkup pins the bottom bar on the feed: exactly two tabs,
// Feed current, Activity linking to the account page, and no Past tab (the
// history view does not exist, so a Past tab would be a dead control).
func TestPhoneBottomNavMarkup(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")

	nav := regexp.MustCompile(`(?s)<nav class="bottom-nav"[^>]*>(.*?)</nav>`).FindStringSubmatch(index)
	if nav == nil {
		t.Fatal("index.html.tmpl has no nav.bottom-nav")
	}
	tabs := regexp.MustCompile(`<a class="bottom-nav__tab"([^>]*)>([^<]*)</a>`).FindAllStringSubmatch(nav[1], -1)
	if len(tabs) != 2 {
		t.Fatalf("bottom nav has %d tabs, want 2", len(tabs))
	}
	if !strings.Contains(tabs[0][1], `href="/"`) || !strings.Contains(tabs[0][1], `aria-current="page"`) ||
		strings.TrimSpace(tabs[0][2]) != "Feed" {
		t.Errorf("first tab is %q %q, want the current Feed tab linking to /", tabs[0][1], tabs[0][2])
	}
	if !strings.Contains(tabs[1][1], `href="/profile"`) || strings.Contains(tabs[1][1], "aria-current") ||
		strings.TrimSpace(tabs[1][2]) != "Activity" {
		t.Errorf("second tab is %q %q, want the plain Activity tab linking to /profile", tabs[1][1], tabs[1][2])
	}
	if strings.Contains(nav[1], "Past") {
		t.Errorf("bottom nav names a Past tab, which needs a history view that does not exist")
	}
	if strings.Contains(index, `class="view-toggle" id="view-toggle"`) == false {
		t.Errorf("the view toggle button stays in the markup, only phone.css hides it on phones")
	}
}

// TestPhoneFeedHeadingMarkup pins the heading and the live count slot inside
// the list pane, in front of the list itself.
func TestPhoneFeedHeadingMarkup(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")
	p13Ordered(t, index, "index.html.tmpl",
		`class="pane pane--list"`,
		`<div class="feed-heading">`,
		`<h2 class="feed-heading__title">Nearby now</h2>`,
		`<p class="feed-heading__count" id="feed-count"></p>`,
		`<ol id="report-list">`,
	)
	if n := strings.Count(index, `id="feed-count"`); n != 1 {
		t.Errorf("index.html.tmpl has %d feed-count nodes, want 1", n)
	}
}

// TestPhoneFeedScriptContract pins feed.js: the count line is real numbers
// written through setText, the distance line follows the reader's own
// position rule, and the phone list scroll sets only the list pane's own
// scroll position, from the map branch of the one onSelect subscriber.
func TestPhoneFeedScriptContract(t *testing.T) {
	code := p19Code(t, "static/js/feed.js")

	count := jsFunctionBody(t, code, "updateFeedCount(state)")
	p13MustContain(t, "updateFeedCount", count,
		"Pinalert.config.defaultRadiusKm", "state.reports.length", "'disputed'", "'active'",
		"Pinalert.setText(feedCountEl, text)")
	p13MustNotContain(t, "updateFeedCount", count, "innerHTML", "insertAdjacentHTML")

	render := jsFunctionBody(t, code, "render(state)")
	p13Ordered(t, render, "render", "updateFeedCount(state);", "state.status === 'idle'")

	create := jsFunctionBody(t, code, "createRow(id)")
	p13MustContain(t, "createRow", create, "'report-row__away'", "body.appendChild(away)", "away: away")
	update := jsFunctionBody(t, code, "updateRow(row, report)")
	p13Ordered(t, update, "updateRow",
		"Pinalert.feedIsAroundReader()", "Pinalert.formatDistance(report.distance_km)",
		"Pinalert.setText(row.away, away)", "row.away.hidden = !away")

	p13MustContain(t, "feed.js", code, "var PHONE_QUERY = '(max-width: 899.98px)';")

	reveal := jsFunctionBody(t, code, "revealRowInListPane(id)")
	p13MustContain(t, "revealRowInListPane", reveal,
		"window.matchMedia(PHONE_QUERY).matches", "closest('.pane--list')", "pane.scrollTo(", "rowsById[String(id)]")
	banned := regexp.MustCompile(`(window|document\.documentElement|document\.body)\.scroll|scrollingElement|scrollIntoView`)
	if m := banned.FindString(reveal); m != "" {
		t.Errorf("revealRowInListPane calls %q, it may set only the list pane's own scroll position", m)
	}

	if n := strings.Count(code, "revealRowInListPane("); n != 2 {
		t.Errorf("revealRowInListPane appears %d times (definition and one call), want 2", n)
	}
	subscriber, anchorFound, terminatorFound := windowAfter(code, "Pinalert.onSelect(", "});")
	if !anchorFound || !terminatorFound {
		t.Fatalf("could not bound the Pinalert.onSelect subscriber in feed.js")
	}
	p13Ordered(t, subscriber, "onSelect subscriber",
		"source === 'map'", "applySelectionHighlight(id);", "revealRowInListPane(id);")

	activate := jsFunctionBody(t, code, "activateRow(id)")
	p13MustNotContain(t, "activateRow", activate, "revealRowInListPane")
}
