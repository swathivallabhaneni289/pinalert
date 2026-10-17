package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// This file pins the one reader of the Disclaimer and the Policy (Phase 3.4,
// plan 08, 03.4-CONTEXT D-31, D-27): one source for the text, one window, one
// way to close it by the X (plus Escape and the backdrop), and the places that
// open it.

// readerSentences are sentences of the legal text. Each must exist only in the
// shared body partials, so no second copy can drift from what is published.
var readerSentences = []string{
	"Pinalert is a free, unofficial app where people near an emergency share what they see.",
	"Pinalert can't send help.",
	"By using Pinalert you accept these rules.",
	"A critical report is published at once and checked afterwards.",
	"Ended reports stay under My activity, Ended nearby, for 7 days.",
	"Nothing here is deleted on a schedule yet.",
	"There are no advertising or tracking cookies.",
}

func readerTemplate(t *testing.T, name string) string {
	t.Helper()
	return stripReadTemplate(t, name)
}

func TestReaderOneSource(t *testing.T) {
	body := readerTemplate(t, "legal_body.html.tmpl")
	for _, def := range []string{`{{define "legal_disclaimer_body"}}`, `{{define "legal_policy_body"}}`} {
		if n := strings.Count(body, def); n != 1 {
			t.Errorf("legal_body.html.tmpl defines %s %d times, want 1", def, n)
		}
	}

	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".tmpl") || path == "templates/legal_body.html.tmpl" {
			return walkErr
		}
		raw, readErr := fs.ReadFile(TemplatesFS, path)
		if readErr != nil {
			return readErr
		}
		for _, sentence := range readerSentences {
			if strings.Contains(string(raw), sentence) {
				t.Errorf("%s holds the legal sentence %q; the text lives only in legal_body.html.tmpl (D-31)", path, sentence)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the templates: %v", err)
	}
	for _, sentence := range readerSentences {
		if n := strings.Count(body, sentence); n != 1 {
			t.Errorf("legal_body.html.tmpl holds %q %d times, want 1", sentence, n)
		}
	}

	// The standalone page and the window each include both parts once; the
	// first time flow (plan 10) includes the same two.
	for _, name := range []string{"reader.html.tmpl", "reader_dialog.html.tmpl"} {
		src := readerTemplate(t, name)
		for _, part := range []string{`{{template "legal_disclaimer_body" .Legal}}`, `{{template "legal_policy_body" .Legal}}`} {
			if n := strings.Count(src, part); n != 1 {
				t.Errorf("%s includes %s %d times, want 1", name, part, n)
			}
		}
	}

	// The partials read the server settings through the LegalText value alone.
	for _, banned := range []string{".Chrome.", ".Email", ".AccountID", "data-account-id"} {
		if strings.Contains(body, banned) {
			t.Errorf("legal_body.html.tmpl reads %q; it may use LegalText only (R-07, T-34-34)", banned)
		}
	}
}

func TestReaderPolicyAnchor(t *testing.T) {
	body := readerTemplate(t, "legal_body.html.tmpl")
	if n := strings.Count(body, `<h2 id="policy">`); n != 1 {
		t.Errorf("legal_body.html.tmpl holds h2 id=policy %d times, want 1", n)
	}
	_, policyPart, found := strings.Cut(body, `{{define "legal_policy_body"}}`)
	if !found {
		t.Fatalf("legal_body.html.tmpl has no policy part")
	}
	if !strings.HasPrefix(strings.TrimSpace(policyPart), `<h2 id="policy">Policy</h2>`) {
		t.Errorf("the policy part must open with h2 id=policy so a link can put it at the top")
	}
	// The anchor comes from the included partial; no template may carry a second
	// copy, or one page would hold the id twice.
	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".tmpl") || path == "templates/legal_body.html.tmpl" {
			return walkErr
		}
		raw, _ := fs.ReadFile(TemplatesFS, path)
		if strings.Contains(string(raw), `id="policy"`) {
			t.Errorf("%s holds its own id=policy; the anchor is in legal_body.html.tmpl only", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the templates: %v", err)
	}
	// The script scrolls the box to that heading.
	js := p408ReadStatic(t, "static/js/reader.js")
	if !strings.Contains(js, "'policy'") || !strings.Contains(js, "scrollBoxTo") {
		t.Errorf("reader.js must scroll the box to the policy heading")
	}
}

func TestReaderClosesWithX(t *testing.T) {
	dialog := readerTemplate(t, "reader_dialog.html.tmpl")
	if n := strings.Count(dialog, `aria-label="Close"`); n != 1 {
		t.Errorf("the window has %d controls named Close, want exactly one X", n)
	}
	if n := strings.Count(dialog, "<button"); n != 1 {
		t.Errorf("the window has %d buttons, want only the X", n)
	}
	visible := legalActionPattern.ReplaceAllString(dialog, "")
	for _, banned := range []string{"Cancel", "Done", "Dismiss", "Got it", "Back to"} {
		if strings.Contains(visible, banned) {
			t.Errorf("the window holds the word %q; the X is the only close control (D-31)", banned)
		}
	}
	for _, want := range []string{`role="dialog"`, `aria-modal="true"`, `aria-labelledby="reader-title"`, `app-window--reader`, `hidden`} {
		if !strings.Contains(dialog, want) {
			t.Errorf("the window lacks %s", want)
		}
	}

	page := readerTemplate(t, "reader.html.tmpl")
	if n := strings.Count(page, `aria-label="Close"`); n != 1 {
		t.Errorf("the standalone page has %d controls named Close, want exactly one X", n)
	}
	if !regexp.MustCompile(`<a class="legal-close" id="reader-close-link" href="/" aria-label="Close">`).MatchString(page) {
		t.Errorf("the standalone X must be a link to / named Close")
	}
	if strings.Contains(page, "Back to Pinalert") {
		t.Errorf("the standalone page keeps a Back to Pinalert link")
	}

	js := p408ReadStatic(t, "static/js/reader.js")
	for _, want := range []string{"'Escape'", "event.target === win", "closeButton.addEventListener('click', close)"} {
		if !strings.Contains(js, want) {
			t.Errorf("reader.js lacks %q: the X, Escape and the backdrop must all close the window", want)
		}
	}
	if strings.Contains(js, "innerHTML") || strings.Contains(js, "insertAdjacentHTML") || strings.Contains(js, "outerHTML") {
		t.Errorf("reader.js uses a markup parsing sink (T-34-37)")
	}

	css := stripCSSComments(p408ReadStatic(t, "static/css/window.css"))
	if !regexp.MustCompile(`\.app-window--reader \.app-window__panel\s*\{[^}]*width:\s*min\(820px, 92%\)[^}]*height:\s*min\(760px, 92%\)`).MatchString(css) {
		t.Errorf("window.css lacks the reader size, 820 by 760 on the laptop")
	}
}

func TestReaderOpeners(t *testing.T) {
	strip := readerTemplate(t, "notice_strip.html.tmpl")
	if n := strings.Count(strip, `data-open-reader="disclaimer"`); n != 1 {
		t.Errorf("the strip carries %d reader openers, want 1", n)
	}
	if !strings.Contains(strip, `href="/disclaimer" data-open-reader="disclaimer"`) {
		t.Errorf("the strip link keeps its plain href /disclaimer for a visitor without script")
	}

	settings := readerTemplate(t, "settings.html.tmpl")
	if n := strings.Count(settings, "data-open-reader"); n != 2 {
		t.Errorf("Settings carries %d reader openers, want 2 (Disclaimer and Policy)", n)
	}
	for _, want := range []string{`href="/disclaimer" data-open-reader="disclaimer"`, `href="/policy#policy" data-open-reader="policy"`} {
		if !strings.Contains(settings, want) {
			t.Errorf("Settings lacks the opener %s", want)
		}
	}

	// The window and its script are on exactly the pages that open it.
	for _, name := range []string{"index.html.tmpl", "settings.html.tmpl"} {
		src := readerTemplate(t, name)
		if n := strings.Count(src, `{{template "reader_dialog.html.tmpl" .}}`); n != 1 {
			t.Errorf("%s includes the reader window %d times, want 1", name, n)
		}
		if n := strings.Count(src, "/static/js/reader.js?v={{.AssetVersion}}\" defer"); n != 1 {
			t.Errorf("%s loads reader.js %d times with defer, want 1", name, n)
		}
		if !strings.Contains(src, "/static/css/window.css") || !strings.Contains(src, "/static/css/legal.css") {
			t.Errorf("%s must link window.css and legal.css for the reader", name)
		}
		if strings.Index(src, "/static/css/window.css") > strings.Index(src, "/static/css/phone.css") {
			t.Errorf("%s links window.css after phone.css", name)
		}
	}
	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return walkErr
		}
		name := strings.TrimPrefix(path, "templates/")
		raw, _ := fs.ReadFile(TemplatesFS, path)
		src := string(raw)
		if name != "index.html.tmpl" && name != "settings.html.tmpl" && strings.Contains(src, `{{template "reader_dialog.html.tmpl"`) {
			t.Errorf("%s includes the reader window; only Nearby and Settings open it", name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the templates: %v", err)
	}

	// The login screen keeps plain links to the standalone pages (D-29, D-31).
	login := readerTemplate(t, "login_gate.html.tmpl")
	for _, want := range []string{`href="/disclaimer"`, `href="/policy"`} {
		if !strings.Contains(login, want) {
			t.Errorf("the login screen lost its plain link %s", want)
		}
	}
	if strings.Contains(login, "data-open-reader") {
		t.Errorf("the login screen has no reader window; its links go to the standalone pages")
	}
}
