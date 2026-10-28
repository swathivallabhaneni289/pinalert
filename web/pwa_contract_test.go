package web

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 3.3 plan 05 (UI-SPEC 5a, 5c, decision 13): the installable app head
// block is one shared partial included once on every full page, and no raster
// image file is ever committed under web/. These tests read only the embedded
// templates and the package directory, so they need no database and no server.

const (
	appHeadPath     = "templates/app_head.html.tmpl"
	appHeadInclude  = `{{template "app_head" .AssetVersion}}`
	appHeadNotifyJS = `<script src="/static/js/notify.js?v={{.}}" defer></script>`
)

// TestEveryPageIncludesAppHead: every embedded template that opens a head
// element carries exactly one include of the app_head partial, inside the
// head, before the theme script. The partial file itself is excluded.
func TestEveryPageIncludesAppHead(t *testing.T) {
	checked := 0
	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") || path == appHeadPath {
			return nil
		}
		raw, readErr := fs.ReadFile(TemplatesFS, path)
		if readErr != nil {
			return readErr
		}
		text := string(raw)
		if !strings.Contains(text, "<head>") {
			return nil
		}
		// Amended 2026-10-29 by plan 05-13 (D-05, research G5 gotcha 6): the Demo
		// page is the one page that must not carry the app head. The partial
		// loads notify.js, which registers the service worker and the phone
		// alert subscription, and the Demo registers nothing and stores nothing.
		// The exemption is this one named file, and the Demo page is pinned to
		// hold no app head and no notify.js by TestDemoPageLoadsNoForbiddenScripts.
		//
		// Amended 2026-10-29 by plan 05-19: the Demo's Official alerts and My
		// activity pages are exempt for the same reason (each is pinned to carry
		// no app head by its own markup test, TestDemoAlertsPageMarkup and
		// TestDemoActivityPageMarkup).
		if path == "templates/demo.html.tmpl" || path == "templates/demo_alerts.html.tmpl" ||
			path == "templates/demo_activity.html.tmpl" {
			return nil
		}
		checked++

		if got := strings.Count(text, appHeadInclude); got != 1 {
			t.Errorf("%s: expected %q exactly once, found %d", path, appHeadInclude, got)
			return nil
		}
		at := strings.Index(text, appHeadInclude)
		headOpen := strings.Index(text, "<head>")
		headClose := strings.Index(text, "</head>")
		if headClose < 0 || at < headOpen || at > headClose {
			t.Errorf("%s: the app_head include must sit inside the head element", path)
		}
		if theme := strings.Index(text, themeScriptSource); theme >= 0 && at > theme {
			t.Errorf("%s: the app_head include must come before the theme script", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded templates: %v", err)
	}
	if checked < 10 {
		t.Fatalf("found only %d templates with a head element, want at least 10: this test's premise may have evaporated", checked)
	}
}

// TestAppHeadContents pins the eight tags of the partial with their exact
// attribute values, in order.
func TestAppHeadContents(t *testing.T) {
	raw, err := fs.ReadFile(TemplatesFS, appHeadPath)
	if err != nil {
		t.Fatalf("reading %s: %v", appHeadPath, err)
	}
	text := string(raw)

	if !strings.Contains(text, `{{define "app_head"}}`) {
		t.Fatalf("%s must define the app_head template", appHeadPath)
	}

	tags := []string{
		`<link rel="manifest" href="/manifest.webmanifest">`,
		`<meta name="theme-color" media="(prefers-color-scheme: light)" content="#FFFFFF">`,
		`<meta name="theme-color" media="(prefers-color-scheme: dark)" content="#0B0D10">`,
		`<link rel="icon" href="/icons/app.svg" type="image/svg+xml">`,
		`<link rel="apple-touch-icon" href="/icons/apple-touch-icon.png">`,
		`<meta name="apple-mobile-web-app-title" content="Pinalert">`,
		`<meta name="mobile-web-app-capable" content="yes">`,
		appHeadNotifyJS,
	}
	last := -1
	for _, tag := range tags {
		if got := strings.Count(text, tag); got != 1 {
			t.Errorf("%s: expected %q exactly once, found %d", appHeadPath, tag, got)
			continue
		}
		at := strings.Index(text, tag)
		if at < last {
			t.Errorf("%s: tag %q is out of order", appHeadPath, tag)
		}
		last = at
	}

	if !strings.Contains(appHeadNotifyJS, " defer>") || !strings.Contains(appHeadNotifyJS, "?v={{.}}") {
		t.Errorf("the notify.js tag must be deferred and carry the asset version")
	}
	if strings.Contains(text, "viewport-fit") {
		t.Errorf("%s: no viewport-fit setting is allowed until the installed app bottom bar inset is verified (UI-SPEC 5a)", appHeadPath)
	}
	if strings.Contains(text, string(rune(0x2013))) || strings.Contains(text, string(rune(0x2014))) {
		t.Errorf("%s: contains U+2013 or U+2014", appHeadPath)
	}
}

// TestNoRasterImagesUnderWeb walks the web package directory and fails on any
// raster image file (T-03.3-19, decision 13, the owner rule that no image
// files live in the repo). SVG mask icons are markup and are allowed; the app
// icons are drawn by internal/appicon at run time, never stored.
func TestNoRasterImagesUnderWeb(t *testing.T) {
	banned := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".bmp", ".avif", ".heic", ".tif"}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		for _, ext := range banned {
			if strings.HasSuffix(name, ext) {
				t.Errorf("%s: raster image files are not allowed under web/ (decision 13)", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking web/: %v", err)
	}
}
