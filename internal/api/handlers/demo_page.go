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

import (
	"bytes"
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
	// Chrome is the shared bar data. DemoPage is true on it, so the shared
	// report form leaves out the Call 112 box and the What is happening picks.
	Chrome Chrome
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

// DemoPage renders the Demo's Nearby page for a new visit. store makes the
// world; cfg is the page configuration of the real Nearby page. It is reached
// through DemoRoutes, behind the per IP visit limit.
func DemoPage(tmpl *template.Template, store *demo.Store, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		world, err := store.Create()
		if err != nil {
			log.Printf("handlers: DemoPage: starting a visit failed: %v", err)
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, demoBusyMessage, http.StatusServiceUnavailable)
			return
		}
		chrome := cfg.Chrome.withDefaults()
		chrome.DemoPage = true
		vm := demoViewModel{
			AssetVersion:    cfg.AssetVersion,
			Chrome:          chrome,
			Nav:             demoNavFor(NavNearby, navTitle(demoWordmark)),
			Visit:           world.ID,
			APIBase:         "/demo/api/v/" + world.ID,
			ExitHref:        demoExitHref(r.URL.Query().Get("from")),
			FixedLat:        demo.PretendLat,
			FixedLon:        demo.PretendLon,
			PretendPlace:    demo.PretendPlace,
			DefaultRadiusKm: cfg.DefaultRadiusKm,
			VoteRadiusKm:    cfg.VoteRadiusKm,
		}
		// The page is built in full first, so a template error answers a clean
		// 500 and never a half page.
		var page bytes.Buffer
		if err := tmpl.ExecuteTemplate(&page, demoPageTemplate, vm); err != nil {
			log.Printf("handlers: DemoPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The page names a visit that exists once: never cached.
		w.Header().Set("Cache-Control", "no-store")
		// The visit id is in the page; a link out of the Demo must not carry
		// the address (and so the id) along.
		w.Header().Set("Referrer-Policy", "same-origin")
		_, _ = w.Write(page.Bytes())
	}
}

// demoVisitLimiter limits how often one address may start a visit. It answers
// 429 as a plain page sentence, because the response is for a page, not the
// JSON API. The key is the client address the router found, else the peer
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
			if !perIP.Allow(key) {
				w.Header().Set("Cache-Control", "no-store")
				http.Error(w, demoTooManyVisitsMessage, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
