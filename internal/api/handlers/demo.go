// demo.go is the Demo's sandbox API (plan 05-10, D-01, D-04, D-05). It is
// mounted at /demo outside the sign in gate, so a visitor with no account can try
// Pinalert before posting anything real.
//
// Each visit is one in memory world from internal/demo, found by the visit id in
// the path (/demo/api/v/<visit>/...). The world runs the real report and voting
// services over an in memory twin of the store, so the rules are the real ones:
// the 1 km vote distance, no vote on your own report, expiry, the two undo limit,
// the text prefilter, and the same trust object. Every error answer is written by
// the same functions the real routes use (writeCastVoteError, writeSubmitError),
// and the feed is the same handler (nearbyHandler), so the JSON shapes are the
// real ones and the real map, feed, vote and trust scripts work unchanged with a
// different API base.
//
// Nothing is written to the database. The handlers read no session and no
// account (the visitor is the world's own synthetic account), the /demo prefix
// skips the session middleware (sessionSkipSet in internal/api), and no answer
// sets a cookie. The practice text goes to no vendor, because the world's report
// service has no screener, and no phone alert is queued.
//
// These routes carry no swag annotation: they answer in the shapes already
// documented for GET /api/reports, POST /api/reports and the five vote routes,
// and they are a sandbox, not part of the published API.
//
// Flag and Mute have no Demo route and are hidden in the Demo (decided in plan
// 05-10, pinned by TestDemoNoFlagOrMuteRoutes): Settings, the bell and phone
// alerts are not live there, so there is nothing for them to act on. The vote
// routes are registered one by one for the five words below, so any other word
// under a report is an ordinary 404.
package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/demo"
	"pinalert/internal/ratelimit"
	"pinalert/internal/service"
)

// demoEndedMessage is the 404 sentence for a visit that is unknown, malformed or
// idle for too long.
const demoEndedMessage = "This Demo has ended. Reload the page to start again."

// demoFullMessage is the 429 sentence for a world that holds all the practice
// reports or votes it may.
const demoFullMessage = "This Demo is full. Reload the page to start again."

// DemoLimit is one per IP token bucket: Burst requests at once, then one more
// every Every.
type DemoLimit struct {
	Every time.Duration
	Burst int
}

// DemoConfig configures DemoRoutes. Store holds the worlds. VisitLimit is the
// per IP limit on starting a visit (a new world), read by the Demo page (plan
// 05-13). WriteLimit is the per IP limit on the Demo's POST routes. A zero
// limit takes its default below. Template and Page are what the Demo page
// renders with: the parsed page templates and the same page configuration the
// real Nearby page uses (the fallback centre, the radii, the asset version). A
// nil Template mounts no page, so a router built for the API alone, as the
// plan 05-10 tests do, keeps answering 404 on /demo.
type DemoConfig struct {
	Store      *demo.Store
	VisitLimit DemoLimit
	WriteLimit DemoLimit
	Template   *template.Template
	Page       PageConfig
}

// The defaults are generous because carrier NAT puts many phones behind one
// address. The real defence of memory is the store's world cap and each world's
// report and vote caps, not the rate.
var (
	DemoVisitLimitDefault = DemoLimit{Every: 6 * time.Second, Burst: 30}
	DemoWriteLimitDefault = DemoLimit{Every: time.Second, Burst: 60}
)

// demoVoteRoutes are the five vote words and what each means, the same pairs the
// real router registers. They are registered one by one, never as a wildcard.
var demoVoteRoutes = []struct {
	action string
	kind   service.VoteKind
	value  service.VoteValue
}{
	{"confirm", service.VoteKindContent, service.VoteConfirm},
	{"dispute", service.VoteKindContent, service.VoteDispute},
	{"resolve", service.VoteKindResolution, service.VoteResolve},
	{"reopen", service.VoteKindResolution, service.VoteReopen},
	{"unresolve", service.VoteKindResolution, service.VoteUnresolve},
}

// DemoRoutes builds the Demo API router, to be mounted at /demo:
//
//	GET  /api/v/{visit}/reports
//	POST /api/v/{visit}/reports
//	POST /api/v/{visit}/reports/{id}/confirm | dispute | resolve | reopen | unresolve
//
// The POST routes sit behind the per IP write limiter. Reads are not limited: a
// page polls while a practice report climbs.
func DemoRoutes(cfg DemoConfig) chi.Router {
	store := cfg.Store
	if store == nil {
		store = demo.NewStore(demo.WithWorldSetup(demo.SeedSetup))
	}
	write := cfg.WriteLimit
	if write == (DemoLimit{}) {
		write = DemoWriteLimitDefault
	}
	limiter := ratelimit.NewPerIP(write.Every, write.Burst)

	r := chi.NewRouter()
	// A Demo answer is about one visit's moving world: never cached.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, req)
		})
	})
	r.Get("/api/v/{visit}/reports", demoNearby(store))
	r.With(limiter.Middleware()).Post("/api/v/{visit}/reports", demoSubmit(store))
	for _, v := range demoVoteRoutes {
		r.With(limiter.Middleware()).Post("/api/v/{visit}/reports/{id}/"+v.action, demoVote(store, v.kind, v.value))
	}
	return r
}

// demoWorld finds the world of the request's visit. An unknown, malformed or idle
// visit writes the 404 itself and returns false.
func demoWorld(store *demo.Store, w http.ResponseWriter, r *http.Request) (*demo.World, bool) {
	world, ok := store.Get(chi.URLParam(r, "visit"))
	if !ok {
		writeFieldError(w, http.StatusNotFound, "visit", demoEndedMessage)
		return nil, false
	}
	return world, true
}

// demoVisitor is the viewer of every Demo read: the world's one synthetic
// account, never anything the request says.
func demoVisitor(*http.Request) (int64, bool) { return demo.VisitorAccountID, true }

// demoNearby answers GET /demo/api/v/{visit}/reports exactly as GET /api/reports
// does, from the world. The neighbour votes that have fallen due are cast first,
// so the answer shows the world as of now.
func demoNearby(store *demo.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		world, ok := demoWorld(store, w, r)
		if !ok {
			return
		}
		if err := world.ApplyDue(r.Context()); err != nil {
			log.Printf("handlers: demoNearby: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		nearbyHandler(world.Reports, demoVisitor, world.Nearby).ServeHTTP(w, r)
	}
}

// demoSubmit answers POST /demo/api/v/{visit}/reports like POST /api/reports:
// strict JSON, the real validation and text prefilter, 201 with the report. The
// practice report is posted as the visitor and the pretend neighbours are
// planned to confirm it. expected_account_id and the device queue belong to the
// signed in app and mean nothing here.
func demoSubmit(store *demo.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		world, ok := demoWorld(store, w, r)
		if !ok {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxSubmitBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req SubmitReportRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		res, err := world.PostPractice(r.Context(), submitInputFrom(req))
		if err != nil {
			if errors.Is(err, demo.ErrWorldFull) {
				writeFieldError(w, http.StatusTooManyRequests, "report", demoFullMessage)
				return
			}
			writeSubmitError(w, err)
			return
		}
		if res.Replayed {
			writeJSON(w, http.StatusOK, SubmitReportResponse{Report: reportToResponse(res)})
			return
		}
		writeJSON(w, http.StatusCreated, SubmitReportResponse{Report: reportToResponse(res)})
	}
}

// demoVote answers one of the five vote routes like the real one: the same body
// shape, the same refusals, the same response. The body is decoded strictly, but
// its coordinates are not used: a Demo visitor never shares a location, so every
// vote is cast from the one pretend position (demo.PretendLat, demo.PretendLon)
// and the real 1 km rule applies to it. That is why a report farther away answers
// with the real "too far" sentence.
func demoVote(store *demo.Store, kind service.VoteKind, value service.VoteValue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		world, ok := demoWorld(store, w, r)
		if !ok {
			return
		}
		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxVoteBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req CastVoteRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		res, err := world.Vote(r.Context(), service.CastVoteInput{
			ReportID:  reportID,
			Kind:      kind,
			Value:     value,
			Latitude:  demo.PretendLat,
			Longitude: demo.PretendLon,
		})
		if err != nil {
			if errors.Is(err, demo.ErrWorldFull) {
				writeFieldError(w, http.StatusTooManyRequests, "report", demoFullMessage)
				return
			}
			writeCastVoteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, castVoteResponse(res))
	}
}
