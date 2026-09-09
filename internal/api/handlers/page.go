package handlers

import (
	"html/template"
	"log"
	"net/http"

	"pinalert/web"
)

// templateName is the file executed by Page's ExecuteTemplate call.
// template.ParseFS names a parsed template after its base filename.
const templateName = "index.html.tmpl"

// PageConfig carries the values the app shell's view-model needs, kept
// configurable server-side (see cmd/server/main.go) rather than hard-coded
// in JS.
type PageConfig struct {
	FallbackLat     float64
	FallbackLon     float64
	DefaultRadiusKm float64
	// AssetVersion is appended as a "?v=" query string to every local
	// static asset the template links (CSS, JS — not the third-party CDN
	// tags, which are already content-hashed by their pinned version and
	// SRI integrity attribute). //go:embed bakes web/static into the
	// binary at compile time, and the file server sends a 1-hour
	// Cache-Control on top of that (see router.go's staticFileServer) —
	// without a version bump in the URL itself, a browser that already
	// cached the old bytes has no reason to ever re-fetch a changed CSS/JS
	// file after a restart, even a hard reload. cmd/server/main.go sets
	// this once at process start so every restart naturally busts it.
	AssetVersion string
}

// pageViewModel is exactly what index.html.tmpl reads and nothing more.
type pageViewModel struct {
	FallbackLat     float64
	FallbackLon     float64
	DefaultRadiusKm float64
	AssetVersion    string
}

// ParsePageTemplate parses the embedded app shell template from
// pinalert/web's embedded filesystem. html/template's contextual
// auto-escaping is what keeps every server-injected value safe against the
// "template data → rendered HTML" trust boundary — it does not, however,
// cover anything JavaScript later builds from a fetched JSON response (see
// web/static/js/app.js's Pinalert.setText for that boundary).
func ParsePageTemplate() (*template.Template, error) {
	return template.ParseFS(web.TemplatesFS, "templates/*.tmpl")
}

// Page renders the html/template app shell. tmpl must have been parsed via
// ParsePageTemplate (or an equivalent call against the same embedded
// filesystem) so templateName resolves to a defined template.
func Page(tmpl *template.Template, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vm := pageViewModel{
			FallbackLat:     cfg.FallbackLat,
			FallbackLon:     cfg.FallbackLon,
			DefaultRadiusKm: cfg.DefaultRadiusKm,
			AssetVersion:    cfg.AssetVersion,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, templateName, vm); err != nil {
			log.Printf("handlers: Page: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
