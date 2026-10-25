// This file serves the pieces that make the site installable (Phase 3.3, plan
// 04): the web app manifest, the service worker and the app icons. They are
// public on purpose. A browser fetches the manifest and the icons without the
// session cookie, so an install would fail on a redirect to /login, and an
// update check of the worker must not be sent through a redirect either.
// Nothing here reads or writes an account, a session or the database: the
// bytes are the same for everyone.
//
// The router registers these routes and builds its session bypass from the one
// list, PWAPublicPaths, so the two cannot drift apart.

package handlers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"pinalert/internal/appicon"
)

// ManifestPath is where the page head links the web app manifest.
const ManifestPath = "/manifest.webmanifest"

// The addresses of the service worker and the icons. The icons are drawn by
// package appicon at start up and served from memory; none is a stored file.
const (
	serviceWorkerPath = "/sw.js"
	iconSVGPath       = "/icons/app.svg"
	icon192Path       = "/icons/app-192.png"
	icon512Path       = "/icons/app-512.png"
	iconMaskablePath  = "/icons/app-maskable-512.png"
	iconAppleTouch    = "/icons/apple-touch-icon.png"
	iconBadgePath     = "/icons/badge-96.png"
	rootAppleTouch    = "/apple-touch-icon.png"
	rootApplePrecomp  = "/apple-touch-icon-precomposed.png"
)

// PWAPublicPaths is the one list of the ten addresses that are reachable
// without a session and without the login gate. It is the only place in the
// repo that spells them as Go strings outside the tests. The router registers
// one exact GET route for each entry and skips the session middleware for
// exactly these paths, so a request for anything else, a near miss included,
// is treated like every other route.
var PWAPublicPaths = []string{
	ManifestPath,
	serviceWorkerPath,
	iconSVGPath,
	icon192Path,
	icon512Path,
	iconMaskablePath,
	iconAppleTouch,
	iconBadgePath,
	rootAppleTouch,
	rootApplePrecomp,
}

// Cache headers. The manifest and the worker are checked on every use, so a
// changed worker is found at once. The icons are the same for everyone and
// carry an ETag.
const (
	pwaNoCache    = "no-cache"
	pwaIconCache  = "public, max-age=86400"
	pwaManifestMT = "application/manifest+json"
)

// manifestDescription is the welcome tagline.
const manifestDescription = "Live emergency updates from people nearby, confirmed by others who are there."

// manifestColour is the tile colour: the launch screen shows the white mark on
// it, and the per page theme colour tags override it while the app runs.
const manifestColour = "#1A1D21"

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

type manifestDoc struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	ID              string         `json:"id"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Icons           []manifestIcon `json:"icons"`
}

// PWAAssets holds the bytes behind the ten public addresses, built once.
type PWAAssets struct {
	manifest     []byte
	manifestETag string
	sw           []byte
	icons        *appicon.Set
	svg          []byte
	svgETag      string
}

// NewPWAAssets builds the manifest, reads the service worker from the embedded
// static tree (static is the embedded FS, rooted so that the script sits at
// static/js/sw.js) and draws the icons. It fails when the worker is missing or
// an icon cannot be drawn, which can only be a build defect.
func NewPWAAssets(static fs.FS) (*PWAAssets, error) {
	sw, err := fs.ReadFile(static, "static/js/sw.js")
	if err != nil {
		return nil, fmt.Errorf("pwa: reading the service worker: %w", err)
	}
	icons, err := appicon.Render()
	if err != nil {
		return nil, fmt.Errorf("pwa: drawing the app icons: %w", err)
	}
	manifest, err := json.Marshal(manifestDoc{
		Name:            "Pinalert",
		ShortName:       "Pinalert",
		Description:     manifestDescription,
		ID:              "/",
		StartURL:        "/",
		Scope:           "/",
		Display:         "standalone",
		BackgroundColor: manifestColour,
		ThemeColor:      manifestColour,
		Icons: []manifestIcon{
			{Src: icon192Path, Sizes: "192x192", Type: "image/png", Purpose: "any"},
			{Src: icon512Path, Sizes: "512x512", Type: "image/png", Purpose: "any"},
			{Src: iconMaskablePath, Sizes: "512x512", Type: "image/png", Purpose: "maskable"},
			{Src: iconSVGPath, Sizes: "any", Type: "image/svg+xml"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("pwa: encoding the manifest: %w", err)
	}
	svg := []byte(appicon.SVG())
	return &PWAAssets{
		manifest:     manifest,
		manifestETag: etagFor(manifest),
		sw:           sw,
		icons:        icons,
		svg:          svg,
		svgETag:      etagFor(svg),
	}, nil
}

// etagFor reuses the icon ETag format for the byte slices this file builds.
func etagFor(b []byte) string {
	return appicon.ETag(b)
}

// serveBytes writes one in memory body with its type, cache header and ETag,
// answering 304 when the caller already holds that version. It sets no cookie.
func serveBytes(w http.ResponseWriter, r *http.Request, body []byte, contentType, cacheControl, etag string) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", cacheControl)
	h.Set("X-Content-Type-Options", "nosniff")
	if etag != "" {
		h.Set("ETag", etag)
		if inm := r.Header.Get("If-None-Match"); inm != "" && etagMatches(inm, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// etagMatches reports whether an If-None-Match header names the ETag (or is the
// wildcard).
func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "W/")
		if part == "*" || part == etag {
			return true
		}
	}
	return false
}

// Manifest serves the web app manifest.
func Manifest(a *PWAAssets) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveBytes(w, r, a.manifest, pwaManifestMT, pwaNoCache, a.manifestETag)
	})
}

// ServiceWorker serves the worker from the root address, which gives it the
// root scope with no Service-Worker-Allowed header. It is never cached, so a
// changed worker is found on the next update check.
func ServiceWorker(a *PWAAssets) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveBytes(w, r, a.sw, "text/javascript; charset=utf-8", pwaNoCache, "")
	})
}

// The names AppIcon accepts: the file name of the icon under /icons.
const (
	IconApp192   = "app-192.png"
	IconApp512   = "app-512.png"
	IconMaskable = "app-maskable-512.png"
	IconApple    = "apple-touch-icon.png"
	IconBadge    = "badge-96.png"
)

// AppIcon serves one PNG icon by file name (one of the Icon constants). An
// unknown name gets a 404 handler.
func AppIcon(a *PWAAssets, name string) http.Handler {
	var body []byte
	var etag string
	switch name {
	case IconApp192:
		body, etag = a.icons.Any192, a.icons.Any192ETag
	case IconApp512:
		body, etag = a.icons.Any512, a.icons.Any512ETag
	case IconMaskable:
		body, etag = a.icons.Maskable512, a.icons.Maskable512ETag
	case IconApple:
		body, etag = a.icons.Apple180, a.icons.Apple180ETag
	case IconBadge:
		body, etag = a.icons.Badge96, a.icons.Badge96ETag
	default:
		return http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveBytes(w, r, body, "image/png", pwaIconCache, etag)
	})
}

// AppIconSVG serves the app mark as SVG.
func AppIconSVG(a *PWAAssets) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveBytes(w, r, a.svg, "image/svg+xml", pwaIconCache, a.svgETag)
	})
}

// Handler returns the handler for one entry of PWAPublicPaths, and false for
// any other path. The two root apple touch addresses serve the same 180 icon as
// the one under /icons, because iOS asks for them by itself.
func (a *PWAAssets) Handler(path string) (http.Handler, bool) {
	switch path {
	case ManifestPath:
		return Manifest(a), true
	case serviceWorkerPath:
		return ServiceWorker(a), true
	case iconSVGPath:
		return AppIconSVG(a), true
	case icon192Path:
		return AppIcon(a, IconApp192), true
	case icon512Path:
		return AppIcon(a, IconApp512), true
	case iconMaskablePath:
		return AppIcon(a, IconMaskable), true
	case iconAppleTouch, rootAppleTouch, rootApplePrecomp:
		return AppIcon(a, IconApple), true
	case iconBadgePath:
		return AppIcon(a, IconBadge), true
	}
	return nil, false
}
