package handlers

import (
	"bytes"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"pinalert/internal/account"
	"pinalert/internal/service"
)

// personTemplateName is the file executed by PersonPage's ExecuteTemplate call.
const personTemplateName = "person.html.tmpl"

// navPerson is the key of the profile page in the app bars. Like Settings it is
// not a main place, so no bottom bar item is current; unlike Settings the phone
// top bar keeps its gear (UI-SPEC 1a), so it must not reuse the Settings key,
// which hides the gear.
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
	Nav          NavView
	Profile      service.PersonProfile
	NotFound     bool
	BlockedByYou bool
	PageTitle    string

	// LoadFailed draws the "Couldn't load this profile" state when the profile
	// read failed (status 500). RetryHref is the escaped /u/ address the Try
	// again link returns to.
	LoadFailed bool
	RetryHref  string

	// AccountID is the READER's own account id, never the profile owner's. It
	// is written as data-account-id so position.js can return the position
	// Nearby kept for this account (plan 09-11); it is zero on the not found
	// page, which carries nothing about the request.
	AccountID int64

	// The Recent reports section (plan 09-11, D-06, D-07). ReportsFailed draws
	// the error state; an empty Reports draws the empty state.
	Reports        []personReportRow
	ReportsCut     bool
	ReportsCutText string
	ReportsFailed  bool

	// UsernameWaitUntil is the moment this account's rename wait ends, as an RFC
	// 3339 UTC time, and empty outside the 7 day gap and on anyone else's page. It is
	// written into the Username window of your own page (data-wait-until), which
	// moved here from Settings. Quick 261113-t4y.
	UsernameWaitUntil string
}

// personReportRow is one row of Recent reports, written by the server like a My
// activity row. A live row has an Href (the Phase 8 report link) and the exact
// point the map already shows. A finished row has no Href and carries only the
// coarse cell centre in Lat and Lon (Approx true): the page never holds a
// finished report's exact point, its id or a time of day (D-20, T-09-40).
type personReportRow struct {
	Href             string
	Title            string
	LabelLine        string
	AreaLine         string
	StatusWord       string
	ChipClass        string
	WhenText         string
	DisputedLine     string
	IconGlyph        string
	SeverityClass    string
	Lat              float64
	Lon              float64
	Approx           bool
	AccessiblePrefix string
}

// personReportHref is the Phase 8 report link of a live report: the same
// /?report=ID address a push note and a shared link use. It opens the popup on
// a laptop and the report page on a phone (A-P8-1).
func personReportHref(id int64) string {
	return "/?report=" + strconv.FormatInt(id, 10)
}

// personReportRows turns the service rows into page rows. now is the instant
// the relative time of a live row is measured from.
func personReportRows(rows []service.ProfileReportRow, now time.Time) []personReportRow {
	out := make([]personReportRow, 0, len(rows))
	for _, row := range rows {
		v := row.View
		pr := personReportRow{
			StatusWord:   row.StatusWord,
			IconGlyph:    "icon-glyph--" + string(v.Category),
			Lat:          v.Latitude,
			Lon:          v.Longitude,
			DisputedLine: row.DisputedLine,
		}
		if row.Live {
			pr.Href = personReportHref(v.ID)
			pr.Title = v.Description
			pr.LabelLine = service.CategoryWord(v.Category) + " · " + service.SeverityWord(v.Severity)
			pr.WhenText = "Posted " + profileRelativeTime(v.CreatedAt, now)
			pr.SeverityClass = "sev-" + string(v.Severity)
			switch row.Status {
			case service.ProfileStatusDisputed:
				pr.ChipClass = "person-chip--grey"
				pr.AccessiblePrefix = "Disputed, may not be true."
			case service.ProfileStatusNotConfirmed:
				pr.ChipClass = "person-chip--neutral"
				pr.AccessiblePrefix = "Not confirmed yet."
			default:
				pr.ChipClass = "person-chip--" + pr.SeverityClass
			}
		} else {
			pr.Title = row.FinishedTitle
			pr.AreaLine = row.AreaLine
			pr.WhenText = "Ended " + row.EndedOn
			pr.Approx = true
			pr.AccessiblePrefix = "Ended."
			if row.Status == service.ProfileStatusResolved {
				pr.ChipClass = "person-chip--low"
			} else {
				pr.ChipClass = "person-chip--grey"
			}
		}
		out = append(out, pr)
	}
	return out
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
func PersonPage(tmpl *template.Template, people *service.PeopleService, reports *service.ReportService, accounts *service.AccountService, cfg PageConfig) http.HandlerFunc {
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
			// UI-SPEC Surface 6 (P9-C12): a failed read is a framed state with a
			// Try again link, not a bare line. The box holds no profile and no
			// stored text; the link goes back to the same address.
			log.Printf("handlers: PersonPage: %v", err)
			status = http.StatusInternalServerError
			vm.LoadFailed = true
			vm.RetryHref = "/u/" + url.PathEscape(chi.URLParam(r, "username"))
			vm.PageTitle = "Pinalert: Couldn't load this profile"
			vm.Nav = navFor(navPerson, navTitle(personTitleOther), navBack("/"))
		default:
			vm.Profile = profile
			vm.BlockedByYou = profile.BlockedByYou
			vm.PageTitle = "Pinalert: " + profile.Username
			title := personTitleOther
			if profile.IsSelf {
				title = personTitleOwn
			}
			vm.Nav = navFor(navPerson, navTitle(title), navBack("/"))
			vm.AccountID = acc.ID

			// Quick 261113-t4y (owner decision 2026-11-13): your own page holds the
			// Username window, so it needs the end of the rename wait. A failed read
			// is logged and leaves the window drawn as usual; the server still
			// decides on save.
			if profile.IsSelf && accounts != nil {
				own, err := accounts.Profile(r.Context(), acc.ID)
				if err != nil {
					log.Printf("handlers: PersonPage: reading the rename wait of account %d: %v", acc.ID, err)
				} else {
					vm.UsernameWaitUntil = accounts.UsernameWaitUntil(own)
				}
			}

			// Recent reports (plan 09-11): drawn by the server like My activity,
			// for a profile the viewer may see and has not blocked. The read has
			// no centre: distances are filled in the browser from the position
			// Nearby kept, so the server learns no reader location. A failed read
			// is logged with ids only and the page still renders.
			if !profile.BlockedByYou {
				if reports == nil {
					vm.ReportsFailed = true
				} else {
					list, err := reports.ProfileReports(r.Context(), service.ProfileQuery{
						AccountID:       profile.AccountID(),
						ViewerAccountID: acc.ID,
					})
					if err != nil {
						log.Printf("handlers: PersonPage: reading reports of account %d for viewer %d: %v", profile.AccountID(), acc.ID, err)
						vm.ReportsFailed = true
					} else {
						vm.Reports = personReportRows(list.Rows, time.Now().UTC())
						vm.ReportsCut = list.Cut
						vm.ReportsCutText = list.CutText
					}
				}
			}
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
// account with no username has no profile page yet, so it goes to Settings. The
// onboarding gate sends such an account to /welcome first, where step 4 sets the
// username, so this is only a defensive answer: since quick 261113-t4y (owner
// decision 2026-11-13) the username is edited on the profile page, not in
// Settings.
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
