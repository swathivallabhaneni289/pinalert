// Package api wires Pinalert's HTTP router: middleware, the anonymous
// session cookie, the verified-account access gate (gate.go), and the route
// table. Route handlers themselves live in internal/api/handlers.
package api

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"pinalert/internal/api/handlers"
	"pinalert/internal/ratelimit"
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
	// Votes backs the four vote routes registered below. Additive exactly
	// as Dev and RequestLinkRateLimit were — a Deps{} literal that omits
	// it (such as swagger_test.go's) still compiles and still serves
	// every route that does not touch voting.
	Votes *service.VotingService

	AuthService *service.AuthService
	Auth        handlers.AuthConfig
	Template    *template.Template
	Page        handlers.PageConfig
	// Geocode backs GET /api/geocode (plan 07-01/07-03). Typed as the
	// handlers.GeocodeSearcher interface, not *geocode.Client, so this file
	// needs no import of internal/geocode and a test can substitute a stub.
	// Additive exactly as Votes and RequestLinkRateLimit were — a Deps{}
	// literal that omits it (such as swagger_test.go's) still compiles and
	// still serves every route that does not touch geocoding, and a
	// request to GET /api/geocode against a nil value degrades to the D-04
	// "search unavailable" message rather than panicking (the handler
	// guards it).
	Geocode handlers.GeocodeSearcher
	// GeocodeRateLimit configures the per-IP token-bucket limiter wrapped
	// around GET /api/geocode only. Left at its zero value,
	// GeocodeRateLimitDefault below is used, matching RequestLinkRateLimit's
	// contract — cmd/server/main.go is the one place the numbers are
	// visible rather than buried as literals in this router.
	GeocodeRateLimit GeocodeRateLimit
	// Dev disables the static asset cache in staticFileServer. Left false
	// (the zero value) selects the production 1-hour cache automatically —
	// every existing Deps{} literal that doesn't set this field keeps
	// today's behavior unchanged. See staticFileServer's doc comment for
	// why a long-lived cache is actively wrong during local dev iteration
	// (found live during 01-15's UAT correction round: PageConfig.AssetVersion
	// busts the CSS/JS *linking* HTML, but the icon SVGs those stylesheets
	// reference via `mask-image: url(...)` are never templated and so were
	// never covered by that fix — this field is the other half of it).
	Dev bool
	// RequestLinkRateLimit configures the per-IP token-bucket limiter plan
	// 01.1-05 wraps around POST /api/auth/request-link only. Left at its
	// zero value, RequestLinkRateLimitDefault below is used — every
	// existing Deps{} literal that predates this field keeps working
	// unchanged, and cmd/server/main.go is the one place the DEC-I numbers
	// (burst 5, one token per 60s) are visible rather than buried as
	// literals in this router.
	RequestLinkRateLimit RequestLinkRateLimit
}

// RequestLinkRateLimit is the burst/refill configuration for the per-IP
// limiter wrapping POST /api/auth/request-link (DEC-I).
type RequestLinkRateLimit struct {
	Burst int
	Every time.Duration
}

// RequestLinkRateLimitDefault is DEC-I's chosen budget — burst 5, one token
// refilled every 60 seconds — used whenever Deps.RequestLinkRateLimit is
// left at its zero value.
var RequestLinkRateLimitDefault = RequestLinkRateLimit{Burst: 5, Every: 60 * time.Second}

// GeocodeRateLimit is the burst/refill configuration for the per-IP limiter
// wrapping GET /api/geocode only.
type GeocodeRateLimit struct {
	Burst int
	Every time.Duration
}

// GeocodeRateLimitDefault is this plan's chosen budget — burst 3, one token
// refilled every 2 seconds — used whenever Deps.GeocodeRateLimit is left at
// its zero value. This is a fresh named budget, not a reuse of
// RequestLinkRateLimitDefault (burst 5, one token per 60s): that budget was
// tuned for "request a login link" traffic, which is rare and deliberate,
// whereas typing into a search box is bursty within one report submission
// and rare across a session, so it needs its own shape (07-RESEARCH.md Open
// Question 2). This per-IP limiter is the secondary, anti-abuse control
// only — the actual Nominatim policy ceiling (D-02) is the process-wide
// limiter inside internal/geocode, because a per-IP limiter cannot bound
// total outbound traffic across many clients.
var GeocodeRateLimitDefault = GeocodeRateLimit{Burst: 3, Every: 2 * time.Second}

// @title        Pinalert API
// @version      1.0
// @description  Crowd-reported local emergency feed for floods, cyclones, and other disasters.
// @description  Reports are posted by nearby people and confirmed or disputed by other nearby
// @description  people; this API is the read/write surface for that data. Reading and writing
// @description  reports both require a session verified by email through the magic-link flow —
// @description  an unverified caller is refused with 401. Pinalert is an unofficial,
// @description  unaffiliated project and is not a substitute for contacting emergency services.
// @description  This document is Swagger 2.0 — swag (the generator behind this spec) does not
// @description  emit OpenAPI 3.x, so tooling that expects OpenAPI 3.x specifically should account
// @description  for that; OPS-01's "OpenAPI/Swagger spec" requirement is satisfied by either
// @description  format.
// @BasePath     /api
// @license.name Unlicensed (portfolio project, all rights reserved)
// @contact.name Pinalert project
// @contact.url  https://github.com/swathivallabhaneni289/pinalert

// NewRouter builds the chi router: request-id/client-ip-resolution/
// recoverer/logger middleware, then the session middleware (issuing or verifying the
// pinalert_session cookie on every request), then the route table. D-05
// ("login is required to view anything, not merely to act") is enforced by
// splitting the table into two parts: a short, explicitly-documented set of
// routes reachable without a verified account, and a single r.Group — every
// other route — wrapped in the gate.go access-gate middleware. A route added
// to the group is protected by construction; a route added outside it is
// visibly, by construction, unprotected (threat T-01-70).
//
// The exempt set is exactly: /static/*, /swagger/*, GET /login, POST
// /api/auth/request-link, and GET /auth/verify — the login surface itself,
// the two request/verify steps that make a session verified in the first
// place, static assets the login page needs to render, and the published
// API reference (DEC-G, 01.1-04-PLAN.md; the /swagger/* exemption's tension
// with D-05 is recorded there as accepted risk AR-32).
func NewRouter(deps Deps) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// The client IP is resolved from the TCP peer address because nothing in
	// front of this service is confirmed to overwrite inbound proxy headers
	// (DEC-U). If this app is ever deployed behind a confirmed single
	// trusted reverse proxy, middleware.ClientIPFromXFFTrustedProxies
	// configured with that proxy's real hop count is the correct upgrade.
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(deps.Session.Middleware(deps.persistSession))

	// --- Exempt: reachable without a verified account. ---

	r.Get("/login", handlers.LoginGate(deps.Template, deps.Auth))
	// GET only, deliberately: chi does not auto-map HEAD onto a GET-only
	// handler the way net/http.ServeMux does, and registering only GET here
	// cheaply sidesteps one class of mail-security-scanner prefetch
	// consuming a visitor's single-use link before they click it
	// (threat T-01-62). Left outside the gate — it is how a session becomes
	// verified in the first place.
	r.Get("/auth/verify", handlers.Verify(deps.AuthService, deps.Template, deps.Auth))
	r.Handle("/static/*", http.StripPrefix("/static/", staticFileServer(deps.Dev)))
	// Left publicly reachable: this is a public read-only API with no
	// privileged operations to hide, and OPS-01 asks for a stable,
	// browsable URL. /swagger/index.html is that stable URL. The v2 handler
	// serves its UI assets from the binary — no third-party CDN request at
	// view time.
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	// Registered as a literal path (not nested under an r.Route("/api", ...)
	// mount) so it can stay outside the gated group below while /api/reports
	// stays inside it — chi does not allow a literal sibling path alongside
	// a wildcard r.Route mount at the same prefix, so /api is never mounted
	// as its own subrouter in this router; every /api/* path is registered
	// flat, gated or not, exactly as this one is. Deliberately reachable
	// without verification — this is how a visitor becomes verified.
	//
	// The per-IP limiter (DEC-I: burst 5, one token per 60s) is scoped with
	// r.With(...) to this single route only — never applied via a top-level
	// r.Use — so it throttles POST /api/auth/request-link exclusively.
	// Gating the whole router would throttle static assets and the login
	// page itself during exactly the burst a real emergency produces
	// (T-01-79's CGNAT concern compounds this: a shared carrier IP hitting
	// a router-wide limit would lock out every other route too).
	requestLinkLimit := deps.RequestLinkRateLimit
	if requestLinkLimit == (RequestLinkRateLimit{}) {
		requestLinkLimit = RequestLinkRateLimitDefault
	}
	requestLinkLimiter := ratelimit.NewPerIP(requestLinkLimit.Every, requestLinkLimit.Burst)
	r.With(requestLinkLimiter.Middleware()).Post("/api/auth/request-link", handlers.RequestLink(deps.AuthService))

	// --- Gated: requires a verified account (D-05). ---

	r.Group(func(r chi.Router) {
		r.Use(requireVerifiedAccount(deps.Sessions))
		r.Get("/", handlers.Page(deps.Template, deps.Page))
		r.Post("/api/reports", handlers.SubmitReport(deps.Reports))
		r.Get("/api/reports", handlers.NearbyReports(deps.Reports))
		// The four vote routes (02-03b). All four come from ONE
		// handlers.CastVote factory so the JSON contract and error mapping
		// exist exactly once. Content votes (confirm/dispute) and
		// resolution votes (resolve/reopen) are distinguished by the kind
		// argument rather than by four separate handlers, keeping "is this
		// report still true" and "is this resolved" as two separately
		// tracked signals (D-02, D-16). They are registered flat — as four
		// separate r.Post calls on full literal paths, NOT via
		// r.Route("/api/reports/{id}", ...) as 02-PATTERNS.md sketches —
		// because this router already has a literal /api/reports leaf that
		// a wildcard subrouter mounted at an overlapping prefix would sit
		// awkwardly beside; every /api/* path in this router is registered
		// flat, gated or not (see the /api/auth/request-link comment
		// above). Flat registration also keeps these four routes in the
		// same visual list as every other route in the group, which is
		// exactly the property this function's own doc comment relies on
		// for T-01-70: you can see at a glance that they are inside the
		// gate. They are inside the gate at all because a vote must be
		// attributable to a verified account, which is Phase 1.1's entire
		// reason for existing.
		r.Post("/api/reports/{id}/confirm", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteConfirm))
		r.Post("/api/reports/{id}/dispute", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteDispute))
		r.Post("/api/reports/{id}/resolve", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteResolve))
		r.Post("/api/reports/{id}/reopen", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteReopen))
		// GET /api/geocode (plan 07-03). Registered as a flat literal path,
		// not under an r.Route mount — see this router's own doc comments
		// above for why every /api/* path here is flat, gated or not.
		// Gated at all because geocoding is only reachable from the report
		// submission modal, which is already behind login (D-05); leaving
		// it ungated would hand an unauthenticated caller a free proxy onto
		// this app's shared Nominatim budget.
		geocodeLimit := deps.GeocodeRateLimit
		if geocodeLimit == (GeocodeRateLimit{}) {
			geocodeLimit = GeocodeRateLimitDefault
		}
		geocodeLimiter := ratelimit.NewPerIP(geocodeLimit.Every, geocodeLimit.Burst)
		r.With(geocodeLimiter.Middleware()).Get("/api/geocode", handlers.Geocode(deps.Geocode))
		// deps.Template — the same ParsePageTemplate result / handlers.Page
		// gets, not a separately-parsed single file — is load-bearing:
		// profile.html.tmpl includes 01.1-06's account_header.html.tmpl
		// partial via {{template}}, which html/template only resolves
		// within the template set it was parsed into (01.1-07-PLAN.md).
		r.Get("/profile", handlers.Profile(deps.AuthService, deps.Template, deps.Auth))
		// D-09's explicit log-out action; only POST is ever registered for
		// this path (DEC-L: SameSite=Lax withholds the session cookie from
		// a cross-site POST, so a forged logout carries no authenticated
		// session — the same property would not hold for GET).
		r.Post("/auth/logout", handlers.Logout(deps.Session))
	})

	return r
}

// staticFileServer serves web/static's embedded files. In production
// (dev=false) a one-hour cache is safe and costs nothing to invalidate,
// because static assets are rebuilt into a new binary on every deploy —
// there is no long-running process whose embedded bytes go stale under a
// caller's feet. That assumption is backwards during local dev iteration:
// `go run` recompiles and restarts the SAME long-lived process repeatedly
// against a browser that never navigates away, so an hour-long cache is an
// hour spent unable to see a change land. Found live during 01-15's UAT
// correction round — PageConfig.AssetVersion busts the cache for the CSS/JS
// files a browser reaches via the templated HTML's own <link>/<script>
// tags, but the icon SVGs those stylesheets reference via CSS
// `mask-image: url(...)` are never templated, so they were still exposed
// to a full hour of stale caching after every restart. dev=true disables
// caching entirely instead of also trying to version those URLs, since
// nothing here is served to a real browser audience in dev mode anyway.
func staticFileServer(dev bool) http.Handler {
	sub, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		// Only fails if the //go:embed directive itself is wrong — a
		// build-time programming error, not a runtime condition a caller
		// could meaningfully recover from.
		panic("api: embedding web/static: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	cacheControl := "public, max-age=3600"
	if dev {
		cacheControl = "no-store"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl)
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
