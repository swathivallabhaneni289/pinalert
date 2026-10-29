package handlers

import (
	"bytes"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"pinalert/internal/account"
	"pinalert/internal/service"
)

// personTemplateName is the file executed by PersonPage's ExecuteTemplate call.
const personTemplateName = "person.html.tmpl"

// navPerson is the key of the profile page in the app bars. Like Settings it is
// not a main place, so no bottom bar item is current; unlike Settings the phone
// top bar keeps its gear (UI-SPEC 1a), so it must not be NavSettings, which
// hides the gear.
const navPerson = "person"

// The phone top bar titles (P9-C2, UI-SPEC 1a).
const (
	personTitleOwn   = "My profile"
	personTitleOther = "Profile"
)

// personViewModel is exactly what person.html.tmpl reads (Phase 9 plan 08).
//
// Profile is the same server written profile the JSON route returns, so the
// page and the script never disagree. NotFound draws the one uniform not found
// box and nothing from the request: every unknown, private, pending, deleted
// and blocking profile renders the same bytes for one viewer (research
// Pitfall 9, T-09-30). BlockedByYou draws only the You blocked box.
type personViewModel struct {
	AssetVersion string
	Chrome       Chrome
	Legal        LegalText
	Nav          NavView
	Profile      service.PersonProfile
	NotFound     bool
	BlockedByYou bool
	PageTitle    string
}

// personPageHeaders sets the headers every answer of the profile routes carries:
// the page is for the signed in person only, so no shared cache keeps it and no
// search engine lists it (T-09-31).
func personPageHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
}

// PersonPage handles GET /u/{username}: one profile page for your own profile
// and for anyone else's (D-04, UI-SPEC Surface 1).
//
// Like SettingsPage it is only reached through the gated r.Group, so a missing
// account in the context is a routing bug, logged and answered with a 500.
// ErrPersonNotFound renders the not found state with status 404 and the same
// body for every cause. A person you blocked renders the You blocked state with
// status 200. The page is rendered into a buffer first, so a template failure
// is a clean 500 and never a half written 404.
//
// Deliberately carries no swag annotation: it serves text/html and lives
// outside @BasePath /api.
func PersonPage(tmpl *template.Template, people *service.PeopleService, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: PersonPage: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		chrome := cfg.Chrome.withDefaults()
		vm := personViewModel{
			AssetVersion: cfg.AssetVersion,
			Chrome:       chrome,
			Legal:        legalTextFor(chrome),
		}
		status := http.StatusOK

		profile, err := people.Profile(r.Context(), acc.ID, chi.URLParam(r, "username"))
		switch {
		case errors.Is(err, service.ErrPersonNotFound):
			status = http.StatusNotFound
			vm.NotFound = true
			vm.PageTitle = "Pinalert: Profile not found"
			vm.Nav = navFor(navPerson, navTitle(personTitleOther), navBack("/"))
		case err != nil:
			log.Printf("handlers: PersonPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		default:
			vm.Profile = profile
			vm.BlockedByYou = profile.BlockedByYou
			vm.PageTitle = "Pinalert: " + profile.Username
			title := personTitleOther
			if profile.IsSelf {
				title = personTitleOwn
			}
			vm.Nav = navFor(navPerson, navTitle(title), navBack("/"))
		}

		var page bytes.Buffer
		if err := tmpl.ExecuteTemplate(&page, personTemplateName, vm); err != nil {
			log.Printf("handlers: PersonPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		personPageHeaders(w)
		w.WriteHeader(status)
		if _, err := w.Write(page.Bytes()); err != nil {
			log.Printf("handlers: PersonPage: writing the page: %v", err)
		}
	}
}

// MyProfilePage handles GET /me: a 303 to /u/{your username}, so the person
// menu and the phone Settings row need no username (UI-SPEC Surface 1). An
// account with no username has no profile page yet, so it goes to Settings,
// where the username is set.
//
// Deliberately carries no swag annotation: it is a page redirect, not API.
func MyProfilePage(people *service.PeopleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: MyProfilePage: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		personPageHeaders(w)
		username, err := people.MyUsername(r.Context(), acc.ID)
		switch {
		case errors.Is(err, service.ErrPersonNotFound):
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
		case err != nil:
			log.Printf("handlers: MyProfilePage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		default:
			http.Redirect(w, r, "/u/"+url.PathEscape(username), http.StatusSeeOther)
		}
	}
}
