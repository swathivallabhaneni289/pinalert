// Package web embeds Pinalert's server-rendered template and static asset
// trees so the compiled binary is self-contained and runs from any working
// directory. Go's //go:embed directive cannot reach outside the embedding
// package's own directory, so internal/api/handlers (which needs the
// templates) and internal/api (which needs the static files) cannot embed
// ../../web directly — this thin package, mirroring the same pattern
// internal/store/migrations.go already uses for goose migrations, is the
// load-bearing bridge every consumer imports instead.
package web

import "embed"

// TemplatesFS embeds every html/template file under web/templates.
//
//go:embed templates/*.tmpl
var TemplatesFS embed.FS

// StaticFS embeds the entire web/static tree (CSS, JS, icons) rooted at
// "static/...". Callers that need a filesystem rooted at the static
// directory itself (for http.FileServer) should take fs.Sub(StaticFS,
// "static") rather than serving StaticFS directly.
//
//go:embed static
var StaticFS embed.FS
