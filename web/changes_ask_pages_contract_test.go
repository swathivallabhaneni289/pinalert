package web

import (
	"strings"
	"testing"
)

// Phase 3.3 plan 21 (the 03.3-UAT gap for test 4, owner 2026-10-27): Nearby and
// Low data, the two pages where people post and vote, render the changes_ask
// partial behind the ChangesAsk flag and load changes-ask.js. These checks read
// the shipped templates; the handler tests in internal/api/handlers pin when the
// flag is true. Helpers carry the c33p21 prefix.

const c33p21Script = `/static/js/changes-ask.js?v={{.AssetVersion}}`

// c33p21Check pins the include and the script tag of one page template.
// afterMarker is a snippet that must come before the include (the end of the
// page shell).
//
// Amended by the 03.3-REVIEW fix WR-03: the changes-ask.js tag sits inside the
// same {{if .ChangesAsk}} block as the partial, right after it, so an account
// that can never see the pop-up does not load the script. It used to follow
// report-form.js for every account. The callers in votes.js and report-form.js
// check window.PinalertChangesAsk before they use it, and the script reads
// window.PinalertNotify only when it is offered, so its place among the
// deferred scripts does not matter.
func c33p21Check(t *testing.T, name, afterMarker string) {
	t.Helper()
	src := templateCopy(stripReadTemplate(t, name))

	if n := strings.Count(src, "{{if .ChangesAsk}}"); n != 1 {
		t.Fatalf("%s holds {{if .ChangesAsk}} %d times, want 1", name, n)
	}
	if n := strings.Count(src, `{{template "changes_ask"}}`); n != 1 {
		t.Fatalf(`%s holds {{template "changes_ask"}} %d times, want 1`, name, n)
	}
	ifAt := strings.Index(src, "{{if .ChangesAsk}}")
	callAt := strings.Index(src, `{{template "changes_ask"}}`)
	endAt := strings.Index(src[ifAt:], "{{end}}")
	if endAt == -1 || callAt < ifAt || callAt > ifAt+endAt {
		t.Errorf(`%s: {{template "changes_ask"}} must sit inside the {{if .ChangesAsk}} block`, name)
	}

	// After the page shell closes and before the first script tag of the body,
	// as a direct child of the body like the welcome pop-up (P-4).
	markerAt := strings.Index(src, afterMarker)
	if markerAt == -1 {
		t.Fatalf("%s has no %q", name, afterMarker)
	}
	if ifAt < markerAt {
		t.Errorf("%s: the changes pop-up must come after %q", name, afterMarker)
	}
	bodyAt := strings.Index(src, "<body")
	if bodyAt == -1 {
		t.Fatalf("%s has no body tag", name)
	}
	firstScript := bodyAt + strings.Index(src[bodyAt:], "<script")
	if firstScript < bodyAt {
		t.Fatalf("%s has no script tag in the body", name)
	}
	if ifAt > firstScript {
		t.Errorf("%s: the changes pop-up must come before the first script tag of the body", name)
	}

	tag := `<script src="` + c33p21Script + `" defer></script>`
	if n := strings.Count(src, "/static/js/changes-ask.js"); n != 1 {
		t.Errorf("%s holds changes-ask.js %d times, want 1", name, n)
	}
	if n := strings.Count(src, tag); n != 1 {
		t.Errorf("%s: want exactly one %s", name, tag)
	}
	block := `{{if .ChangesAsk}}{{template "changes_ask"}}` + "\n  " + tag + `{{end}}`
	if !strings.Contains(src, block) {
		t.Errorf("%s: the changes-ask.js tag must sit inside the {{if .ChangesAsk}} block, right after the partial: want %q", name, block)
	}
}

// TestChangesAskOnNearbyContract pins the include and the script tag on Nearby.
func TestChangesAskOnNearbyContract(t *testing.T) {
	c33p21Check(t, "index.html.tmpl", `{{template "app_bottombar" .}}`)
}

// TestChangesAskOnLowDataContract pins them on Low data.
func TestChangesAskOnLowDataContract(t *testing.T) {
	c33p21Check(t, "low.html.tmpl", `<div id="toast"`)
}
