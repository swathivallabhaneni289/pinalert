package web

// Plan 05-18 contract tests: the Demo is offered inside the first-time flow
// (D-03, ROBUST-07, UI-SPEC 4e and 4f). Beside Continue on step 1, and in the
// step 2 Disagree box, which gets the restored second sentence and a third
// control so the sentence is never a dead pointer. Both controls are links
// styled as outlined buttons that go to /demo?from=welcome, so the Demo's Exit
// (plan 05-13) brings the person back to /welcome. Like the other contract
// files these inspect the embedded bytes; how a browser lays the pair out is
// recorded in the 05-18 SUMMARY (the browser check). Every helper carries the
// de18 prefix.

import (
	"regexp"
	"strings"
	"testing"
)

const (
	de18Href      = `href="/demo?from=welcome"`
	de18DisagreeP = `<p class="welcome__notice-lead" id="welcome-disagree-lead">Sorry, you need to agree before you can continue. You can still look around in the Demo without agreeing.</p>`
)

// de18Section returns the text of the template from the first occurrence of
// from up to the first occurrence of to after it.
func de18Section(t *testing.T, page, from, to string) string {
	t.Helper()
	start := strings.Index(page, from)
	if start == -1 {
		t.Fatalf("welcome.html.tmpl has no %q", from)
	}
	end := strings.Index(page[start:], to)
	if end == -1 {
		t.Fatalf("welcome.html.tmpl has no %q after %q", to, from)
	}
	return page[start : start+end]
}

// de18Rule returns the declarations of the rule with this exact head in the
// welcome stylesheet, outside any media query when media is empty, else inside
// the named one.
func de18Rule(t *testing.T, media, head string) map[string]string {
	t.Helper()
	css := p421ReadStatic(t, "css/welcome.css")
	// A rule inside an at-rule keeps its preamble in its head, so an exact head
	// match on the whole stylesheet is already outside every media query.
	rules := parseCSSRules(stripCSSComments(css))
	if media != "" {
		rules = p408MediaBlocks(css, media)
	}
	r, ok := ruleBySelector(rules, head)
	if !ok {
		t.Fatalf("welcome.css has no %q rule (media %q)", head, media)
	}
	return declsOf(r.declBody)
}

// TestWelcomeStepOneDemoPair pins step 1's actions: Try the demo outlined on the
// left, then Continue filled on the right, in the built pair pattern, with no
// hint line under them and no new step.
func TestWelcomeStepOneDemoPair(t *testing.T) {
	page := w10Template(t)

	actions := de18Section(t, page, `<div class="welcome__actions welcome__actions--pair welcome__actions--demo">`, `</section>`)
	want := `<div class="welcome__actions welcome__actions--pair welcome__actions--demo">
          <a class="btn" id="welcome-try-demo-1" ` + de18Href + `>Try the demo</a>
          <button type="button" class="btn btn--primary" id="welcome-continue-1">Continue</button>
        </div>`
	if !strings.HasPrefix(actions, want) {
		t.Errorf("step 1 actions are\n%s\nwant them to start with\n%s", actions, want)
	}
	// The pair is in step 1 and nowhere else; the old single Continue row is gone.
	step1 := de18Section(t, page, `id="welcome-step-1"`, `id="welcome-step-2"`)
	w10Count(t, "step 1", step1, `id="welcome-try-demo-1"`, 1)
	w10Count(t, "step 1", step1, `id="welcome-continue-1"`, 1)
	w10Count(t, "welcome.html.tmpl", page, `id="welcome-try-demo-1"`, 1)
	p13Ordered(t, step1, "step 1", `id="welcome-try-demo-1"`, `id="welcome-continue-1"`)
	if strings.Contains(step1, `<div class="welcome__actions">`) {
		t.Errorf("step 1 still holds the single Continue row")
	}
	// Outlined, not filled: only Continue is btn--primary.
	link := regexp.MustCompile(`<a [^>]*id="welcome-try-demo-1"[^>]*>`).FindString(step1)
	if link == "" || strings.Contains(link, "btn--primary") {
		t.Errorf("Try the demo %q must be an a element and not btn--primary", link)
	}
	// Nothing follows the actions inside the step: no hint line under the pair.
	tail := strings.TrimSpace(strings.TrimPrefix(actions, want))
	if tail != "" {
		t.Errorf("step 1 holds %q under the pair; the owner wants no hint text", tail)
	}
	// No new step: the flow stays four steps.
	w10Count(t, "welcome.html.tmpl", page, `<section class="welcome__step"`, 4)
}

// TestDisagreeBoxPointsToDemo pins the step 2 Disagree box: the built lead and
// the restored sentence, then Read again, Log out and Try the demo, in that
// order.
func TestDisagreeBoxPointsToDemo(t *testing.T) {
	page := w10Template(t)

	w10Count(t, "welcome.html.tmpl", page, de18DisagreeP, 1)
	w10Count(t, "welcome.html.tmpl", page, "You can still look around in the Demo without agreeing.", 1)

	actions := de18Section(t, page, `<div class="welcome__notice-actions">`, `<p class="inline-error" id="welcome-error-2"`)
	p13Ordered(t, actions, "disagree box", `id="welcome-reread"`, `id="welcome-logout"`, `id="welcome-try-demo-2"`)
	want := `<a class="btn" id="welcome-try-demo-2" ` + de18Href + `>Try the demo</a>`
	w10Count(t, "disagree box", actions, want, 1)
	// The link comes after the Log out form closes, as the third control.
	if i, j := strings.Index(actions, `</form>`), strings.Index(actions, `id="welcome-try-demo-2"`); i == -1 || j < i {
		t.Errorf("Try the demo must come after the Log out form")
	}
	// Both Demo links carry the literal word welcome and nothing else.
	w10Count(t, "welcome.html.tmpl", page, de18Href, 2)
	if m := regexp.MustCompile(`/demo[^"]*`).FindAllString(page, -1); len(m) != 2 || m[0] != "/demo?from=welcome" || m[1] != "/demo?from=welcome" {
		t.Errorf("the Demo links are %v, want exactly two of /demo?from=welcome", m)
	}
	// The box keeps its built pieces.
	p13MustContain(t, "disagree box", page,
		`<div class="welcome__notice welcome__notice--box" id="welcome-disagree-message" role="group" aria-labelledby="welcome-disagree-lead" tabindex="-1" hidden>`,
		`<button type="button" class="btn" id="welcome-reread">Read again</button>`,
		`<form class="welcome__notice-form" method="post" action="/auth/logout">`)
}

// TestWelcomeDemoLinksAreButtons pins how the two links look: a elements with
// the .btn class (outlined, 8px or 6px corners from .btn, no underline at rest,
// the owner's no underlined action rule), at least 48px tall by the rules of the
// pair and of the Disagree box, with the step 1 padding and no label wrapping,
// stacking below 340px with Continue last, and the third Disagree control full
// width on the phone and in the one row on the laptop.
func TestWelcomeDemoLinksAreButtons(t *testing.T) {
	page := w10Template(t)
	for _, id := range []string{"welcome-try-demo-1", "welcome-try-demo-2"} {
		tag := regexp.MustCompile(`<a [^>]*id="` + id + `"[^>]*>`).FindString(page)
		if tag == "" {
			t.Errorf("%s is not an a element", id)
			continue
		}
		p13MustContain(t, id, tag, `class="btn"`, de18Href)
		p13MustNotContain(t, id, tag, "btn--primary", "style=", "target=")
	}

	// A link styled as .btn never underlines at rest (the base rule).
	btn := declsOf(mustRule(t, parseCSSRules(p421ReadStatic(t, "css/main.css")), ".btn").declBody)
	if btn["text-decoration"] != "none" {
		t.Errorf(".btn text-decoration is %q, want none, so a.btn draws no underline", btn["text-decoration"])
	}

	// 48px tall: the pair's buttons and the Disagree box's buttons.
	for _, head := range []string{".welcome__actions .btn", ".welcome__notice-actions .btn"} {
		d := de18Rule(t, "", head)
		if d["min-height"] != "48px" {
			t.Errorf("%s min-height is %q, want 48px", head, d["min-height"])
		}
	}

	// Step 1: the narrower padding and a label that never wraps.
	pad := de18Rule(t, "", ".welcome__actions--demo .btn")
	if pad["padding"] != "0 var(--space-md)" || pad["white-space"] != "nowrap" {
		t.Errorf(".welcome__actions--demo .btn is %v, want padding 0 var(--space-md) and white-space nowrap", pad)
	}

	// Below 340px the pair stacks in source order: Try the demo above, Continue last.
	stack := de18Rule(t, "(max-width: 339.98px)", ".welcome__actions--demo")
	if stack["flex-direction"] != "column" {
		t.Errorf("below 340px .welcome__actions--demo is %v, want flex-direction column", stack)
	}
	css := stripCSSComments(p421ReadStatic(t, "css/welcome.css"))
	if strings.Contains(css, "column-reverse") || regexp.MustCompile(`(^|[\s;{])order\s*:`).MatchString(css) {
		t.Errorf("welcome.css reorders controls; the stack must follow the source order")
	}

	// The Disagree box: third control full width on the phone, one row on the laptop.
	third := de18Rule(t, "", "#welcome-try-demo-2")
	if third["grid-column"] != "1 / -1" {
		t.Errorf("#welcome-try-demo-2 grid-column is %q on the phone, want 1 / -1", third["grid-column"])
	}
	laptop := de18Rule(t, "(min-width: 900px)", ".welcome__notice-actions")
	if laptop["grid-template-columns"] != "repeat(3, max-content)" {
		t.Errorf("laptop .welcome__notice-actions columns are %q, want repeat(3, max-content)", laptop["grid-template-columns"])
	}
	laptopThird := de18Rule(t, "(min-width: 900px)", "#welcome-try-demo-2")
	if laptopThird["grid-column"] != "auto" {
		t.Errorf("laptop #welcome-try-demo-2 grid-column is %q, want auto", laptopThird["grid-column"])
	}
}

// mustRule returns the first rule with this exact head or fails the test.
func mustRule(t *testing.T, rules []cssRule, head string) cssRule {
	t.Helper()
	r, ok := ruleBySelector(rules, head)
	if !ok {
		t.Fatalf("no %q rule", head)
	}
	return r
}

// TestWelcomeScriptIsUnchangedByTheDemoLinks pins that welcome.js neither names
// the Demo nor handles the two links: they are plain navigation.
func TestWelcomeScriptIsUnchangedByTheDemoLinks(t *testing.T) {
	js := stripJSComments(p421ReadStatic(t, "js/welcome.js"))
	for _, banned := range []string{"try-demo", "/demo"} {
		if strings.Contains(js, banned) {
			t.Errorf("welcome.js holds %q; the Demo links are plain links", banned)
		}
	}
}
