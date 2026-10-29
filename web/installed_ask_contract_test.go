package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 7.1 plan 13c (D-17, D-18): Nearby asks about alerts once inside the
// installed iPhone or iPad app, with the same words as the sign up alerts
// pop-up of the 3.3 gap fix. These checks read the shipped templates so the
// words cannot drift. Helpers carry the i13c prefix.

var (
	i13cTitle  = regexp.MustCompile(`(?s)<h2[^>]*id="[a-z-]*ask-title"[^>]*>(.*?)</h2>`)
	i13cLabel  = regexp.MustCompile(`(?s)<label[^>]*>\s*<input type="checkbox"[^>]*><span>(.*?)</span></label>`)
	i13cButton = regexp.MustCompile(`(?s)<button type="button" class="([^"]*)" id="[a-z-]*-(later|allow)">(.*?)</button>`)
)

// i13cWords pulls the words of one pop-up: the title up to the home area, the
// tick box label, and the two button labels with their classes, in order.
func i13cWords(t *testing.T, src string, dialogID string) (titleStart, label string, buttons []string, ticked bool) {
	t.Helper()
	at := strings.Index(src, `<dialog id="`+dialogID+`"`)
	if at == -1 {
		t.Fatalf("no dialog with id %q", dialogID)
	}
	end := strings.Index(src[at:], "</dialog>")
	if end == -1 {
		t.Fatalf("dialog %q is not closed", dialogID)
	}
	box := src[at : at+end]
	m := i13cTitle.FindStringSubmatch(box)
	if m == nil {
		t.Fatalf("dialog %q has no title", dialogID)
	}
	titleStart = m[1]
	if cut := strings.Index(titleStart, "{{"); cut != -1 {
		titleStart = titleStart[:cut]
	}
	l := i13cLabel.FindStringSubmatch(box)
	if l == nil {
		t.Fatalf("dialog %q has no tick box label", dialogID)
	}
	label = strings.TrimSpace(l[1])
	for _, b := range i13cButton.FindAllStringSubmatch(box, -1) {
		buttons = append(buttons, b[2]+"|"+strings.TrimSpace(b[3])+"|"+b[1])
	}
	ticked = regexp.MustCompile(`<input type="checkbox"[^>]*\bchecked\b`).MatchString(box)
	return titleStart, label, buttons, ticked
}

// TestInstalledAlertsAskMatchesSignUp pins that the Nearby dialog uses the title
// words, the Reliable reporter tick box label and the two buttons of the sign up
// alerts pop-up, and that the box starts unticked in both.
func TestInstalledAlertsAskMatchesSignUp(t *testing.T) {
	welcome := templateCopy(stripReadTemplate(t, "welcome.html.tmpl"))
	installed := templateCopy(stripReadTemplate(t, "installed_alerts_ask.html.tmpl"))

	wTitle, wLabel, wButtons, wTicked := i13cWords(t, welcome, "welcome-alerts-ask")
	iTitle, iLabel, iButtons, iTicked := i13cWords(t, installed, "installed-alerts-ask")

	if wTitle != "Get alerts for critical reports near " {
		t.Fatalf("the sign up title words changed to %q; update this test and welcome.js ASK_TITLE_START together", wTitle)
	}
	if iTitle != wTitle {
		t.Errorf("title words = %q, sign up pop-up has %q", iTitle, wTitle)
	}
	if !strings.Contains(installed, `Get alerts for critical reports near {{.HomeAreaName}}?</h2>`) {
		t.Errorf("the title must end with the home area and a question mark")
	}
	if iLabel != wLabel || iLabel == "" {
		t.Errorf("tick box label = %q, sign up pop-up has %q", iLabel, wLabel)
	}
	if wTicked || iTicked {
		t.Errorf("the Reliable tick box starts unticked in both pop-ups (sign up %v, installed %v)", wTicked, iTicked)
	}
	if len(wButtons) != 2 || len(iButtons) != 2 {
		t.Fatalf("each pop-up has Not now and Allow: sign up %v, installed %v", wButtons, iButtons)
	}
	for i := range wButtons {
		if wButtons[i] != iButtons[i] {
			t.Errorf("button %d = %q, sign up pop-up has %q", i, iButtons[i], wButtons[i])
		}
	}
	if !strings.HasPrefix(iButtons[0], "later|Not now|") || !strings.HasPrefix(iButtons[1], "allow|Allow|") {
		t.Errorf("buttons must be Not now then Allow, got %v", iButtons)
	}

	// The ids and the attributes the script and the page need.
	for _, want := range []string{
		`{{define "installed_alerts_ask"}}`,
		`class="logout-confirm"`,
		`id="installed-alerts-ask-title"`,
		`id="installed-alerts-ask-reliable"`,
		`id="installed-alerts-ask-later"`,
		`id="installed-alerts-ask-allow"`,
		`data-push-key="{{.PushPublicKey}}"`,
		`data-account-id="{{.AccountID}}"`,
	} {
		if !strings.Contains(installed, want) {
			t.Errorf("installed_alerts_ask.html.tmpl lacks %s", want)
		}
	}
	if strings.Contains(installed, "innerHTML") || regexp.MustCompile(`<dialog[^>]*\sopen\b`).MatchString(installed) {
		t.Errorf("the dialog starts closed and holds no script sink")
	}
	// The script keeps the title words in step with the sign up script.
	js := p421ReadStatic(t, "js/welcome.js")
	if !strings.Contains(js, "var ASK_TITLE_START = '"+wTitle+"';") {
		t.Errorf("welcome.js ASK_TITLE_START no longer equals the template title words %q", wTitle)
	}
}

// TestInstalledAlertsAskOnlyOnNearby pins that only the Nearby template includes
// the dialog and loads the script, inside a condition on the push key and the
// home area name, and that the Low data view and the Demo carry neither.
func TestInstalledAlertsAskOnlyOnNearby(t *testing.T) {
	src := templateCopy(stripReadTemplate(t, "index.html.tmpl"))
	const call = `{{template "installed_alerts_ask" .}}`
	if n := strings.Count(src, call); n != 1 {
		t.Fatalf("index.html.tmpl includes the partial %d times, want 1", n)
	}
	ifAt := strings.Index(src, `{{if and .PushPublicKey .HomeAreaName}}`)
	if ifAt == -1 {
		t.Fatalf("the include must sit in {{if and .PushPublicKey .HomeAreaName}}")
	}
	callAt := strings.Index(src, call)
	endAt := strings.Index(src[ifAt:], "{{end}}")
	if callAt < ifAt || endAt == -1 || callAt > ifAt+endAt {
		t.Errorf("the include is not inside the condition")
	}
	script := `<script src="/static/js/installed-ask.js?v={{.AssetVersion}}" defer></script>`
	if n := strings.Count(src, script); n != 1 {
		t.Errorf("index.html.tmpl holds the deferred script tag %d times, want 1", n)
	}
	if bodyAt := strings.Index(src, "<body"); strings.Index(src, script) < bodyAt || callAt < bodyAt {
		t.Errorf("the dialog and its script belong in the body")
	}

	for _, name := range []string{"low.html.tmpl", "demo.html.tmpl", "demo_activity.html.tmpl", "demo_alerts.html.tmpl", "welcome.html.tmpl"} {
		other := stripReadTemplate(t, name)
		if strings.Contains(other, "installed_alerts_ask") || strings.Contains(other, "installed-ask.js") {
			t.Errorf("%s must not include the installed alerts ask", name)
		}
	}
}
