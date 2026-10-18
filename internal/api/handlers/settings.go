package handlers

import (
	"html/template"
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// settingsTemplateName is the file executed by SettingsPage's ExecuteTemplate
// call.
const settingsTemplateName = "settings.html.tmpl"

// settingsViewModel is exactly what settings.html.tmpl reads.
type settingsViewModel struct {
	AssetVersion string
	Chrome       Chrome
	// AccountID is the verified account's numeric id, rendered as the
	// data-account-id attribute of the page. It is never the email.
	AccountID int64
	// Nav is the data of the two app bars. Settings is not a main place, so no
	// item is current; the phone top bar shows the title Settings and a back
	// arrow whose address is the constant "/" (D-11, T-34-20).
	Nav NavView
	// Profile is the account's own name and home area as the service returns
	// them. The two strings below are what About you draws from it.
	Profile service.AccountProfile
	// DisplayName and HomeAreaName are the stored name and the stored home area
	// place name as plain strings, empty when unset (D-24b). The template
	// shows "Not set" for an empty one and html/template escapes both (T-34-26).
	// The home area's coarse cell is never read into the page.
	DisplayName  string
	HomeAreaName string
	// Legal is the text of the in app reader window opened from the About
	// Pinalert rows (D-31). It holds nothing about the account (R-07).
	Legal LegalText
	// MutedCount is how many reports the account muted. It is drawn by plan 12's
	// Muted reports row in the Notifications section (Phase 3.2).
	MutedCount int
}

// SettingsPage handles GET /settings: the one Settings page (Phase 3.4, D-25,
// D-26). It shows only rows whose feature exists, About Pinalert on both sizes
// and the Account section on the phone. Later phases add their rows here.
//
// Like Profile it is only reached through the gated r.Group, so a missing
// account in the context is a routing bug, logged and answered with a 500, and
// the response is no-store. A failed read of the account's own profile is also a
// 500: the page would otherwise render rows that look empty when they are not.
//
// Deliberately carries no swag annotation: it serves text/html and lives
// outside @BasePath /api.
func SettingsPage(tmpl *template.Template, accounts *service.AccountService, mutes *service.MuteService, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: SettingsPage: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		profile, err := accounts.Profile(r.Context(), acc.ID)
		if err != nil {
			log.Printf("handlers: SettingsPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// The muted count for the People and reports row. A failed read is a
		// logged 500 like the profile read: a row that reads 0 when it is not
		// would hide a mute the person can no longer find.
		mutedCount := 0
		if mutes != nil {
			mutedCount, err = mutes.MutedCount(r.Context(), acc.ID)
			if err != nil {
				log.Printf("handlers: SettingsPage: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		chrome := cfg.Chrome.withDefaults()
		vm := settingsViewModel{
			MutedCount:   mutedCount,
			AssetVersion: cfg.AssetVersion,
			Chrome:       chrome,
			AccountID:    acc.ID,
			Nav:          navFor(NavSettings, navTitle("Settings"), navBack("/")),
			Profile:      profile,
			Legal:        legalTextFor(chrome),
		}
		if profile.DisplayName != nil {
			vm.DisplayName = *profile.DisplayName
		}
		if profile.HomeAreaName != nil {
			vm.HomeAreaName = *profile.HomeAreaName
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, settingsTemplateName, vm); err != nil {
			log.Printf("handlers: SettingsPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
