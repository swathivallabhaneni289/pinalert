package handlers

// demo_official.go is the Demo's Official alerts place (plan 05-19, D-05, D-06,
// ROBUST-06, ROBUST-07). The page is the shell of the real Official alerts page
// with the Demo strip, and the same renderer (official-alerts.js) fills it from
// the visit's own sample route, GET /demo/api/v/{visit}/official/alerts. That
// route answers in exactly the shape of GET /api/official/alerts, because it
// runs the sample alerts (internal/demo SampleView) through the same view and
// the same conversion (officialViewToResponse) as the real route does. Every
// row is a Demo row with no authority badge, and the banner carries the Demo
// flag, so nothing here can be read as an official alert.
//
// Nothing is stored. Neither handler reads a session or an account, writes a row
// or sets a cookie, and the position in the query is ignored: the reader is
// always the pretend position.

import (
	"html/template"
	"net/http"

	"pinalert/internal/demo"
)

// demoAlertsTemplate is the template DemoAlertsPage executes.
const demoAlertsTemplate = "demo_alerts.html.tmpl"

// DemoAlertsPage handles GET /demo/alerts: the Demo's Official alerts place for
// the visit named by ?v= when that visit is valid and alive, else for a new
// world (as a reload of /demo would). It is reached through DemoRoutes, behind
// the per IP visit limit.
func DemoAlertsPage(tmpl *template.Template, store *demo.Store, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vm, _, ok := newDemoPlaceModel(w, r, store, cfg, NavAlerts)
		if !ok {
			return
		}
		serveDemoPlace(w, tmpl, demoAlertsTemplate, vm)
	}
}

// demoOfficialAlerts answers GET /demo/api/v/{visit}/official/alerts: the
// sample alerts as the real route would answer them. An unknown, malformed or
// ended visit is the Demo's 404 with field visit.
func demoOfficialAlerts(store *demo.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		world, ok := demoWorld(store, w, r)
		if !ok {
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, officialViewToResponse(demo.SampleView(world, world.Now())))
	}
}
