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
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"pinalert/internal/api/handlers"
	"pinalert/internal/demo"
	"pinalert/internal/push"
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
	// as Dev was — a Deps{} literal that omits
	// it (such as swagger_test.go's) still compiles and still serves
	// every route that does not touch voting.
	Votes *service.VotingService
	// Ratings is the tapped ratings service (Phase 9 plan 14). It is the one
	// signal source cmd/server/main.go hands to every reader of a reporter tag,
	// and here it backs the two rating routes and the profile's rating fields.
	// Nil, which every Deps{} literal that does not set it gives, leaves the
	// profile without rating fields and both rating routes answering 404.
	Ratings *service.RatingService

	AuthService *service.AuthService
	Auth        handlers.AuthConfig
	Template    *template.Template
	Page        handlers.PageConfig
	// Geocode backs GET /api/geocode (plan 07-01/07-03). Typed as the
	// handlers.GeocodeSearcher interface, not *geocode.Client, so this file
	// needs no import of internal/geocode and a test can substitute a stub.
	// Additive exactly as Votes was — a Deps{}
	// literal that omits it (such as swagger_test.go's) still compiles and
	// still serves every route that does not touch geocoding, and a
	// request to GET /api/geocode against a nil value degrades to the D-04
	// "search unavailable" message rather than panicking (the handler
	// guards it).
	Geocode handlers.GeocodeSearcher
	// GeocodeRateLimit configures the per account budget of GET /api/geocode
	// only (quick 261016-sne; it was a per IP limiter before). Left at its
	// zero value, GeocodeRateLimitDefault below is used:
	// cmd/server/main.go is the one place the
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
	// CommentModerationTx runs the moderator Keep and Remove of a comment
	// inside one transaction, so the change, the flag dismissal and the audit
	// row commit together (Phase 8 plan 13, D-14). cmd/server/main.go sets it
	// from the pool through service.PgxCommentModerationTx. Nil, which every
	// Deps{} literal that does not set it gives, runs the same writes without a
	// transaction (tests only).
	CommentModerationTx service.CommentModerationTxFunc
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
	// GlobeUpstream replaces the NASA GIBS base address behind the sign in
	// globe's tile routes. Only tests set it, to point at a fake server; empty,
	// which is what every Deps{} literal and production gives, means the NASA
	// constant (plan 05-04).
	GlobeUpstream string
	// GlobeUserAgent is the User-Agent sent to NASA GIBS when a tile is fetched.
	// cmd/server/main.go builds it from BASE_URL and CONTACT_EMAIL. Empty sends
	// Go's stock agent, which only a test harness gives.
	GlobeUserAgent string
	// Official backs GET /api/official/alerts (plan 05-08): the reader's
	// official alerts and whether the answer can be trusted right now. Typed as
	// the handlers.OfficialReader interface so this file needs no import of
	// internal/official and a test can substitute a stub. cmd/server/main.go
	// builds the one official.Service. Additive, as the fields above were: a
	// Deps{} literal that omits it still compiles and still serves every other
	// route, and a request against a nil value is answered 503 with a plain
	// message rather than a panic (the handler guards it).
	Official handlers.OfficialReader
	// Demo is the store of Demo worlds behind everything under /demo (plan
	// 05-10, D-04, D-05). Each visit is one in memory world running the real
	// services, so the Demo writes no table and no session. Left nil, which every
	// Deps{} literal gives, NewRouter builds a store seeded with demo.SeedSetup.
	Demo *demo.Store
	// OfficialRefreshToken is the bearer secret of POST /internal/official/refresh,
	// the door the external cron uses to refresh the official feed (plan 05-12,
	// D-15). Empty, which every Deps{} literal gives, keeps the route a plain 404.
	// cmd/server/main.go reads it from OFFICIAL_REFRESH_TOKEN and refuses a value
	// under handlers.RefreshTokenMinLen characters. The value never appears in a
	// log or a page.
	OfficialRefreshToken string
	// Credentials is the server side of Create account, the typed code, Sign in,
	// Resend and Forgot password (Phase 7.1, plan 07). cmd/server/main.go builds
	// it with keys derived from SESSION_SECRET. Additive, as the fields above were:
	// a Deps{} literal that omits it still compiles and still serves every other
	// route, and the six auth routes then answer 503 with a plain message rather
	// than panic.
	Credentials *service.CredentialService
	// AuthLimits holds the device and network buckets of those six routes. A zero
	// field takes its AuthLimitsDefault value; cmd/server/main.go sets every field
	// from its constants block. The network buckets are used only when
	// ClientIPHeader is set.
	AuthLimits AuthLimits
	// PublicBaseURL is the public address that roots every share link and the
	// public page's og:url (Phase 8 plan 03). It is never taken from the request,
	// so a crafted Host header cannot poison a forwarded link. cmd/server/main.go
	// sets it from BASE_URL and refuses to start outside ENV=development unless
	// it is an https address. Empty, which every Deps{} literal gives, means
	// handlers.DefaultPublicBaseURL, for development and tests only.
	PublicBaseURL string
	// Shares makes share links and reads their public status (Phase 8 plan 03).
	// Left nil, which every Deps{} literal gives, NewRouter builds one from the
	// sessions store and the report service with no area namer, so a title has
	// no area. cmd/server/main.go sets it with the one geocode client as the
	// namer.
	Shares *service.ShareService
	// Comments is the comment service behind the six comment routes (Phase 8
	// plan 12, PAGE-02). Left nil, which every Deps{} literal gives, NewRouter
	// builds one from the sessions store and the report service's own text
	// screener. cmd/server/main.go sets it with the screener and, only when push
	// is on, the change notifier.
	Comments *service.CommentService
	// SharePageRateLimit is the per client IP budget of GET /r/{token} (Phase 8
	// plan 04). A zero value takes SharePageRateLimitDefault. It is used only when
	// ClientIPHeader is set; without it every request is admitted.
	SharePageRateLimit SharePageRateLimit
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
		path:   loggedPath(r.URL),
	}
}

// loggedPath is the path an access log line may hold. Under /r/ the path is the
// share token, a capability that opens a report's public page and can never be
// revoked, so the line names the route instead (review WR-S01, T-08-19). The
// redaction sits here, in the formatter, so every route and the Panic line are
// covered, whatever follows the prefix. A path that only resolves under /r/
// (//r/, /%72/, /./r/, /%2Fr/) still carries the token, so the decoded path is
// cleaned and checked as well (review fix round 2, SK-04).
func loggedPath(u *url.URL) string {
	escaped := u.EscapedPath()
	if strings.HasPrefix(escaped, "/r/") || strings.HasPrefix(path.Clean("/"+u.Path), "/r/") {
		return "/r/{token}"
	}
	return escaped
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

// SharePageRateLimit is the per client IP budget of the public status page
// GET /r/{token} (Phase 8 plan 04). It is used only when Deps.ClientIPHeader is
// set, exactly like the Phase 4 network backstops: without that header the peer
// address is the host's proxy, shared by every reader and by WhatsApp's
// crawler, so a limit keyed on it would lock real readers out of a link
// forwarded into a busy group, in the emergency the page exists for. The 128 bit
// token already stops anyone walking through reports. Even with the header the
// numbers stay loose, because the crawler's addresses are shared by every
// WhatsApp user; a refusal means a plain link in the chat, not an outage. It is
// never router wide.
type SharePageRateLimit struct {
	Every time.Duration
	Burst int
}

// SharePageRateLimitDefault is used whenever Deps leaves SharePageRateLimit at
// its zero value: a burst of 120, then one more a second. cmd/server/main.go
// sets the same numbers from its constants.
var SharePageRateLimitDefault = SharePageRateLimit{Every: time.Second, Burst: 120}

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

// The budgets of the three phone alert subscription routes (Phase 3.3, D-03).
// Both are per account, keyed by the verified account id and never the client
// address, so many phones behind one carrier NAT address do not share them.
// Registering and removing a device are rare, so 5 at once and then one a
// minute leaves room for one person setting up a few devices and stops a loop.
// The status question runs each time Settings opens, so it gets 10 at once and
// then one every 6 seconds.
const (
	pushWriteBurst  = 5
	pushWriteEvery  = time.Minute
	pushStatusBurst = 10
	pushStatusEvery = 6 * time.Second
)

// BucketLimit and RateLimits are aliases of the handlers types, the same types
// under a second name. Package api imports handlers, so the gate in handlers
// cannot take a type declared here; the struct bodies live beside the gate and
// callers write api.BucketLimit and api.RateLimits.
type BucketLimit = handlers.BucketLimit
type RateLimits = handlers.RateLimits

// OfficialRefreshLimit is the per IP budget of POST /internal/official/refresh
// (plan 05-12): a burst of 6, then one call every 10 seconds. The cron calls once
// in ten minutes, so this only slows a caller guessing the token or hammering the
// route; a refused call runs no refresh.
var OfficialRefreshLimit = BucketLimit{Every: 10 * time.Second, Burst: 6}

// RateLimitsDefault mirrors the D-11 constants block in cmd/server/main.go, the
// one named place. It is used for any field a Deps literal leaves zero.
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
	// Share (Phase 8 plan 03, A-P8-4): a real reader shares a few reports in a
	// minute, so a burst of 10 then one more every 30 seconds is generous, and
	// it still bounds the reverse area lookups a share can spend. The share
	// backstop matches the flag family's.
	Share:           BucketLimit{Every: 30 * time.Second, Burst: 10},
	IPBackstopShare: BucketLimit{Every: 1200 * time.Millisecond, Burst: 1000},
	// Comment and Like (Phase 8 plan 12, D-13, A-P8-4): a real thread is a few
	// lines a minute, so a burst of 6 then one more every 20 seconds is
	// generous, and every attempt may cost one screener call, so nothing is
	// refunded. Likes, unlikes and deletes are cheap toggles: a burst of 30 then
	// one every 3 seconds. The backstops follow the "100 phones behind one
	// carrier address" rule main.go documents: 100 x the account burst, refilling
	// at 100 x the account rate.
	Comment:           BucketLimit{Every: 20 * time.Second, Burst: 6},
	IPBackstopComment: BucketLimit{Every: 200 * time.Millisecond, Burst: 600},
	Like:              BucketLimit{Every: 3 * time.Second, Burst: 30},
	IPBackstopLike:    BucketLimit{Every: 30 * time.Millisecond, Burst: 3000},
	// People (Phase 9 plan 07): the lists are public to every signed in account
	// by D-03, so reads are bounded to slow scraping and actions to slow follow
	// churn. Both are keyed per account, never per network address (CGNAT).
	PeopleRead:   BucketLimit{Every: time.Second, Burst: 60},
	PeopleAction: BucketLimit{Every: 3 * time.Second, Burst: 20},
	// Rate (Phase 9 plan 14, D-02, T-09-54): a burst of 10 then one every 30
	// seconds, per account. The once a day change rule sits on top of it.
	Rate: BucketLimit{Every: 30 * time.Second, Burst: 10},
	// Visibility (review WR-E02, 2026-10-30): turning the profile switch on may
	// call the shared text screener, so a burst of 6 then one every 20 seconds,
	// the Comment numbers, per account.
	Visibility: BucketLimit{Every: 20 * time.Second, Burst: 6},
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
	fill(&l.Share, RateLimitsDefault.Share)
	fill(&l.IPBackstopShare, RateLimitsDefault.IPBackstopShare)
	fill(&l.Comment, RateLimitsDefault.Comment)
	fill(&l.Like, RateLimitsDefault.Like)
	fill(&l.IPBackstopComment, RateLimitsDefault.IPBackstopComment)
	fill(&l.IPBackstopLike, RateLimitsDefault.IPBackstopLike)
	fill(&l.PeopleRead, RateLimitsDefault.PeopleRead)
	fill(&l.PeopleAction, RateLimitsDefault.PeopleAction)
	fill(&l.Rate, RateLimitsDefault.Rate)
	fill(&l.Visibility, RateLimitsDefault.Visibility)
	return l
}

// AuthLimits is an alias of the handlers type, like RateLimits above.
type AuthLimits = handlers.AuthLimits

// AuthLimitsDefault mirrors the auth bucket constants block in cmd/server/main.go
// (Phase 7.1, plan 07, D-02). It is used for any field a Deps literal leaves
// zero. The per device numbers are what one person at one browser needs; the
// network numbers are loose on purpose, because India's carrier NAT puts many
// phones behind one address, and they are used only with CLIENT_IP_HEADER.
var AuthLimitsDefault = AuthLimits{
	SignInDevice:     BucketLimit{Every: time.Minute, Burst: 8},
	SignInIP:         BucketLimit{Every: 2 * time.Second, Burst: 300},
	CodeSendDevice:   BucketLimit{Every: time.Minute, Burst: 5},
	CodeSendIP:       BucketLimit{Every: 10 * time.Second, Burst: 60},
	CodeSendNoCookie: BucketLimit{Every: time.Minute, Burst: 10},
	SignInNoCookie:   BucketLimit{Every: time.Minute, Burst: 10},
	CodeTryDevice:    BucketLimit{Every: 30 * time.Second, Burst: 10},
	UsernameCheck:    BucketLimit{Every: time.Second, Burst: 10},
	UsernameSave:     BucketLimit{Every: 10 * time.Second, Burst: 5},
}

// resolveAuthLimits replaces every field equal to BucketLimit{} with the
// AuthLimitsDefault field of the same name.
func resolveAuthLimits(l AuthLimits) AuthLimits {
	fill := func(got *BucketLimit, def BucketLimit) {
		if *got == (BucketLimit{}) {
			*got = def
		}
	}
	fill(&l.SignInDevice, AuthLimitsDefault.SignInDevice)
	fill(&l.SignInIP, AuthLimitsDefault.SignInIP)
	fill(&l.CodeSendDevice, AuthLimitsDefault.CodeSendDevice)
	fill(&l.CodeSendIP, AuthLimitsDefault.CodeSendIP)
	fill(&l.CodeSendNoCookie, AuthLimitsDefault.CodeSendNoCookie)
	fill(&l.SignInNoCookie, AuthLimitsDefault.SignInNoCookie)
	fill(&l.CodeTryDevice, AuthLimitsDefault.CodeTryDevice)
	fill(&l.UsernameCheck, AuthLimitsDefault.UsernameCheck)
	fill(&l.UsernameSave, AuthLimitsDefault.UsernameSave)
	return l
}

// The CredentialService is the login state the sign in page reads; a mismatch
// of the interface fails the build here.
var _ handlers.LoginState = (*service.CredentialService)(nil)

// @title        Pinalert API
// @version      1.0
// @description  Crowd-reported local emergency feed for floods, cyclones, and other disasters.
// @description  Reports are posted by nearby people and confirmed or disputed by other nearby
// @description  people; this API is the read/write surface for that data. Reading and writing
// @description  reports both require a signed in session, an account signed in with an email
// @description  and a password, and a session that is not signed in is refused with 401. Pinalert is an unofficial,
// @description  unaffiliated project and is not a substitute for contacting emergency services.
// @description  This document is Swagger 2.0: swag (the generator behind this spec) does not
// @description  emit OpenAPI 3.x, so tooling that expects OpenAPI 3.x specifically should account
// @description  for that; OPS-01's "OpenAPI/Swagger spec" requirement is satisfied by either
// @description  format.
// @BasePath     /api
// @license.name Unlicensed (all rights reserved)
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
//
// Phase 3.3 (plan 04) adds application/manifest+json, the type of the web app
// manifest, which is small but is plain text like the rest.
var compressibleContentTypes = []string{"text/html", "text/css", "text/javascript", "application/javascript", "application/json", "application/manifest+json", "image/svg+xml"}

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
// The exempt set is exactly: /static/*, /swagger/*, GET /login, POST /login (a
// fixed 303 back to GET /login that never reads the body, review WR-T04), the six POST
// /api/auth routes (sign-up, sign-up/code, code/resend, sign-in, reset and
// reset/code), GET /auth/verify (the retired emailed link redirect), GET
// /disclaimer and GET /policy, the ten app asset paths, and Phase 5's entries
// (the Demo prefix, the refresh route and the globe tiles): the login surface
// itself, the code and password steps that give a person an account and a
// session in the first place, static assets the login page needs to render, and
// the published API reference (DEC-G, 01.1-04-PLAN.md; the /swagger/*
// exemption's tension with D-05 is recorded there as accepted risk AR-32).
//
// Amended 2026-10-29 by Phase 7.1 plan 14 (D-12): this is the final list of the
// phase. The emailed link request route is removed, and GET /auth/verify stays
// only as a redirect (303 to /login?done=old-link) for links already sent. Phase
// 8 adds its shared report status page to it.
// The last two are the public Disclaimer and Policy pages (Phase 4 D-23,
// P4-A18), readable without logging in like the API reference above; they
// render no account data (R-07) and are cacheable only in the browser.
//
// Amended 2026-10-18 by Phase 3.3 plan 04 (DEC-G, Phase 1.1 D-05, decision 13):
// ten more exact GET paths join the exempt set, listed once in
// handlers.PWAPublicPaths: /manifest.webmanifest, /sw.js, the five app icons
// under /icons, the SVG mark /icons/app.svg, and the two root apple touch icon
// addresses iOS asks for by itself. They carry nothing sensitive and a browser
// fetches them without the session cookie, so an install would fail on a
// redirect to /login. They also skip the session middleware (skipSessionFor),
// so a cookieless fetch neither sets a cookie nor writes a sessions row.
//
// Amended 2026-10-28 by Phase 5 plan 05-04 (D-13): 85 more exact GET paths join
// the exempt set, /globe/tiles/{z}/{y}/{x}.jpg for zoom 0 to 3, listed once in
// handlers.GlobeTilePaths. They are the Earth pictures of the sign in globe,
// served from Pinalert's memory after one fetch each from NASA GIBS, and they
// skip the session middleware like the app assets. The skip function now takes
// one rule set, sessionSkip: exact GET paths, exact POST paths and path
// prefixes, all built in sessionSkipSet, so the later sessionless routes of
// Phase 5 join that set instead of adding a second bypass.
//
// Amended 2026-10-29 by Phase 5 plan 05-10 (D-04): everything under /demo joins
// the exempt set and skips the session middleware (the first entry of the rule
// set's prefixes). It is the Demo's sandbox, mounted from handlers.DemoRoutes: it
// reads and writes no account and no table, sets no cookie and writes no sessions
// row. Only the /demo prefix is exempt: /demonstration, dot segments, a double
// slash and another case are not Demo routes, and /api/reports, /alerts and
// /api/official/alerts stay behind the gate.
//
// Amended 2026-10-29 by Phase 5 plan 05-12 (D-15): one exact POST path joins the
// exempt set, /internal/official/refresh, the door of the external cron that
// keeps official alerts current while the host sleeps. It is protected by its own
// bearer token (Deps.OfficialRefreshToken), skips the session middleware (the
// first entry of the rule set's POST paths, so about 4,300 calls a month write no
// sessions row), and answers a plain 404 when no token is set and for every
// method but POST, registered with r.Handle so chi never answers 405. A per IP
// limiter wraps only the POST path of a configured route, so the limiter never
// shows a hidden route.
//
// Amended 2026-10-29 by Phase 7.1 plan 07 (D-03): six more exact POST paths join
// the exempt set, the code and password routes: /api/auth/sign-up,
// /api/auth/sign-up/code, /api/auth/code/resend, /api/auth/sign-in,
// /api/auth/reset and /api/auth/reset/code. They are how a person gets an
// account and a session in the first place, so they cannot sit behind the gate.
// Each one is wrapped by net/http CrossOriginProtection and takes a strict
// application/json body of at most 4 KB. They do NOT skip the session
// middleware: the typed code row and the new session binding are keyed on the
// browser's session id. The older link routes stay until the cleanup plans.
//
// Amended 2026-10-29 by Phase 8 plan 04 (D-05, D-06): GET /r/{token} joins the
// exempt set and skips the session middleware under the /r prefix; it is server
// rendered, limited per client IP only when CLIENT_IP_HEADER is set, and reads
// one report's public status by a random token. A numeric id, a bad token and
// every near miss spelling never answer 200, and opening the page, by a person
// or by WhatsApp's crawler, writes no sessions row and sets no cookie.
//
// The route table has a third part, the moderator group (D-08), which is the
// one exception to "every other route" above. It sits outside the access gate
// and answers a plain 404 that is byte equal to an unknown route to everyone
// whose verified email is not on MODERATOR_EMAILS, on every method. Its paths
// are registered with r.Handle for that reason: a method specific call would
// let chi answer a wrong method with a 405 and an Allow header, which would
// tell a stranger the path exists (correction 2).
func NewRouter(deps Deps) *chi.Mux {
	// The manifest, the worker and the icons are built once, here. Building can
	// only fail on a build defect (a missing embedded file), so it panics.
	assets, err := handlers.NewPWAAssets(web.StaticFS)
	if err != nil {
		panic("api: building the installable app assets: " + err.Error())
	}
	// The globe tiles are fetched lazily, once each, so building this fetches
	// nothing.
	globeTiles := handlers.NewGlobeTiles(handlers.GlobeTilesConfig{
		Upstream:  deps.GlobeUpstream,
		UserAgent: deps.GlobeUserAgent,
	})

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// The client IP is the TCP peer address (DEC-U) unless Deps.ClientIPHeader
	// names a header the host overwrites. When it does, that header becomes the
	// client IP for every per IP limiter, so those limiters (the two
	// searches key on the account since quick 261016-sne) also key on it
	// whenever it is present and parses, and fall back
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
	// The session middleware runs for every request except the public sessionless
	// routes in sessionSkipSet: the exact public app asset paths (plan 03.3-04, a
	// launcher or manifest fetch without a cookie must not create a session) and
	// the 85 globe tile paths (plan 05-04), everything under /demo (plan 05-10),
	// the cron's refresh POST (plan 05-12) and the static files under /static
	// (2026-10-29, follow up to plan 05-13), and the public status page under /r
	// (plan 08-04).
	r.Use(skipSessionFor(sessionSkipSet(), deps.Session.Middleware(deps.persistSession)))

	// Local copies of the page and auth configs that carry the shared Chrome
	// data, so the four page routes below render the unofficial bar without
	// mutating Deps.
	page := deps.Page
	page.Chrome = deps.Chrome
	auth := deps.Auth
	auth.Chrome = deps.Chrome
	// My activity reads Ended nearby with the same radius as Nearby (D-18).
	auth.DefaultRadiusKm = deps.Page.DefaultRadiusKm
	// The sign in page's login state and the session manager (Phase 7.1, plan 07).
	// Login is assigned only when the service exists: a typed nil pointer stored
	// in the interface would not be a nil interface (the PlaceSearch comment says
	// the same).
	if deps.Credentials != nil {
		auth.Login = deps.Credentials
	}
	auth.Session = deps.Session

	limiterClock := deps.LimiterClock
	if limiterClock == nil {
		limiterClock = time.Now
	}
	// The one gate for every Phase 4 limiter. Plan 04-15 passes it to
	// SubmitReport and plan 04-20 to the flag route.
	rateGate := handlers.NewRateGate(resolveRateLimits(deps.RateLimits), limiterClock, deps.ClientIPHeader, deps.Chrome)
	// The flag service has no Deps field: it is built here from the sessions
	// store, so every harness gets a non nil service (plan 04-20). Its note goes
	// through the report service's own text screener, so a flag note meets the
	// same abuse screen as a report description, without the gibberish rule
	// (plan 04-31). Without a report service the note is not screened, as a
	// description is not.
	var flagOpts []service.FlagOption
	if deps.Reports != nil {
		flagOpts = append(flagOpts, service.WithFlagScreener(deps.Reports.Screener()))
	}
	flags := service.NewFlagService(deps.Sessions, flagOpts...)
	// The report history service (Phase 8, plan 01). Like the flag service it has
	// no Deps field. Its Focus gate is the report service, so without one every
	// read is an error and the route is never reached by a real request.
	timeline := service.NewTimelineService(deps.Sessions, deps.Reports)
	// The share service and the configured base of every share link (Phase 8
	// plan 03). A nil Shares builds one with no area namer; an empty base
	// becomes the development default. The base comes from configuration only.
	shares := deps.Shares
	if shares == nil {
		shares = service.NewShareService(deps.Sessions, deps.Reports)
	}
	// The comment service (Phase 8 plan 12). A nil Comments builds one that
	// screens through the report service's own screener, like the flag service.
	comments := deps.Comments
	if comments == nil {
		var commentOpts []service.CommentOption
		if deps.Reports != nil {
			commentOpts = append(commentOpts, service.WithCommentScreener(deps.Reports.Screener()))
		}
		comments = service.NewCommentService(deps.Sessions, commentOpts...)
	}
	shareBase := deps.PublicBaseURL
	if shareBase == "" {
		shareBase = handlers.DefaultPublicBaseURL
	}
	// Same for the account service behind the /api/me routes (plan 03.4-01).
	// Its name and home area edits go through the report service's text screener,
	// like a flag note does (Phase 7.1, plan 06); without a report service they
	// are not screened.
	var accountOpts []service.AccountOption
	if deps.Reports != nil {
		accountOpts = append(accountOpts, service.WithAccountScreener(deps.Reports.Screener()))
	}
	// Phase 9 plan 07 (G1): a Name edit while the profile is on is screened, so
	// the account service reads the public profile state from the same store.
	accountOpts = append(accountOpts, service.WithPublicProfileReader(deps.Sessions))
	accounts := service.NewAccountService(deps.Sessions, accountOpts...)
	// Phase 3.3 (plan 08): the phone alert subscription service, from the same
	// sessions store and the push service allowlist.
	pushSubs := service.NewPushSubscriptionService(deps.Sessions, push.DefaultEndpointPolicy())
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
	// Phase 9 plan 07: the people service (opt in switch, follow, block, lists,
	// profile). It screens a Name and a pending username through the same
	// screener the account service uses; without a report service nothing is
	// screened here, as for a flag note.
	var peopleOpts []service.PeopleOption
	if deps.Reports != nil {
		peopleOpts = append(peopleOpts, service.WithPeopleScreener(deps.Reports.Screener()))
	}
	// Plan 14: the same ratings source every other tag reader uses, so the
	// profile reads the tag every other surface reads and fills may_rate and
	// your_rating for the reader. A nil Ratings adds no option (a typed nil pointer in the interface would not be a nil interface).
	if deps.Ratings != nil {
		peopleOpts = append(peopleOpts,
			service.WithPeopleSignals(deps.Ratings),
			service.WithPeopleRatings(deps.Ratings),
		)
	}
	// Quick 261030-u78 (WR-E04): the comment service tells the people service
	// who wrote a comment, for the Block button beside Flag.
	peopleOpts = append(peopleOpts, service.WithPeopleCommentAuthors(comments))
	people := service.NewPeopleService(deps.Sessions, peopleOpts...)
	// Delete my account (plan 03.2-09): built from the sessions store and the
	// optional transaction seam.
	// With a CredentialService, deleting an account also clears the sign in
	// throttle row of the address, through the service's own keyed hash.
	var deleteOpts []service.AccountDeleteOption
	if deps.Credentials != nil {
		deleteOpts = append(deleteOpts, service.WithThrottleKeyer(deps.Credentials.ThrottleKey))
	}
	deletions := service.NewAccountDeletionService(deps.Sessions, deps.AccountTx, deleteOpts...)
	// Log out of all devices (plan 03.2-16): built from the sessions store.
	sessionsSvc := service.NewSessionService(deps.Sessions)
	// Nearby and the Low data view start at the account's home area cell centre
	// when the device location is not available (plan 03.4-33).
	page.HomeCentre = accounts
	// Nearby and the Low data view read the Changes switch to decide whether to
	// render the first post or vote pop-up (plan 03.3-21).
	page.AlertSettings = accounts
	// Nearby reads the home area name for the installed app alerts ask
	// (plan 07.1-13c).
	page.Profiles = accounts

	// --- Exempt: reachable without a verified account. ---

	r.Get("/login", handlers.LoginGate(deps.Template, auth))
	// The card's forms post to /login when the page script has not loaded yet
	// (review WR-T04): a fixed 303 back to the page, no-store, and the body,
	// which carries a plaintext password, is never read.
	r.Post("/login", handlers.LoginFormFallback())
	// The retired emailed link (Phase 7.1 plan 14, D-12): an old link already in
	// somebody's inbox still reaches this path, so it stays public and GET only,
	// and answers a 303 to /login?done=old-link, where the sign in page shows the
	// plain old-link line. It checks no token and signs nobody in (T-07.1-31).
	// Kept outside the gate for the same reason /login is: the person has no
	// account session yet.
	r.Get("/auth/verify", handlers.RetiredLink())
	// The public legal pages (D-23, P4-A18): no account data (R-07), cacheable only in the browser.
	r.Get("/disclaimer", handlers.Legal(deps.Template, auth, handlers.LegalDisclaimerPage))
	r.Get("/policy", handlers.Legal(deps.Template, auth, handlers.LegalPolicyPage))
	// The public status page of a shared link (plan 08-04, D-05): the one Phase 8
	// route that opens without an account. It is addressed by a random 128 bit
	// token, never by a report id, so nobody can walk through reports, and it is
	// server rendered with no script, no session and no cookie (the /r prefix is in
	// sessionSkipSet). The per client IP limit exists only when CLIENT_IP_HEADER is
	// set; with no header the peer address is the host's shared proxy, so every
	// request is admitted and the token's size is the guard (review B1).
	sharePageAllow := func(*http.Request) bool { return true }
	if strings.TrimSpace(deps.ClientIPHeader) != "" {
		pageLimit := deps.SharePageRateLimit
		if pageLimit == (SharePageRateLimit{}) {
			pageLimit = SharePageRateLimitDefault
		}
		sharePageLimiter := ratelimit.NewPerIP(pageLimit.Every, pageLimit.Burst)
		sharePageAllow = func(req *http.Request) bool {
			ip := middleware.GetClientIP(req.Context())
			// An address that is unknown is admitted, as the network backstops skip it.
			return ip == "" || sharePageLimiter.Allow(ip)
		}
	}
	r.Get("/r/{token}", handlers.PublicStatusPage(deps.Template, auth, shares, shareBase, sharePageAllow))
	// The Demo (plan 05-10, D-04, owner 2026-10-17): a visitor with no account can
	// try Pinalert first. Everything under /demo is public and sessionless (the
	// prefix is in sessionSkipSet), and it reads and writes no account and no
	// table: each visit is an in memory world running the real services. Only the
	// /demo prefix is exempt; /api/reports, /alerts and /api/official/alerts stay
	// behind the gate below. The Demo's own per IP limits live in DemoRoutes.
	demoStore := deps.Demo
	if demoStore == nil {
		demoStore = demo.NewStore(demo.WithWorldSetup(demo.SeedSetup))
	}
	// Plan 05-13: the same mount also serves the Demo's Nearby page (GET /demo),
	// which renders with the page templates and the real Nearby page's
	// configuration, so the Demo shows the real feed radius and vote distance.
	r.Mount("/demo", handlers.DemoRoutes(handlers.DemoConfig{Store: demoStore, Template: deps.Template, Page: page}))
	// The installable app assets (plan 03.3-04, T-03.3-05): the manifest, the
	// service worker and the icons. Each is an exact GET path, never a prefix,
	// taken from the one list handlers.PWAPublicPaths, which is also what builds
	// the session bypass above, so the routes and the bypass are the same set by
	// construction. They are public because the browser fetches them without the
	// session cookie and an install would fail on a redirect to /login; none of
	// them reads or writes an account.
	for _, path := range handlers.PWAPublicPaths {
		h, ok := assets.Handler(path)
		if !ok {
			panic("api: no handler for the public app asset " + path)
		}
		r.Method(http.MethodGet, path, h)
	}
	// A browser's own favicon request (review WR-T02): one exact GET route that
	// answers a plain 404, in sessionSkipSet's GET set, so it sets no cookie.
	r.Get(handlers.FaviconPath, handlers.FaviconNotFound)
	// The sign in globe's Earth pictures (plan 05-04, D-13): the same shape, one
	// exact GET route for each of the 85 entries of handlers.GlobeTilePaths, which
	// is also what feeds the session bypass above. Public because the sign in page
	// is where a visitor lands; each picture is fetched once from NASA and served
	// from memory.
	for _, path := range handlers.GlobeTilePaths() {
		h, ok := globeTiles.Handler(path)
		if !ok {
			panic("api: no handler for the globe tile " + path)
		}
		r.Method(http.MethodGet, path, h)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", staticFileServer(deps.Dev)))
	// Left publicly reachable: this is a public read-only API with no
	// privileged operations to hide, and OPS-01 asks for a stable,
	// browsable URL. /swagger/index.html is that stable URL. The v2 handler
	// serves its UI assets from the binary — no third-party CDN request at
	// view time.
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	// Every /api/* path is registered as a flat literal path (not nested under an
	// r.Route("/api", ...) mount) so the exempt ones can stay outside the gated
	// group below while /api/reports stays inside it: chi does not allow a
	// literal sibling path alongside a wildcard r.Route mount at the same prefix,
	// so /api is never mounted as its own subrouter in this router.
	//
	// The six code and password routes (Phase 7.1, plan 07, D-03). Flat literal
	// paths outside the gate: they are how a person gets an account and a session
	// in the first place. They are always registered, so
	// the exempt list is constant; a router built without Deps.Credentials answers
	// each with a 503. cop rejects a cross site browser POST (login CSRF, research
	// Pattern 4) and lets a request without Origin and Sec-Fetch-Site through,
	// which is every script and every same site fetch. None of the six skips the
	// session middleware: the code row and the new session binding are keyed on
	// the browser's session id (research section 7). The buckets are the device
	// and, with CLIENT_IP_HEADER, the network ones of handlers.AuthGate.
	authGate := handlers.NewAuthGate(resolveAuthLimits(deps.AuthLimits), limiterClock, deps.ClientIPHeader)
	cop := http.NewCrossOriginProtection()
	r.With(cop.Handler).Post("/api/auth/sign-up", handlers.SignUp(deps.Credentials, authGate))
	r.With(cop.Handler).Post("/api/auth/sign-up/code", handlers.ConfirmSignUp(deps.Credentials, authGate, deps.Session))
	r.With(cop.Handler).Post("/api/auth/code/resend", handlers.ResendCode(deps.Credentials, authGate))
	r.With(cop.Handler).Post("/api/auth/sign-in", handlers.SignIn(deps.Credentials, authGate, deps.Session))
	r.With(cop.Handler).Post("/api/auth/reset", handlers.RequestReset(deps.Credentials, authGate))
	r.With(cop.Handler).Post("/api/auth/reset/code", handlers.ConfirmReset(deps.Credentials, authGate))

	// The external cron's door (plan 05-12, D-15, research G1). Outside the gate,
	// because the cron has no account; its own bearer token is the guard, and the
	// exact path is in sessionSkipSet's POST set, so a call writes no sessions row.
	// Registered with r.Handle and always present, so a wrong method reaches the
	// handler's 404 and never chi's 405. The handler answers the 404 for an unset
	// token and for any method but POST; only a POST to a configured route is
	// counted by the per IP limiter, so a flood of GETs or of calls to an unset
	// route never turns the 404 into a 429 that shows the route exists. A wrong
	// token spends the budget too, which slows guessing. The official service is
	// the refresher when it implements RefreshAll; otherwise the handler answers
	// 503 for a right token.
	refresher, _ := deps.Official.(handlers.OfficialRefresher)
	refreshDoor := handlers.OfficialRefresh(refresher, deps.OfficialRefreshToken)
	refreshLimited := ratelimit.NewPerIP(OfficialRefreshLimit.Every, OfficialRefreshLimit.Burst).Middleware()(refreshDoor)
	r.Handle("/internal/official/refresh", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if deps.OfficialRefreshToken == "" || req.Method != http.MethodPost {
			refreshDoor.ServeHTTP(w, req)
			return
		}
		refreshLimited.ServeHTTP(w, req)
	}))

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
		// flat, gated or not (see the comment above the six code and
		// password routes). Flat registration also keeps these five routes in the
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
		// GET /api/reports/{id}/timeline (Phase 8 plan 01, PAGE-01, SC2): the
		// report page's history, built from the vote log. Gated like every /api
		// route and flat like the vote routes. No rate gate: it is a read polled
		// at the feed's 30 second cadence.
		r.Get("/api/reports/{id}/timeline", handlers.ReportTimeline(timeline))
		// POST /api/reports/{id}/share (Phase 8 plan 03, COORD-08, D-04): makes
		// or returns the report's share link. Gated so a token and an area
		// lookup are never made by someone without an account. The share bucket
		// and the network backstop run in ShareMiddleware, scoped to this one
		// route with r.With. The absolute address comes from the configured
		// base, never from the request.
		r.With(rateGate.ShareMiddleware()).Post("/api/reports/{id}/share", handlers.ShareReport(shares, shareBase))
		// The seven comment routes (Phase 8 plan 12, PAGE-02, SC3). Gated, flat
		// literal paths. The list is a read polled like the bell, so it has no
		// bucket. Posts and replies spend the comment bucket, with no refund
		// because each attempt may cost a screener call. Likes, unlikes and
		// deletes spend the like bucket. A comment flag shares the report flag
		// bucket and its network backstop (D-14). Block beside Flag spends the
		// per account people action bucket like a profile block (quick 261030-u78).
		r.Get("/api/reports/{id}/comments", handlers.ListComments(comments))
		r.With(rateGate.CommentMiddleware()).Post("/api/reports/{id}/comments", handlers.PostComment(comments))
		r.With(rateGate.LikeMiddleware()).Post("/api/comments/{id}/like", handlers.LikeComment(comments))
		r.With(rateGate.LikeMiddleware()).Post("/api/comments/{id}/unlike", handlers.UnlikeComment(comments))
		r.With(rateGate.LikeMiddleware()).Post("/api/comments/{id}/delete", handlers.DeleteComment(comments))
		r.With(rateGate.FlagMiddleware()).Post("/api/comments/{id}/flag", handlers.FlagComment(comments))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/comments/{id}/block-author", handlers.BlockCommentAuthor(people))
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
		// The twelve Phase 9 people routes (plan 07), flat literal paths in the
		// gated group. Every read goes through the per account PeopleRead bucket
		// and every action through the per account PeopleAction bucket; nothing
		// is keyed on the network address (CGNAT). Every route that reads a
		// person answers one byte equal 404 for any person the caller may not
		// see (handlers.writePersonNotFound).
		r.With(rateGate.PeopleReadMiddleware()).Get("/api/people/{username}", handlers.PersonProfile(people))
		r.With(rateGate.PeopleReadMiddleware()).Get("/api/people/{username}/followers", handlers.PersonFollowers(people))
		r.With(rateGate.PeopleReadMiddleware()).Get("/api/people/{username}/following", handlers.PersonFollowing(people))
		r.With(rateGate.PeopleReadMiddleware()).Get("/api/people/{username}/reports", handlers.PersonReports(people, deps.Reports))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/follow", handlers.FollowPerson(people))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/unfollow", handlers.UnfollowPerson(people))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/remove-follower", handlers.RemoveFollower(people))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/block", handlers.BlockPerson(people))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/unblock", handlers.UnblockPerson(people))
		// The two rating routes (Phase 9 plan 14, D-02): give or remove a star
		// rating, through the per account Rate bucket. Without a ratings service
		// both answer the uniform 404.
		r.With(rateGate.RateMiddleware()).Post("/api/people/{username}/rating", handlers.RatePerson(deps.Ratings))
		r.With(rateGate.RateMiddleware()).Post("/api/people/{username}/rating/clear", handlers.ClearPersonRating(deps.Ratings))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/me/followers/remove-all", handlers.RemoveAllFollowers(people))
		r.With(rateGate.PeopleReadMiddleware()).Get("/api/me/blocked", handlers.BlockedPeopleList(people))
		// Person mute (Phase 9 plan 19, D-12): one way and quiet, buckets as above.
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/mute", handlers.MutePerson(people))
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/unmute", handlers.UnmutePerson(people))
		r.With(rateGate.PeopleReadMiddleware()).Get("/api/me/muted-people", handlers.MutedPeopleList(people))
		// Plan 09-23 (D-08): the Notify me bell of one followed person.
		r.With(rateGate.PeopleActionMiddleware()).Post("/api/people/{username}/notify", handlers.NotifyPerson(people))
		r.With(rateGate.PeopleActionMiddleware()).Patch("/api/me/visibility", handlers.UpdateVisibility(people, rateGate))
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
		// Plan 05-08: the data behind that page and the Nearby banner. Gated
		// because the reader's position travels in its query and the answer is
		// for the signed in app only. A read also starts the lazy SACHET
		// refresh inside the handler; there is no rate gate here because the
		// refresher's own gap and single flight bound what reaches NDMA.
		r.Get("/api/official/alerts", handlers.OfficialAlerts(deps.Official))
		// Plan 05-21: the GDACS hazard pins for the map, gated like the alerts
		// above and so not in the session skip set. The pins reader is found on
		// the same Deps.Official value, so Deps and main.go are unchanged; a value
		// that does not offer it leaves the handler a nil reader and a plain 503.
		pinsReader, _ := deps.Official.(handlers.OfficialPinsReader)
		r.Get("/api/official/pins", handlers.OfficialPins(pinsReader))
		r.Get("/settings", handlers.SettingsPage(deps.Template, accounts, mutes, people, page))
		// Phase 9 plan 08 (D-04, UI-SPEC Surface 1): the profile page, one for
		// your own profile and for anyone else's, and /me which answers a 303 to
		// /u/{your username}. Gated like /settings. Amended 2026-10-30 (review
		// WR-E01): the page load takes a token from the same per account people
		// read bucket as the JSON reads, because it does the same work (the
		// profile, the evaluate on touch write and the recent reports) and a
		// door with no bucket would let a loop over a wordlist skip the throttle.
		// A refusal is a 429 page, never JSON. Its actions pass through the
		// people buckets of the JSON routes above.
		r.With(rateGate.PeopleReadPageMiddleware(page.AssetVersion)).Get("/u/{username}", handlers.PersonPage(deps.Template, people, deps.Reports, page))
		r.Get("/me", handlers.MyProfilePage(people))
		// Phase 3.4 (D-27, plan 10): the first-time flow. It sits in this gated
		// group because it belongs to the signed in account, and the onboarding
		// middleware above lets this exact path through so the redirect to it
		// cannot loop. A person already through the flow gets a 303 to "/".
		r.Get("/welcome", handlers.WelcomePage(deps.Template, accounts, page))
		// D-09's explicit log-out action; only POST is ever registered for
		// this path (DEC-L: SameSite=Lax withholds the session cookie from
		// a cross-site POST, so a forged logout carries no authenticated
		// session — the same property would not hold for GET).
		// Phase 3.3 plan 10 (decision 11, UI-SPEC O-3): it also deletes the push
		// subscriptions this session registered, before the cookie is cleared.
		r.Post("/auth/logout", handlers.Logout(deps.Session, pushSubs))
		// The three /api/me routes (plan 03.4-01, D-24b, D-27). Flat literal
		// paths like the others. They are gated because they change the
		// caller's own account and act only on the account the gate resolved;
		// a cross site POST carries no session cookie (DEC-L), so no CSRF
		// token is needed.
		// POST /api/me/consent records agreement to the current terms version.
		r.Post("/api/me/consent", handlers.AcceptTerms(accounts))
		// PATCH /api/me/profile saves the username, the name and the home area.
		// Amended 2026-10-29 by Phase 7.1 plan 10: it takes the username and the
		// per account save allowance of authGate.
		r.Patch("/api/me/profile", handlers.UpdateProfile(accounts, authGate))
		// GET /api/username/check answers the live check while a person types a
		// username (Phase 7.1, D-07). Gated, per account limited by authGate,
		// never cached, and it never calls the text screening.
		r.Get("/api/username/check", handlers.UsernameCheck(accounts, authGate))
		// PATCH /api/me/alerts saves the Reliable reporter alert switch and its
		// distance from the home area (Phase 3.2, NOTIF-05, D-26). Gated; the
		// account comes from the gate and the body names nothing else.
		r.Patch("/api/me/alerts", handlers.UpdateAlertSettings(accounts))
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
		// The three phone alert subscription routes (Phase 3.3, D-03, D-13,
		// plan 08). Gated, strict application/json bodies of at most 4 KB, and
		// a SameSite Lax cookie as DEC-L. The endpoint is a secret address, so
		// it travels only in the body, never in a URL, and every answer leaves
		// it out. The budgets are per account (CGNAT safe), not per IP. Plans
		// 10 and 11 delete subscriptions on logout and account deletion.
		// POST /api/me/push/subscription registers this device.
		pushWriteLimiter := ratelimit.NewPerIP(pushWriteEvery, pushWriteBurst)
		pushStatusLimiter := ratelimit.NewPerIP(pushStatusEvery, pushStatusBurst)
		r.Post("/api/me/push/subscription", handlers.RegisterPushSubscription(pushSubs, deps.Page.PushPublicKey, pushWriteLimiter))
		// POST /api/me/push/status says whether this device is registered.
		r.Post("/api/me/push/status", handlers.PushSubscriptionStatus(pushSubs, pushStatusLimiter))
		// POST /api/me/push/unsubscribe removes this device for the caller only.
		r.Post("/api/me/push/unsubscribe", handlers.UnregisterPushSubscription(pushSubs, pushWriteLimiter))
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
	// The comment side (Phase 8 plan 13). The comment service resolved above is
	// the notifier, so Keep of a held comment tells its recipient once.
	commentModeration := service.NewCommentModerationService(deps.Sessions, deps.CommentModerationTx, comments)

	r.Group(func(r chi.Router) {
		r.Use(requireModerator(deps.Sessions, deps.ModeratorEmails))
		r.Handle("/moderate", handlers.ModeratePage(deps.Template, page))
		r.Handle("/api/moderation/queue", handlers.ModerationQueue(moderation))
		r.Handle("/api/moderation/reports/{id}", handlers.ModerationReport(moderation))
		r.Handle("/api/moderation/reports/{id}/keep", handlers.ModerationKeep(moderation))
		r.Handle("/api/moderation/reports/{id}/remove", handlers.ModerationRemove(moderation))
		r.Handle("/api/moderation/comments", handlers.ModerationComments(commentModeration))
		r.Handle("/api/moderation/comments/{id}/keep", handlers.ModerationCommentKeep(commentModeration))
		r.Handle("/api/moderation/comments/{id}/remove", handlers.ModerationCommentRemove(commentModeration))
		// Phase 9 plan 15 (D-14): the tapped ratings tool. The rights check is this
		// group's requireModerator; the service methods check none.
		r.Handle("/api/moderation/people/{username}/ratings", handlers.ModerationRatingsList(deps.Ratings))
		r.Handle("/api/moderation/people/{username}/ratings/remove", handlers.ModerationRatingRemove(deps.Ratings))
		r.Handle("/api/moderation/raters/{username}/remove-ratings", handlers.ModerationRaterRemoveAll(deps.Ratings))
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

// sessionSkip is the one rule set of every public, sessionless route: the
// requests that go straight to their handler without a cookie and without a
// sessions row. get holds exact GET paths, post exact POST paths, and prefixes
// path prefixes that cover both GET and POST (a prefix "/p" covers "/p" and
// anything under "/p/"). Later plans add to this one set instead of writing a
// second bypass.
type sessionSkip struct {
	get      map[string]bool
	post     map[string]bool
	prefixes []string
}

// matches reports whether the request skips the session middleware. It is
// deliberately strict, so a crafted address can never write a sessions row for
// free or reach a sessionless handler by a spelling the router would read
// differently (T-03.3-14, T-05-19): the URL carries no encoded form (RawPath is
// empty), the path holds no double slash, no "." or ".." segment, and then one
// of three things holds: the method is GET and the path is an exact get entry,
// the method is POST and the path is an exact post entry, or the method is GET
// or POST and the path is a prefix or lies under a prefix followed by a slash.
// Every other request, a HEAD, a PUT, a trailing slash or a different case
// included, goes through the session middleware as before.
func (s sessionSkip) matches(r *http.Request) bool {
	if r.URL.RawPath != "" {
		return false
	}
	p := r.URL.Path
	if strings.Contains(p, "//") || strings.Contains(p, "/./") || strings.Contains(p, "/../") ||
		strings.HasSuffix(p, "/.") || strings.HasSuffix(p, "/..") {
		return false
	}
	switch r.Method {
	case http.MethodGet:
		if s.get[p] {
			return true
		}
	case http.MethodPost:
		if s.post[p] {
			return true
		}
	default:
		return false
	}
	for _, prefix := range s.prefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

// sessionSkipSet builds the rule set the router uses, from the same lists the
// routes are registered from: the exact GET paths are handlers.PWAPublicPaths
// (plan 03.3-04), handlers.GlobeTilePaths() (plan 05-04) and
// handlers.FaviconPath (review WR-T02). The one exact POST
// path is the external cron's refresh endpoint (plan 05-12). The prefixes are
// /demo (plan 05-10), /static (the Phase 5 follow up) and /r (plan 08-04).
func sessionSkipSet() sessionSkip {
	tiles := handlers.GlobeTilePaths()
	get := make(map[string]bool, len(handlers.PWAPublicPaths)+len(tiles)+1)
	for _, path := range handlers.PWAPublicPaths {
		get[path] = true
	}
	for _, path := range tiles {
		get[path] = true
	}
	// "/favicon.ico" (added 2026-10-29, review WR-T02): a browser that opens the
	// public page of a shared link with no cookie may still ask for it, and that
	// request must not set a cookie or write a sessions row. Its route answers a
	// plain 404.
	get[handlers.FaviconPath] = true
	// "/demo" covers GET and POST under it: the Demo's sandbox API (plan 05-10).
	// The cron's refresh door: an exact POST path, so a call neither sets a cookie
	// nor writes a sessions row (plan 05-12, research G1, Pitfall 2).
	post := map[string]bool{"/internal/official/refresh": true}
	// "/static" (added 2026-10-29, Phase 5 follow up to plan 05-13, D-05): the
	// stylesheets, scripts and icons. A browser fetches them with no cookie on a
	// first visit to the Demo or the sign in page, and each fetch used to set a
	// cookie and write a sessions row (12 rows for one Demo visit). The files are
	// public and read nothing from a session.
	// "/r" (added 2026-10-29, Phase 8 plan 04, D-05): the public status page of a
	// shared link. WhatsApp's crawler and a reader with no account fetch it with
	// no cookie, and the page needs no session, so neither a cookie nor a sessions
	// row is written (research Pitfall 2). Only GET /r/{token} is a route; a POST
	// under the prefix is skipped too and is answered 405.
	return sessionSkip{get: get, post: post, prefixes: []string{"/demo", "/static", "/r"}}
}

// skipSessionFor wraps the session middleware so a request goes straight to the
// next handler, without a cookie and without a sessions row, only when the rule
// set says so (see sessionSkip.matches). Every other request goes through the
// session middleware as before.
func skipSessionFor(skip sessionSkip, next func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		withSession := next(h)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip.matches(r) {
				h.ServeHTTP(w, r)
				return
			}
			withSession.ServeHTTP(w, r)
		})
	}
}

// persistSession is the session Manager's persist callback: it writes the
// sessions row eagerly on first visit, per FOUND-01 ("issued to a
// first-time visitor") and Open Question 1's recommendation, so Phase 2's
// reputation work can read a session's full history rather than only its
// submissions.
func (d Deps) persistSession(ctx context.Context, id string) error {
	return d.Sessions.UpsertSession(ctx, id)
}
