package handlers

// demo_page.go is the Demo's Nearby page (plan 05-13, D-01, D-02, D-05,
// ROBUST-06, ROBUST-07). GET /demo starts a visit: it creates one fresh in
// memory world, names it in the page (the API base is the visit's own path,
// /demo/api/v/<visit>), and renders demo.html.tmpl, so the real map, list and
// vote scripts run on the Vijayawada sample with no account.
//
// Nothing is stored. The handler reads no session and no account, writes no
// row and sets no cookie (the /demo prefix skips the session middleware, see
// sessionSkipSet in internal/api). Opening the page again, or reloading it,
// starts a new world, so nothing a visitor did survives the visit. The page
// asks the browser for no location: the pretend position is on the page shell
// and the scripts use it instead.
//
// Plan 05-19 added the Demo's other two places (Official alerts and My
// activity). They share this file's view model and builders. The bars of the
// three pages link to each other with ?v=<visit>, and a page given a valid,
// living visit shows that world again, so the three places of one visit agree.
// A page removes the v from its own address as it loads (demo.js), so a reload
// or a copied address has no visit and starts a fresh Demo.

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"pinalert/internal/demo"
	"pinalert/internal/ratelimit"
)

// demoPageTemplate is the template DemoPage executes.
const demoPageTemplate = "demo.html.tmpl"

// demoBusyMessage is the plain page sentence for a Demo that cannot start a
// visit right now (the world could not be made).
const demoBusyMessage = "The Demo is busy. Try again in a minute."

// demoTooManyVisitsMessage is the plain page sentence for a visitor address
// that has started too many visits in a short time.
const demoTooManyVisitsMessage = "Too many visits. Try again in a minute."

// demoWordmark is the phone top bar's title on the Demo's Nearby: the wordmark
// and nothing else (UI-SPEC 4c). The real Nearby shows Search a place there, so
// its title is left out; the Demo has no search, so the bar draws the wordmark.
const demoWordmark = "Pinalert"

// demoFromWelcome is the one value of the from parameter that sends Exit demo
// to the first-time flow (UI-SPEC 4a). Every other value, or none, sends it to
// the sign in page.
const demoFromWelcome = "welcome"

// demoViewModel is exactly what demo.html.tmpl and demo_strip.html.tmpl read.
type demoViewModel struct {
	AssetVersion string
	// Chrome is the shared bar data. The shared report form reads the Demo
	// field below (plan 05-17).
	Chrome Chrome
	// Demo is true on this page only. The shared report form reads it and
	// leaves out the Call 112 box and the What is happening picks (plan 05-17,
	// T-05-91).
	Demo bool
	// Nav is the data of the two app bars, built by demoNavFor: the Demo's
	// places, and no search, bell, gear or person menu.
	Nav NavView
	// Visit is the id of this visit's world. It is also inside APIBase.
	Visit string
	// APIBase is the path the page's scripts send report reads, posts and
	// votes to, read by api-base.js from data-api-base.
	APIBase string
	// ExitHref is where Exit demo goes: /welcome or /login, nothing else.
	ExitHref string
	// AlertsHref is the address of the Demo's Official alerts for this visit,
	// the target of the Nearby banner.
	AlertsHref string
	// FixedLat and FixedLon are the pretend position (Singh Nagar). The map,
	// the report window and the vote path use it instead of the browser's.
	FixedLat, FixedLon float64
	// PretendPlace names the pretend position for the line under the map.
	PretendPlace string
	// DefaultRadiusKm and VoteRadiusKm are the page configuration of the real
	// Nearby page: the feed radius and the distance rule, so the Demo shows
	// the real numbers.
	DefaultRadiusKm float64
	VoteRadiusKm    float64
}

// demoExitHref answers where Exit demo goes. from is the page's from query
// parameter. Only the exact word welcome (the visitor came from the first-time
// flow) goes to /welcome; anything else, including a free address, goes to
// /login. The result is always one of these two constants (T-05-69).
func demoExitHref(from string) string {
	if from == demoFromWelcome {
		return "/welcome"
	}
	return "/login"
}

// demoFromWord is the from query value to carry on: demoFromWelcome when the
// address says exactly that, else empty. Nothing else is ever carried on.
func demoFromWord(r *http.Request) string {
	if r.URL.Query().Get(demoFromParam) == demoFromWelcome {
		return demoFromWelcome
	}
	return ""
}

// demoVisitFor finds the world a Demo page is for. A valid, living visit named
// by ?v= is reused, so the three places of one visit show one world; a missing,
// malformed, unknown or ended visit starts a new world, as a reload does. The
// shape is checked before the lookup (T-05-102), and a reused world costs no
// new memory (T-05-101).
func demoVisitFor(store *demo.Store, r *http.Request) (*demo.World, error) {
	if visit := r.URL.Query().Get(demoVisitParam); demo.ValidVisitID(visit) {
		if world, ok := store.Get(visit); ok {
			return world, nil
		}
	}
	// Only a new world spends the per address visit limit (WR-S06 of the Phase 5
	// review): moving between the places of a living visit is never refused.
	// The limit fails closed (NP-3 of the review fix round): a request whose
	// context carries no charge, because its page was served without
	// demoVisitLimiter in front of it, is refused, never given a free world.
	if charge, ok := r.Context().Value(demoVisitChargeKey{}).(func() bool); !ok || !charge() {
		return nil, errDemoTooManyVisits
	}
	return store.Create()
}

// demoVisitChargeKey holds, in a request's context, the function that spends
// one token of the visit limit for the request's address.
type demoVisitChargeKey struct{}

// errDemoTooManyVisits is a new world refused by the visit limit.
var errDemoTooManyVisits = errors.New("handlers: too many new Demo visits from one address")

// newDemoPlaceModel builds the view model shared by the Demo's pages for the
// place current (one of the Nav constants): the visit's world, the Demo bars
// whose places carry the visit, the Exit address and the pretend position. When
// the world cannot be made it answers 503 itself and returns false.
func newDemoPlaceModel(w http.ResponseWriter, r *http.Request, store *demo.Store, cfg PageConfig, current string, opts ...navOption) (demoViewModel, *demo.World, bool) {
	world, err := demoVisitFor(store, r)
	if err != nil {
		if errors.Is(err, errDemoTooManyVisits) {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, demoTooManyVisitsMessage, http.StatusTooManyRequests)
			return demoViewModel{}, nil, false
		}
		log.Printf("handlers: Demo page: starting a visit failed: %v", err)
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, demoBusyMessage, http.StatusServiceUnavailable)
		return demoViewModel{}, nil, false
	}
	from := demoFromWord(r)
	chrome := cfg.Chrome.withDefaults()
	return demoViewModel{
		AssetVersion:    cfg.AssetVersion,
		Chrome:          chrome,
		Demo:            true,
		Nav:             demoNavFor(current, world.ID, from, opts...),
		Visit:           world.ID,
		APIBase:         "/demo/api/v/" + world.ID,
		ExitHref:        demoExitHref(r.URL.Query().Get(demoFromParam)),
		AlertsHref:      demoPlaceHref(demoNavHrefs[NavAlerts], world.ID, from),
		FixedLat:        demo.PretendLat,
		FixedLon:        demo.PretendLon,
		PretendPlace:    demo.PretendPlace,
		DefaultRadiusKm: cfg.DefaultRadiusKm,
		VoteRadiusKm:    cfg.VoteRadiusKm,
	}, world, true
}

// serveDemoPlace renders one Demo page from a view model. The page is built in
// full first, so a template error answers a clean 500 and never a half page.
// The page names a visit that exists once, so it is never cached, and a link
// out of the Demo must not carry the address (and so the visit id) along.
func serveDemoPlace(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	var page bytes.Buffer
	if err := tmpl.ExecuteTemplate(&page, name, data); err != nil {
		log.Printf("handlers: Demo page %s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	_, _ = w.Write(page.Bytes())
}

// DemoPage renders the Demo's Nearby page. store holds the worlds; cfg is the
// page configuration of the real Nearby page. A valid, living ?v= names the
// visit to show again (the Demo's other places link back with it); anything
// else starts a new world, so a reload or a shared address starts fresh. It is
// reached through DemoRoutes, behind the per IP visit limit.
func DemoPage(tmpl *template.Template, store *demo.Store, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vm, _, ok := newDemoPlaceModel(w, r, store, cfg, NavNearby, navTitle(demoWordmark))
		if !ok {
			return
		}
		serveDemoPlace(w, tmpl, demoPageTemplate, vm)
	}
}

// demoVisitLimiter limits how often one address may start a visit. It charges
// nothing itself: it hands the page a function that spends one token, and the
// page calls it only when it starts a new world, then answers 429 as a plain
// page sentence, because the response is for a page, not the JSON API. The key is the client address the router found, else the peer
// address, as ratelimit's own middleware reads it.
func demoVisitLimiter(limit DemoLimit) func(http.Handler) http.Handler {
	perIP := ratelimit.NewPerIP(limit.Every, limit.Burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := middleware.GetClientIP(r.Context())
			if key == "" {
				if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
					key = host
				} else {
					key = r.RemoteAddr
				}
			}
			charge := func() bool { return perIP.Allow(key) }
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), demoVisitChargeKey{}, charge)))
		})
	}
}
