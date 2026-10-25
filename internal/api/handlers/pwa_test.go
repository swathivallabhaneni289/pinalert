package handlers

import (
	"bytes"
	"encoding/json"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinalert/internal/appicon"
	"pinalert/web"
)

func newTestPWAAssets(t *testing.T) *PWAAssets {
	t.Helper()
	a, err := NewPWAAssets(web.StaticFS)
	if err != nil {
		t.Fatalf("NewPWAAssets: %v", err)
	}
	return a
}

func pwaGet(t *testing.T, h http.Handler, path string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestManifestIsValid(t *testing.T) {
	a := newTestPWAAssets(t)
	rec := pwaGet(t, Manifest(a), ManifestPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/manifest+json" {
		t.Errorf("Content-Type = %q, want application/manifest+json", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if len(rec.Header().Values("Set-Cookie")) != 0 {
		t.Error("the manifest sets a cookie")
	}

	var m struct {
		Name            string `json:"name"`
		ShortName       string `json:"short_name"`
		Description     string `json:"description"`
		ID              string `json:"id"`
		StartURL        string `json:"start_url"`
		Scope           string `json:"scope"`
		Display         string `json:"display"`
		BackgroundColor string `json:"background_color"`
		ThemeColor      string `json:"theme_color"`
		Icons           []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decoding the manifest: %v", err)
	}
	for field, got := range map[string]string{
		"name": m.Name, "short_name": m.ShortName,
	} {
		if got != "Pinalert" {
			t.Errorf("%s = %q, want Pinalert", field, got)
		}
	}
	for field, got := range map[string]string{
		"id": m.ID, "start_url": m.StartURL, "scope": m.Scope,
	} {
		if got != "/" {
			t.Errorf("%s = %q, want /", field, got)
		}
	}
	if m.Display != "standalone" {
		t.Errorf("display = %q, want standalone", m.Display)
	}
	if m.BackgroundColor != "#1A1D21" || m.ThemeColor != "#1A1D21" {
		t.Errorf("colours = %q and %q, want #1A1D21", m.BackgroundColor, m.ThemeColor)
	}
	if m.Description == "" || strings.ContainsAny(m.Description, string([]rune{0x2013, 0x2014, 0x2d})) {
		t.Errorf("description = %q, want text with no dash", m.Description)
	}

	type want struct{ sizes, typ, purpose string }
	wantIcons := map[string]want{
		"/icons/app-192.png":          {"192x192", "image/png", "any"},
		"/icons/app-512.png":          {"512x512", "image/png", "any"},
		"/icons/app-maskable-512.png": {"512x512", "image/png", "maskable"},
		"/icons/app.svg":              {"any", "image/svg+xml", ""},
	}
	if len(m.Icons) != len(wantIcons) {
		t.Fatalf("manifest lists %d icons, want %d", len(m.Icons), len(wantIcons))
	}
	for _, ic := range m.Icons {
		w, ok := wantIcons[ic.Src]
		if !ok {
			t.Errorf("unexpected icon %q", ic.Src)
			continue
		}
		if ic.Sizes != w.sizes || ic.Type != w.typ || ic.Purpose != w.purpose {
			t.Errorf("icon %s = %+v, want sizes %s type %s purpose %q", ic.Src, ic, w.sizes, w.typ, w.purpose)
		}
		if _, public := a.Handler(ic.Src); !public {
			t.Errorf("manifest icon %s has no public handler", ic.Src)
		}
	}
}

func TestAppIconsAreServedNotStored(t *testing.T) {
	a := newTestPWAAssets(t)
	fresh, err := appicon.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	type spec struct {
		size int
		body []byte
	}
	pngs := map[string]spec{
		"/icons/app-192.png":                {192, fresh.Any192},
		"/icons/app-512.png":                {512, fresh.Any512},
		"/icons/app-maskable-512.png":       {512, fresh.Maskable512},
		"/icons/apple-touch-icon.png":       {180, fresh.Apple180},
		"/apple-touch-icon.png":             {180, fresh.Apple180},
		"/apple-touch-icon-precomposed.png": {180, fresh.Apple180},
		"/icons/badge-96.png":               {96, fresh.Badge96},
	}
	for path, want := range pngs {
		h, ok := a.Handler(path)
		if !ok {
			t.Errorf("%s has no handler", path)
			continue
		}
		rec := pwaGet(t, h, path, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
			continue
		}
		if got := rec.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("%s: Content-Type = %q, want image/png", path, got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=86400" {
			t.Errorf("%s: Cache-Control = %q", path, got)
		}
		if len(rec.Header().Values("Set-Cookie")) != 0 {
			t.Errorf("%s sets a cookie", path)
		}
		img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Errorf("%s: not a PNG: %v", path, err)
			continue
		}
		if b := img.Bounds(); b.Dx() != want.size || b.Dy() != want.size {
			t.Errorf("%s: %dx%d, want %dx%d", path, b.Dx(), b.Dy(), want.size, want.size)
		}
		if !bytes.Equal(rec.Body.Bytes(), want.body) {
			t.Errorf("%s: bytes differ from a fresh appicon render", path)
		}

		// A caller that already holds this version gets a 304 and no body.
		etag := rec.Header().Get("ETag")
		if etag == "" {
			t.Errorf("%s: no ETag", path)
			continue
		}
		again := pwaGet(t, h, path, map[string]string{"If-None-Match": etag})
		if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
			t.Errorf("%s: with the ETag got %d and %d body bytes, want 304 and none", path, again.Code, again.Body.Len())
		}
	}

	svg, ok := a.Handler("/icons/app.svg")
	if !ok {
		t.Fatal("/icons/app.svg has no handler")
	}
	rec := pwaGet(t, svg, "/icons/app.svg", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Errorf("svg: status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != appicon.SVG() {
		t.Error("the served SVG is not appicon.SVG()")
	}

	// The list and the handler lookup agree.
	if len(PWAPublicPaths) != 10 {
		t.Errorf("PWAPublicPaths has %d entries, want 10", len(PWAPublicPaths))
	}
	seen := map[string]bool{}
	for _, p := range PWAPublicPaths {
		if seen[p] {
			t.Errorf("%s is listed twice", p)
		}
		seen[p] = true
		if h, ok := a.Handler(p); !ok || h == nil {
			t.Errorf("%s is on the list but has no handler", p)
		}
	}
	for _, p := range []string{"/icons/other.png", "/sw.js/", "/sw.js.map", "/icons/", "/manifest.webmanifest/", ""} {
		if h, ok := a.Handler(p); ok || h != nil {
			t.Errorf("%q is not on the list but has a handler", p)
		}
	}

	// Nothing is stored: the embedded static tree holds no raster image.
	err = fs.WalkDir(web.StaticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(path)
		for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".bmp", ".avif"} {
			if strings.HasSuffix(lower, ext) {
				t.Errorf("the static tree holds the image file %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the static tree: %v", err)
	}
}

func TestServiceWorkerServedAtRoot(t *testing.T) {
	a := newTestPWAAssets(t)
	rec := pwaGet(t, ServiceWorker(a), "/sw.js", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if got := rec.Header().Get("Service-Worker-Allowed"); got != "" {
		t.Errorf("Service-Worker-Allowed = %q, want none (the root address gives the root scope)", got)
	}
	if len(rec.Header().Values("Set-Cookie")) != 0 {
		t.Error("the worker sets a cookie")
	}
	embedded, err := fs.ReadFile(web.StaticFS, "static/js/sw.js")
	if err != nil {
		t.Fatalf("reading the embedded sw.js: %v", err)
	}
	if !bytes.Equal(rec.Body.Bytes(), embedded) {
		t.Error("the served worker differs from the embedded static/js/sw.js")
	}
}
