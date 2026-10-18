// Package api wires Pinalert's HTTP router: middleware, the anonymous
// session cookie, the verified-account access gate (gate.go), and the route
// table. Route handlers themselves live in internal/api/handlers.
package api

import (
	"context"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
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
	// GeocodeRateLimit configures the per account budget of GET /api/geocode
	// only (quick 261016-sne; it was a per IP limiter before). Left at its
	// zero value, GeocodeRateLimitDefault below is used, matching
	// RequestLinkRateLimit's contract: cmd/server/main.go is the one place the
	// numbers are visible rather than buried as literals in this router.
	GeocodeRateLimit GeocodeRateLimit
	// PlaceSearch backs GET /api/geocode/place, the reader's place search on
	// the history views (HIST-05, D-14). It is typed as the
	// handlers.PlaceSearcher interface for the same reasons Geocode is. It
	// must be backed by the SAME *geocode.Client as Geocode, so the process
	// wide limiter stays process wide. A caller with geocoding switched off
	// must leave it as an untyped nil: a typed nil *geocode.Client stored in
	// the interface would not match the handler's nil check.
	PlaceSearch handlers.PlaceSearcher
	// PlaceRateLimit configures the per account budget of GET
	// /api/geocode/place. Left at its zero value, PlaceRateLimitDefault below
	// is used.
	PlaceRateLimit PlaceRateLimit
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
	// Chrome is the shared data for the unofficial bar (D-21, D-22, D-24),
	// built once by cmd/server/main.go from the environment. Additive, as the
	// fields above were: a zero value keeps every Deps{} literal compiling and
	// renders the default numbers.
	Chrome handlers.Chrome
	// RequestLog receives the one line written per request (see
	// requestLogger). Left nil, a logger on standard output with the usual
	// date and time prefix is used, which is where chi's own logger wrote.
	RequestLog *log.Logger
	// RateLimits holds the six D-11 account shapes and the three network
	// backstop shapes. A zero field takes its
	// RateLimitsDefault value; cmd/server/main.go sets every field from its
	// constants block.
	RateLimits RateLimits
	// ClientIPHeader names the header the host overwrites with the visitor
	// address (CF-Connecting-IP on Render, Fly-Client-IP on Fly.io, X-Real-IP on
	// Railway, confirmed by a canary request first). Empty keeps the TCP peer
	// address for every per IP limiter and turns the network backstop off.
	ClientIPHeader string
	// LimiterClock is the clock of the Phase 4 rate limit buckets. Nil means
	// time.Now; tests inject a clock.
	LimiterClock func() time.Time
	// ModeratorEmails is the set of normalised addresses that may open the
	// moderator screen (D-08). It is filled by cmd/server/main.go from
	// MODERATOR_EMAILS through service.ParseModeratorEmails. Nil, which is
	// what every Deps{} literal and test harness that does not set it
	// gives, means nobody is a moderator and every moderator route answers
	// 404.
	ModeratorEmails map[string]struct{}
	// ModerationTx runs the moderator Keep and Remove writes inside one
	// transaction that locks the check row first, so a Keep and the
	// background worker cannot deadlock (WR-S01). cmd/server/main.go sets it
	// from the pool through service.PgxModerationTx. Nil, which every Deps{}
	// literal that does not set it gives, runs the same writes without a
	// transaction.
	ModerationTx service.ModerationTxFunc
	// AccountTx runs Delete my account (POST /api/me/delete) inside one
	// transaction, so the scrub, the deletes and the session revocation commit
	// together or not at all. cmd/server/main.go sets it from the pool through
	// service.PgxAccountTx. Additive: nil, which every Deps{} literal that does
	// not set it gives, runs the same writes without a transaction.
	AccountTx service.AccountTxFunc
	// OnboardingGate turns on the first-time flow redirect: an HTML GET from an
	// account that has not agreed to the current terms version or not finished
	// the flow goes to /welcome (D-27). Additive, as the fields above were: a
	// zero value keeps every Deps{} literal compiling and every existing
	// harness unchanged; cmd/server/main.go turns it on once the /welcome page
	// exists (plan 10).
	OnboardingGate bool
}

// requestLogger returns the access log middleware. It replaces chi's
// middleware.Logger, which prints r.RequestURI and so wrote every reader's
// search text and map centre (the query string of /api/geocode,
// /api/geocode/place and the /api/reports poll) into a log the host stores.
// Pinalert stores no reader location, so the line holds only the request id,
// the method, the path without its query string, the status, the size and the
// duration. Nothing from the query, the body, the cookies or the headers is
// read. The path is written in its escaped form so a crafted path cannot
// break the line in two.
func requestLogger(l *log.Logger) func(http.Handler) http.Handler {
	if l == nil {
		l = log.New(os.Stdout, "", log.LstdFlags)
	}
	return middleware.RequestLogger(&pathOnlyLogFormatter{logger: l})
}

// pathOnlyLogFormatter is the middleware.LogFormatter behind requestLogger.
type pathOnlyLogFormatter struct {
	logger *log.Logger
}

func (f *pathOnlyLogFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &pathOnlyLogEntry{
		logger: f.logger,
		reqID:  middleware.GetReqID(r.Context()),
		method: r.Method,
		path:   r.URL.EscapedPath(),
	}
}

type pathOnlyLogEntry struct {
	logger *log.Logger
	reqID  string
	method string
	path   string
}

func (e *pathOnlyLogEntry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	if status == 0 {
		status = http.StatusOK
	}
	e.logger.Printf("[%s] %s %s %d %dB in %s", e.reqID, e.method, e.path, status, bytes, elapsed)
}

// Panic is only reached if a recoverer sits inside this middleware. The
// Recoverer in NewRouter sits outside it and prints its own report, so this
// stays a one line note with no request data beyond the method and path.
func (e *pathOnlyLogEntry) Panic(v interface{}, _ []byte) {
	e.logger.Printf("[%s] %s %s panic: %v", e.reqID, e.method, e.path, v)
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

// GeocodeRateLimit is the burst/refill configuration for the per account
// budget of GET /api/geocode only.
type GeocodeRateLimit struct {
	Burst int
	Every time.Duration
}

// GeocodeRateLimitDefault and PlaceRateLimitDefault are the budgets of the two
// searches, one person each, used whenever Deps leaves them at their zero
// value (quick 261016-sne). Both are 8 searches at once, then 1 a second.
//
// Why these numbers. Both searches now run after a 300 ms pause in typing
// (quick 261016-sfu), so one person typing one address sends a search at
// most about every 300 ms while they type in short bursts. A burst of 8 covers
// one address typed with several pauses, and a refill of 1 a second keeps up
// with steady typing a little behind, which the browser's quiet retry of the
// latest text (report-form.js, map-tools.js) absorbs without showing an
// error. The old numbers (3, then 1 every 2 s for the report search; 3, then
// 1 every 4 s for the place search) ran out within one address.
//
// Why per account. Both are keyed by the verified account, never the client
// address: carrier NAT puts many phones behind one address, and on a host
// behind a proxy without CLIENT_IP_HEADER every visitor may share one, so a
// per IP budget would make them share these 8 searches (D-16).
//
// These budgets are anti abuse controls for one person only. The real ceilings
// on Photon and Nominatim are the process wide limiters inside
// internal/geocode, which bound total outbound traffic across every person and
// are unchanged; a search past them is answered busy or waits, never sent.
var GeocodeRateLimitDefault = GeocodeRateLimit{Burst: 8, Every: time.Second}

// PlaceRateLimit is the burst/refill configuration of the per account budget
// on GET /api/geocode/place.
type PlaceRateLimit struct {
	Burst int
	Every time.Duration
}

// PlaceRateLimitDefault is the reader lane's budget, the same as the report
// search's (see GeocodeRateLimitDefault for the numbers and the reason). The
// reader lane also never queues ahead of a report search inside
// internal/geocode, so a larger budget here costs a reporter nothing.
var PlaceRateLimitDefault = PlaceRateLimit{Burst: 8, Every: time.Second}

// BucketLimit and RateLimits are aliases of the handlers types, the same types
// under a second name. Package api imports handlers, so the gate in handlers
// cannot take a type declared here; the struct bodies live beside the gate and
// callers write api.BucketLimit and api.RateLimits.
type BucketLimit = handlers.BucketLimit
type RateLimits = handlers.RateLimits

// RateLimitsDefault mirrors the D-11 constants block in cmd/server/main.go, the
// one named place. It is used for any field a Deps literal leaves zero, the
// RequestLinkRateLimitDefault precedent.
var RateLimitsDefault = RateLimits{
	ReportSerious: BucketLimit{Every: 5 * time.Minute, Burst: 5},
	ReportOther:   BucketLimit{Every: 15 * time.Minute, Burst: 8},
	Vote:          BucketLimit{Every: 15 * time.Second, Burst: 20},
	VoteChange:    BucketLimit{Every: 30 * time.Second, Burst: 1},
	Flag:          BucketLimit{Every: 2 * time.Minute, Burst: 10},
	SubmitAttempt: BucketLimit{Every: 6 * time.Second, Burst: 20},
	// The network backstop, one bucket per action family (WR-S02). The
	// reasoning for the numbers sits beside the constants in cmd/server/main.go.
	IPBackstopReport: BucketLimit{Every: time.Second, Burst: 2000},
	IPBackstopVote:   BucketLimit{Every: 150 * time.Millisecond, Burst: 2000},
	IPBackstopFlag:   BucketLimit{Every: 1200 * time.Millisecond, Burst: 1000},
}

// resolveRateLimits replaces every field equal to BucketLimit{} with the
// RateLimitsDefault field of the same name, so the gate receives fully resolved
// limits and needs no default logic of its own.
func resolveRateLimits(l RateLimits) RateLimits {
	fill := func(got *BucketLimit, def BucketLimit) {
		if *got == (BucketLimit{}) {
			*got = def
		}
	}
	fill(&l.ReportSerious, RateLimitsDefault.ReportSerious)
	fill(&l.ReportOther, RateLimitsDefault.ReportOther)
	fill(&l.Vote, RateLimitsDefault.Vote)
	fill(&l.VoteChange, RateLimitsDefault.VoteChange)
	fill(&l.Flag, RateLimitsDefault.Flag)
	fill(&l.SubmitAttempt, RateLimitsDefault.SubmitAttempt)
	fill(&l.IPBackstopReport, RateLimitsDefault.IPBackstopReport)
	fill(&l.IPBackstopVote, RateLimitsDefault.IPBackstopVote)
	fill(&l.IPBackstopFlag, RateLimitsDefault.IPBackstopFlag)
	return l
}

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

// compressionLevel is the gzip level of the response compression middleware.
const compressionLevel = 5

// compressibleContentTypes lists the response types NewRouter compresses (D-14,
// RESEARCH "Size budget"). The router used to send everything uncompressed. The
// list is chi's default minus text/plain and the formats this app never serves
// (markdown, csv, vtt, xml, atom, rss, x-javascript). SVG stays because the
// shipped icons are SVG mask files. text/plain is left out on purpose, so error
// bodies such as the one http.NotFound writes stay byte for byte unchanged. The
// Low data byte budget is still measured in raw bytes (04-27).
var compressibleContentTypes = []string{"text/html", "text/css", "text/javascript", "application/javascript", "application/json", "image/svg+xml"}

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
// /api/auth/request-link, GET /auth/verify, GET /disclaimer and GET /policy:
// the login surface itself,
// the two request/verify steps that make a session verified in the first
// place, static assets the login page needs to render, and the published
// API reference (DEC-G, 01.1-04-PLAN.md; the /swagger/* exemption's tension
// with D-05 is recorded there as accepted risk AR-32).
// The last two are the public Disclaimer and Policy pages (Phase 4 D-23,
// P4-A18), readable without logging in like the API reference above; they
// render no account data (R-07) and are cacheable only in the browser.
//
// The route table has a third part, the moderator group (D-08), which is the
// one exception to "every other route" above. It sits outside the access gate
// and answers a plain 404 that is byte equal to an unknown route to everyone
// whose verified email is not on MODERATOR_EMAILS, on every method. Its paths
// are registered with r.Handle for that reason: a method specific call would
// let chi answer a wrong method with a 405 and an Allow header, which would
// tell a stranger the path exists (correction 2).
func NewRouter(deps Deps) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// The client IP is the TCP peer address (DEC-U) unless Deps.ClientIPHeader
	// names a header the host overwrites. When it does, that header becomes the
	// client IP for every per IP limiter, so the request link limiter (the two
	// searches key on the account since quick 261016-sne) also keys on it
	// whenever it is present and parses, and falls back
	// to the peer address only when it is missing or unparseable. On a host
	// that does not overwrite that header, a caller could rotate it to dodge
	// those limiters (the T-01-92 property), so the setting is only for a host
	// confirmed by the canary check. Both middlewares fail closed, and the
	// Phase 4 network backstop skips an unknown address instead of falling
	// back. If this app is ever deployed behind a confirmed single trusted
	// reverse proxy, middleware.ClientIPFromXFFTrustedProxies configured with
	// that proxy's real hop count is the other upgrade.
	if header := strings.TrimSpace(deps.ClientIPHeader); header != "" {
		r.Use(middleware.ClientIPFromHeader(header))
	} else {
		r.Use(middleware.ClientIPFromRemoteAddr)
	}
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(deps.RequestLog))
	// Gzip for HTML, CSS, JavaScript, JSON and SVG when the client accepts it (D-14).
	r.Use(middleware.Compress(compressionLevel, compressibleContentTypes...))
	r.Use(deps.Session.Middleware(deps.persistSession))

	// Local copies of the page and auth configs that carry the shared Chrome
	// data, so the four page routes below render the unofficial bar without
	// mutating Deps.
	page := deps.Page
	page.Chrome = deps.Chrome
	auth := deps.Auth
	auth.Chrome = deps.Chrome
	// My activity reads Ended nearby with the same radius as Nearby (D-18).
	auth.DefaultRadiusKm = deps.Page.DefaultRadiusKm

	limiterClock := deps.LimiterClock
	if limiterClock == nil {
		limiterClock = time.Now
	}
	// The one gate for every Phase 4 limiter. Plan 04-15 passes it to
	// SubmitReport and plan 04-20 to the flag route.
	rateGate := handlers.NewRateGate(resolveRateLimits(deps.RateLimits), limiterClock, deps.ClientIPHeader, deps.Chrome)
	// The flag service has no Deps field: it is built here from the sessions
	// store, so every harness gets a non nil service (plan 04-20).
	flags := service.NewFlagService(deps.Sessions)
	// Same for the account service behind the /api/me routes (plan 03.4-01).
	accounts := service.NewAccountService(deps.Sessions)
	// Phase 3.2 (plan 07): the updates and mute services, built from the same
	// sessions store. The nearby reader is an untyped nil when there is no
	// ReportService, because a typed nil pointer in the interface is not a nil
	// interface (the PlaceSearch comment below says the same).
	var nearbyReader service.NearbyReader
	if deps.Reports != nil {
		nearbyReader = deps.Reports
	}
	updates := service.NewUpdatesService(deps.Sessions, nearbyReader)
	mutes := service.NewMuteService(deps.Sessions)
	// Delete my account (plan 03.2-09): built from the sessions store and the
	// optional transaction seam.
	deletions := service.NewAccountDeletionService(deps.Sessions, deps.AccountTx)
	// Log out of all devices (plan 03.2-16): built from the sessions store.
	sessionsSvc := service.NewSessionService(deps.Sessions)
	// Nearby and the Low data view start at the account's home area cell centre
	// when the device location is not available (plan 03.4-33).
	page.HomeCentre = accounts

	// --- Exempt: reachable without a verified account. ---

	r.Get("/login", handlers.LoginGate(deps.Template, auth))
	// GET only, deliberately: chi does not auto-map HEAD onto a GET-only
	// handler the way net/http.ServeMux does, and registering only GET here
	// cheaply sidesteps one class of mail-security-scanner prefetch
	// consuming a visitor's single-use link before they click it
	// (threat T-01-62). Left outside the gate — it is how a session becomes
	// verified in the first place.
	r.Get("/auth/verify", handlers.Verify(deps.AuthService, deps.Session, deps.Template, auth))
	// The public legal pages (D-23, P4-A18): no account data (R-07), cacheable only in the browser.
	r.Get("/disclaimer", handlers.Legal(deps.Template, auth, handlers.LegalDisclaimerPage))
	r.Get("/policy", handlers.Legal(deps.Template, auth, handlers.LegalPolicyPage))
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
		// Off unless Deps.OnboardingGate is set; /api/ and /welcome always pass.
		r.Use(requireOnboarded(deps.OnboardingGate))
		r.Get("/", handlers.Page(deps.Template, page))
		// D-13, D-14: the Low data view, its own bookmarkable page behind the
		// gate. It takes the same page copy so the bar numbers and the vote
		// radius match the main page.
		r.Get(handlers.LowDataPath, handlers.LowPage(deps.Template, page))
		// The report gate runs inside the handler, after decode (R-04), never as route middleware.
		r.Post("/api/reports", handlers.SubmitReport(deps.Reports, rateGate))
		r.Get("/api/reports", handlers.NearbyReports(deps.Reports))
		// The five vote routes (02-03b, HIST-09). All five come from ONE
		// handlers.CastVote factory so the JSON contract and error mapping
		// exist exactly once. Content votes (confirm/dispute) and
		// resolution votes (resolve/reopen/unresolve) are distinguished by
		// the kind argument rather than by five separate handlers, keeping
		// "is this report still true" and "is this resolved" as two
		// separately tracked signals (D-02, D-16). They are registered
		// flat, as five separate r.Post calls on full literal paths, NOT
		// via r.Route("/api/reports/{id}", ...) as 02-PATTERNS.md
		// sketches, because this router already has a literal /api/reports leaf that
		// a wildcard subrouter mounted at an overlapping prefix would sit
		// awkwardly beside; every /api/* path in this router is registered
		// flat, gated or not (see the /api/auth/request-link comment
		// above). Flat registration also keeps these five routes in the
		// same visual list as every other route in the group, which is
		// exactly the property this function's own doc comment relies on
		// for T-01-70: you can see at a glance that they are inside the
		// gate. They are inside the gate at all because a vote must be
		// attributable to a verified account, which is Phase 1.1's entire
		// reason for existing.
		//
		// D-10 and D-11: each vote route is wrapped with the one gate's vote
		// middleware, the per account vote bucket and the 30 second cooldown
		// on casting the same kind of vote again on one report, on all five
		// vote routes; the take back below is never refused by that
		// cooldown (IN-S01). This closes WR-01, the missing throttle on the
		// vote routes.
		r.With(rateGate.VoteMiddleware(service.VoteKindContent)).Post("/api/reports/{id}/confirm", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteConfirm))
		r.With(rateGate.VoteMiddleware(service.VoteKindContent)).Post("/api/reports/{id}/dispute", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteDispute))
		r.With(rateGate.VoteMiddleware(service.VoteKindResolution)).Post("/api/reports/{id}/resolve", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteResolve))
		r.With(rateGate.VoteMiddleware(service.VoteKindResolution)).Post("/api/reports/{id}/reopen", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteReopen))
		// The fifth vote route (HIST-09, D-22): the take back of a mistaken
		// Mark resolved. It has its own value, service.VoteUnresolve, and
		// does not reuse reopen, because a reopen from a non poster counts
		// as a reopen claim toward the shared threshold (D-28) while the
		// take back only withdraws the caller's own resolve and claims
		// nothing about the place. It needs no location, never counts as a
		// reopen, and shares its limit of two with the poster's reopen.
		// TakeBackMiddleware is the vote throttle except that the 30 second
		// resolution cooldown never refuses it (IN-S01), so the Undo shown
		// for 8 seconds on the Mark resolved message works; the limit of two
		// undos bounds it, and it never clears the cooldown a resolve set.
		r.With(rateGate.TakeBackMiddleware()).Post("/api/reports/{id}/unresolve", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteUnresolve))
		// POST /api/reports/{id}/flag (plan 04-20, D-09). Gated because a flag
		// must belong to a verified account. A flat literal path like the vote
		// routes. The flag bucket and the network backstop run in
		// FlagMiddleware, scoped to this one route with r.With. A cross site
		// POST carries no session cookie (DEC-L), so no CSRF token is needed.
		r.With(rateGate.FlagMiddleware()).Post("/api/reports/{id}/flag", handlers.FlagReport(flags))
		// The five Phase 3.2 routes (plan 07). Flat literal paths like the
		// others, gated because they act only for the account the gate
		// resolved. The bell polls every 30 seconds from its own script, so
		// there is no rate gate here; the per account limiter in
		// internal/ratelimit can be added later if load shows it is needed.
		// GET /api/updates refreshes the caller's updates and answers the bell.
		r.Get("/api/updates", handlers.GetUpdates(updates))
		// POST /api/updates/read marks the caller's own rows read up to an id.
		r.Post("/api/updates/read", handlers.MarkUpdatesRead(updates))
		// POST /api/reports/{id}/mute turns updates off for the caller only.
		r.Post("/api/reports/{id}/mute", handlers.MuteReport(mutes))
		// POST /api/reports/{id}/unmute turns them back on, whatever the report is now.
		r.Post("/api/reports/{id}/unmute", handlers.UnmuteReport(mutes))
		// GET /api/me/muted-reports lists the caller's mutes for Settings.
		r.Get("/api/me/muted-reports", handlers.GetMutedReports(mutes))
		// GET /api/geocode (plan 07-03). Registered as a flat literal path,
		// not under an r.Route mount — see this router's own doc comments
		// above for why every /api/* path here is flat, gated or not.
		// Gated at all because geocoding is only reachable from the report
		// submission modal, which is already behind login (D-05); leaving
		// it ungated would hand an unauthenticated caller a free proxy onto
		// this app's shared Nominatim budget.
		//
		// Quick 261016-sne: it is NOT wrapped in the per IP middleware any
		// more. The handler keys its budget by the verified account id, the
		// same as the place search below, because carrier NAT shares one IP
		// across many phones (D-16). The limiter type is ratelimit.PerIP,
		// used here only as a keyed token bucket.
		geocodeLimit := deps.GeocodeRateLimit
		if geocodeLimit == (GeocodeRateLimit{}) {
			geocodeLimit = GeocodeRateLimitDefault
		}
		geocodeBudget := ratelimit.NewPerIP(geocodeLimit.Every, geocodeLimit.Burst)
		r.Get("/api/geocode", handlers.Geocode(deps.Geocode, geocodeBudget))
		// GET /api/geocode/place (HIST-05, D-14): the reader lane. A flat
		// literal path next to /api/geocode, inside the gate for the same
		// reason. It is NOT wrapped in the per IP middleware: the handler
		// keys this budget by the verified account id, because carrier NAT
		// shares one IP across many phones (D-16). The limiter type is
		// ratelimit.PerIP, used here only as a keyed token bucket.
		placeLimit := deps.PlaceRateLimit
		if placeLimit == (PlaceRateLimit{}) {
			placeLimit = PlaceRateLimitDefault
		}
		placeBudget := ratelimit.NewPerIP(placeLimit.Every, placeLimit.Burst)
		r.Get("/api/geocode/place", handlers.GeocodePlace(deps.PlaceSearch, placeBudget))
		// deps.Template — the same ParsePageTemplate result / handlers.Page
		// gets, not a separately-parsed single file — is load-bearing:
		// profile.html.tmpl includes 01.1-06's account_header.html.tmpl
		// partial via {{template}}, which html/template only resolves
		// within the template set it was parsed into (01.1-07-PLAN.md).
		r.Get("/profile", handlers.Profile(deps.AuthService, updates, deps.Template, auth))
		// Phase 3.4 (D-22, D-25): the Official alerts place and the one
		// Settings page. Both are gated because they belong to the signed in
		// app, and both pass the onboarding middleware like every page above.
		// They serve text/html, so they carry no swag annotation.
		r.Get("/alerts", handlers.AlertsPage(deps.Template, page))
		r.Get("/settings", handlers.SettingsPage(deps.Template, accounts, mutes, page))
		// Phase 3.4 (D-27, plan 10): the first-time flow. It sits in this gated
		// group because it belongs to the signed in account, and the onboarding
		// middleware above lets this exact path through so the redirect to it
		// cannot loop. A person already through the flow gets a 303 to "/".
		r.Get("/welcome", handlers.WelcomePage(deps.Template, accounts, page))
		// D-09's explicit log-out action; only POST is ever registered for
		// this path (DEC-L: SameSite=Lax withholds the session cookie from
		// a cross-site POST, so a forged logout carries no authenticated
		// session — the same property would not hold for GET).
		r.Post("/auth/logout", handlers.Logout(deps.Session))
		// The three /api/me routes (plan 03.4-01, D-24b, D-27). Flat literal
		// paths like the others. They are gated because they change the
		// caller's own account and act only on the account the gate resolved;
		// a cross site POST carries no session cookie (DEC-L), so no CSRF
		// token is needed.
		// POST /api/me/consent records agreement to the current terms version.
		r.Post("/api/me/consent", handlers.AcceptTerms(accounts))
		// PATCH /api/me/profile saves or clears the optional name and home area.
		r.Patch("/api/me/profile", handlers.UpdateProfile(accounts))
		// POST /api/me/first-run-done marks the first-time flow finished.
		r.Post("/api/me/first-run-done", handlers.FirstRunDone(accounts))
		// POST /api/me/delete deletes the caller's account by anonymising it
		// (Phase 3.2, D-23). Gated; the strict JSON body must say confirm true
		// and the session cookie is SameSite Lax, so a cross site form cannot
		// send it.
		r.Post("/api/me/delete", handlers.DeleteAccount(deletions, deps.Session))
		// POST /api/me/logout-all ends every session of the caller's account
		// on every device, this one included (Phase 3.2, D-23). Gated, JSON
		// body plus a SameSite Lax cookie as DEC-L. Plain /auth/logout above is
		// unchanged (orchestrator decision 6).
		r.Post("/api/me/logout-all", handlers.LogOutAllDevices(sessionsSvc, deps.Session))
	})

	// --- Moderator group (D-08): outside the access gate, so an unverified
	// visitor gets the same 404 as an unknown route and never a 302 or a 401.
	// Every path is registered with r.Handle so a wrong method reaches the gate
	// and not chi's 405 (correction 2). The local page copy carries Chrome. ---

	// The moderator service has no Deps field: it is built here from the
	// sessions store and deps.ModerationTx, so every harness gets a non nil
	// service. With a nil deps.Sessions the gate answers 404 before any
	// handler runs.
	moderation := service.NewModerationService(deps.Sessions, service.WithModerationTx(deps.ModerationTx))

	r.Group(func(r chi.Router) {
		r.Use(requireModerator(deps.Sessions, deps.ModeratorEmails))
		r.Handle("/moderate", handlers.ModeratePage(deps.Template, page))
		r.Handle("/api/moderation/queue", handlers.ModerationQueue(moderation))
		r.Handle("/api/moderation/reports/{id}", handlers.ModerationReport(moderation))
		r.Handle("/api/moderation/reports/{id}/keep", handlers.ModerationKeep(moderation))
		r.Handle("/api/moderation/reports/{id}/remove", handlers.ModerationRemove(moderation))
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
