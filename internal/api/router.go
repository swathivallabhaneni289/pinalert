// Package api wires Pinalert's HTTP router: middleware, the anonymous
// session gate, and the /api route group. Route handlers themselves live in
// internal/api/handlers.
package api

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/web"

	_ "pinalert/docs" // swag-generated Swagger 2.0 spec, registered with http-swagger below
)

// Deps holds every dependency a route handler needs. Plan 01-07 extends this
// struct further (swagger mount) without touching this file's shape.
type Deps struct {
	Session  *session.Manager
	Sessions *sqlcgen.Queries
	Reports  *service.ReportService
	Template *template.Template
	Page     handlers.PageConfig
}

// @title        Pinalert API
// @version      1.0
// @description  Anonymous, crowd-reported local emergency feed for floods, cyclones, and other
// @description  disasters. Reports are posted by nearby people and confirmed or disputed by
// @description  other nearby people; this API is the read/write surface for that data. Pinalert
// @description  is an unofficial, unaffiliated project and is not a substitute for contacting
// @description  emergency services. This document is Swagger 2.0 — swag (the generator behind
// @description  this spec) does not emit OpenAPI 3.x, so tooling that expects OpenAPI 3.x
// @description  specifically should account for that; OPS-01's "OpenAPI/Swagger spec" requirement
// @description  is satisfied by either format.
// @BasePath     /api
// @license.name Unlicensed (portfolio project, all rights reserved)
// @contact.name Pinalert project
// @contact.url  https://github.com/swathivallabhaneni289/pinalert

// NewRouter builds the chi router: request-id/real-ip/recoverer/logger
// middleware, then the session middleware (issuing or verifying the
// pinalert_session cookie on every request), then the /api route group.
func NewRouter(deps Deps) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(deps.Session.Middleware(deps.persistSession))

	r.Get("/", handlers.Page(deps.Template, deps.Page))
	r.Handle("/static/*", http.StripPrefix("/static/", staticFileServer()))

	// Mounted outside the /api group so its middleware stack stays
	// independent, and left publicly reachable: this is a public read-only
	// API with no privileged operations to hide, and OPS-01 asks for a
	// stable, browsable URL. /swagger/index.html is that stable URL. The v2
	// handler serves its UI assets from the binary — no third-party CDN
	// request at view time.
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	r.Route("/api", func(r chi.Router) {
		r.Post("/reports", handlers.SubmitReport(deps.Reports))
		r.Get("/reports", handlers.NearbyReports(deps.Reports))
	})

	return r
}

// staticFileServer serves web/static's embedded files with a conservative
// cache header. Static assets are rebuilt into a new binary on every
// deploy, so a one-hour cache is safe and costs nothing to invalidate.
func staticFileServer() http.Handler {
	sub, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		// Only fails if the //go:embed directive itself is wrong — a
		// build-time programming error, not a runtime condition a caller
		// could meaningfully recover from.
		panic("api: embedding web/static: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fileServer.ServeHTTP(w, r)
	})
}

// persistSession is the session Manager's persist callback: it writes the
// sessions row eagerly on first visit, per FOUND-01 ("issued to a
// first-time visitor") and Open Question 1's recommendation, so Phase 2's
// reputation work can read a session's full history rather than only its
// submissions.
func (d Deps) persistSession(ctx context.Context, id string) error {
	return d.Sessions.UpsertSession(ctx, id)
}
