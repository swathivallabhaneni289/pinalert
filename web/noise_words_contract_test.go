package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 3.3 plan 06: the copy contract for alerts (owner rule 2026-10-18,
// decision 4). Every phone notification appears with no sound and no
// vibration, so no word on screen may promise or mention noise. This file holds
// the banned list, the list of files whose user visible words are scanned, and
// the checks for the install guide partial and for notify.js.

// bannedNoiseWords is the closed list of words that never appear in user
// visible copy. They are matched as whole words, ignoring case.
var bannedNoiseWords = []string{
	"buzz", "buzzes", "vibrate", "vibration", "sound", "ring",
	"silent", "silently", "quiet", "quietly", "hear",
}

// noiseCheckedFiles lists the embedded files whose copy is scanned: paths under
// "templates/" are read from TemplatesFS, every other path from StaticFS.
//
// Plan 03.3-06 lists only its own files. Plans 04 and 05 write the worker
// (static/sw.js) and the shared head (templates/app_head.html.tmpl) in the same
// wave, so plan 13 adds them once all of wave 1 is merged. Plans 13 and 14 also
// add the welcome and Settings files (welcome.html.tmpl, settings.html.tmpl,
// welcome.js, settings-alerts.js). A listed file that is missing from the
// embedded trees fails the test; it is never skipped.
//
// Amended 2026-10-18 by Phase 3.3 plan 13 (decision 4): the worker, the shared
// head and the welcome files are listed now. sw.js is scanned by its string
// literals only, so the worker's option names are not mistaken for copy.
//
// Amended 2026-10-18 by Phase 3.3 plan 14 (decision 4): the Settings template
// and settings-alerts.js are listed now.
var noiseCheckedFiles = []string{
	"templates/device_guide.html.tmpl",
	"static/js/notify.js",
	"static/js/sw.js",
	"templates/app_head.html.tmpl",
	"templates/welcome.html.tmpl",
	"static/js/welcome.js",
	"templates/settings.html.tmpl",
	"static/js/settings-alerts.js",
}

var (
	// {{/* ... */}} template comments and <!-- ... --> HTML comments are not
	// shown to anyone, so they are removed before a template is scanned.
	templateCommentPattern = regexp.MustCompile(`(?s)\{\{/\*.*?\*/\}\}`)
	htmlCommentPattern     = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlTagPattern         = regexp.MustCompile(`(?s)<[^>]*>`)
	noiseWordPattern       = regexp.MustCompile(`(?i)\b(` + strings.Join(bannedNoiseWords, "|") + `)\b`)
)

// readEmbeddedCopyFile reads one entry of noiseCheckedFiles.
func readEmbeddedCopyFile(t *testing.T, path string) string {
	t.Helper()
	var raw []byte
	var err error
	if strings.HasPrefix(path, "templates/") {
		raw, err = TemplatesFS.ReadFile(path)
	} else {
		raw, err = StaticFS.ReadFile(path)
	}
	if err != nil {
		t.Fatalf("%s is listed in noiseCheckedFiles but cannot be read from the embedded trees: %v", path, err)
	}
	return string(raw)
}

// templateCopy is a template with its comments removed.
func templateCopy(src string) string {
	return htmlCommentPattern.ReplaceAllString(templateCommentPattern.ReplaceAllString(src, ""), "")
}

// scriptStringLiterals returns the content of every string literal of a script
// whose comments were removed. A word that is only an option name or an
// identifier of the code is not copy, so only literals are scanned.
func scriptStringLiterals(src string) []string {
	code := stripJSComments(src)
	var out []string
	for i := 0; i < len(code); i++ {
		quote := code[i]
		if quote != '\'' && quote != '"' && quote != '`' {
			continue
		}
		var lit strings.Builder
		j := i + 1
		for j < len(code) && code[j] != quote {
			if code[j] == '\\' && j+1 < len(code) {
				j++
			}
			lit.WriteByte(code[j])
			j++
		}
		out = append(out, lit.String())
		i = j
	}
	return out
}

// noiseWordsIn lists the banned words found in text, lower cased.
func noiseWordsIn(text string) []string {
	var found []string
	for _, m := range noiseWordPattern.FindAllString(text, -1) {
		found = append(found, strings.ToLower(m))
	}
	return found
}

func TestNoiseScannerFindsWords(t *testing.T) {
	if len(bannedNoiseWords) != 11 {
		t.Fatalf("bannedNoiseWords holds %d words, want the eleven of the owner rule", len(bannedNoiseWords))
	}
	for _, word := range bannedNoiseWords {
		if got := noiseWordsIn("Your phone will " + strings.ToUpper(word) + " now."); len(got) != 1 {
			t.Errorf("the scanner missed %q in a sentence: %v", word, got)
		}
	}
	for _, clean := range []string{"during the spring", "a Ringo song", "the shearing", "Add to Home Screen"} {
		if got := noiseWordsIn(clean); len(got) != 0 {
			t.Errorf("%q is not a banned word but was flagged: %v", clean, got)
		}
	}
	if got := noiseWordsIn(templateCopy(`{{/* quiet */}}<!-- sound --><p>Hello</p>`)); len(got) != 0 {
		t.Errorf("comments are not copy but were flagged: %v", got)
	}
	lits := scriptStringLiterals("var a = 'one'; // silent\nvar b = \"two \\\" hear\"; /* quiet */ var c = `three`;")
	if len(lits) != 3 || lits[0] != "one" || lits[2] != "three" {
		t.Errorf("scriptStringLiterals returned %q", lits)
	}
	if got := noiseWordsIn(strings.Join(lits, " ")); len(got) != 1 || got[0] != "hear" {
		t.Errorf("only the literal word may be found, got %v", got)
	}
}

// TestNoNoiseWordsInUserCopy fails when any listed file holds a banned word in
// its user visible words: a template's text and attributes, and the string
// literals of a script.
func TestNoNoiseWordsInUserCopy(t *testing.T) {
	if len(noiseCheckedFiles) == 0 {
		t.Fatal("noiseCheckedFiles is empty, so every check below would pass vacuously")
	}
	for _, path := range noiseCheckedFiles {
		src := readEmbeddedCopyFile(t, path)
		var copyText string
		if strings.HasPrefix(path, "templates/") {
			copyText = templateCopy(src)
		} else {
			copyText = strings.Join(scriptStringLiterals(src), "\n")
		}
		for _, word := range noiseWordsIn(copyText) {
			t.Errorf("%s: user visible copy holds %q; notifications are silent and no copy may mention noise (owner rule 2026-10-18)", path, word)
		}
	}
}

// TestNotifyLeavesInstallToTheBrowser (decision 5, UI-SPEC 4c): Android and
// laptop browsers keep their own install control, so notify.js never names the
// Chromium install prompt event, never keeps an install event to prompt later,
// and adds no listener of its own.
func TestNotifyLeavesInstallToTheBrowser(t *testing.T) {
	code := stripJSComments(readEmbeddedCopyFile(t, "static/js/notify.js"))
	for _, forbidden := range []string{"beforeinstallprompt", "appinstalled", "deferredPrompt", "installPrompt", ".prompt(", "addEventListener"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("static/js/notify.js holds %q; the browser owns the install prompt and notify.js adds no listener (decision 5)", forbidden)
		}
	}
}

// TestDeviceGuideCopy pins the words of the iPhone install guide (UI-SPEC
// Surface 4a): the lead, the three steps with their key words in strong, the
// three icons, and no title, button, link or long dash.
func TestDeviceGuideCopy(t *testing.T) {
	src := readEmbeddedCopyFile(t, "templates/device_guide.html.tmpl")
	body := templateCopy(src)

	for _, want := range []string{
		`{{define "device_guide"}}`,
		`<p class="device-guide__lead">On this device, alerts work once Pinalert is on your Home Screen.</p>`,
		`<ol class="device-guide__steps">`,
		`<span class="app-icon app-icon--share" aria-hidden="true"></span>`,
		`<span class="app-icon app-icon--square-plus" aria-hidden="true"></span>`,
		`<span class="app-icon app-icon--bell" aria-hidden="true"></span>`,
		`Tap the <strong>Share</strong> button.`,
		`Choose <strong>Add to Home Screen</strong>, then tap <strong>Add</strong>.`,
		`Open Pinalert from your Home Screen. In Settings, turn on <strong>On this device</strong>.`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("device_guide.html.tmpl lacks %s", want)
		}
	}
	if got := strings.Count(body, `<li class="device-guide__step">`); got != 3 {
		t.Errorf("the guide holds %d steps, want 3", got)
	}
	if got := strings.Count(body, `class="device-guide__tile"`); got != 3 {
		t.Errorf("the guide holds %d icon tiles, want 3", got)
	}
	// The plain text of the steps, tags removed.
	var steps []string
	for _, li := range regexp.MustCompile(`(?s)<li[^>]*>.*?</li>`).FindAllString(body, -1) {
		steps = append(steps, strings.TrimSpace(htmlTagPattern.ReplaceAllString(li, "")))
	}
	wantSteps := []string{
		"Tap the Share button.",
		"Choose Add to Home Screen, then tap Add.",
		"Open Pinalert from your Home Screen. In Settings, turn on On this device.",
	}
	if len(steps) != len(wantSteps) {
		t.Fatalf("found %d step texts %q, want %d", len(steps), steps, len(wantSteps))
	}
	for i := range wantSteps {
		if steps[i] != wantSteps[i] {
			t.Errorf("step %d reads %q, want %q", i+1, steps[i], wantSteps[i])
		}
	}
	// No title (the step title or the window title holds it), no control, no
	// hint line.
	for _, forbidden := range []string{"<h1", "<h2", "<h3", "<button", "<a ", "<input", "hint"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("device_guide.html.tmpl holds %q, which the guide never carries (UI-SPEC Surface 4)", forbidden)
		}
	}
	for _, dash := range []string{string(rune(0x2013)), string(rune(0x2014))} {
		if strings.Contains(src, dash) {
			t.Errorf("device_guide.html.tmpl holds the long dash U+%04X", []rune(dash)[0])
		}
	}
}
