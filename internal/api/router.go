// Package api wires Pinalert's HTTP router: middleware, the anonymous
// session gate, and the /api route group. Route handlers themselves live in
// internal/api/handlers.
package api

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
)

// Deps holds every dependency a route handler needs. Plans 01-04 and 01-07
// extend this struct further (page templates, swagger mount) without
// touching this file's shape.
type Deps struct {
	Session  *session.Manager
	Sessions *sqlcgen.Queries
	Reports  *service.ReportService
}

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

	r.Route("/api", func(r chi.Router) {
		r.Post("/reports", handlers.SubmitReport(deps.Reports))
		r.Get("/reports", handlers.NearbyReports(deps.Reports))
	})

	return r
}

// persistSession is the session Manager's persist callback: it writes the
// sessions row eagerly on first visit, per FOUND-01 ("issued to a
// first-time visitor") and Open Question 1's recommendation, so Phase 2's
// reputation work can read a session's full history rather than only its
// submissions.
func (d Deps) persistSession(ctx context.Context, id string) error {
	return d.Sessions.UpsertSession(ctx, id)
}
