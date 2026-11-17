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
	// Quick 261113-t4y (owner decision 2026-11-13): the About you section, with the
	// Username, Name and Home area rows and their windows, moved to your profile
	// page behind Edit profile (PersonPage reads the name, the home area and the
	// rename wait there), so this page no longer reads or draws any of them.

	// Legal is the text of the in app reader window opened from the About
	// Pinalert rows (D-31). It holds nothing about the account (R-07).
	Legal LegalText
	// MutedCount is how many reports the account muted. It is drawn by plan 12's
	// Muted reports row in the Notifications section (Phase 3.2).
	MutedCount int
	// AlertRadiusKm, HasHomeArea and AlertRadii draw the distance row of the
	// Notifications box (quick 261103-jal, owner decision 2026-11-03): the
	// distance picker over AlertRadii, and the "Add a home area first" button
	// when HasHomeArea is false. The home area cell never enters the page, only
	// this boolean. The four switches that used to sit above it are gone: the
	// device switch is the only way to stop phone notes.
	AlertRadiusKm int
	HasHomeArea   bool
	AlertRadii    []int
	// PushPublicKey is the VAPID public key. The device row renders it as
	// data-push-key. The key is public by design; the home cell and every
	// coordinate stay out of this model (T-03.2-14).
	PushPublicKey string
	// ProfilePublic, FollowersCount, BlockedCount and MyUsername are Phase 9 plan
	// 12 (D-04): the Public profile switch state, the Followers count the switch
	// reads before it is turned off, the Blocked people count and the caller's own
	// username, which the list windows need to read your own lists. All are the
	// caller's own; nothing about anyone else enters this model. html/template
	// escapes the username. Amended 2026-11-17 (quick 261117-lq0): the Following
	// and Followers rows left Settings (both are on your profile), so the
	// Following count is gone and the Followers count is drawn only as the
	// data-followers attribute the switch reads.
	ProfilePublic  bool
	FollowersCount int
	BlockedCount   int
	MyUsername     string
	// MutedPeopleCount is how many people the caller muted (Phase 9 plan 20, D-12),
	// for the Muted people row. It can exceed the list by the people whose new
	// username is still pending, as the Blocked people count does.
	MutedPeopleCount int
}

// SettingsPage handles GET /settings: the one Settings page (Phase 3.4, D-25,
// D-26). It shows only rows whose feature exists, About Pinalert on both sizes
// and the Account section on the phone. Later phases add their rows here.
//
// Like Profile it is only reached through the gated r.Group, so a missing
// account in the context is a routing bug, logged and answered with a 500, and
// the response is no-store. A failed read of the muted count, the people rows or the
// alert distance is also a 500: the page would otherwise render rows that look
// empty when they are not.
//
// Deliberately carries no swag annotation: it serves text/html and lives
// outside @BasePath /api.
func SettingsPage(tmpl *template.Template, accounts *service.AccountService, mutes *service.MuteService, people *service.PeopleService, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: SettingsPage: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		var err error

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

		// The people rows (Phase 9 plan 12, D-04): the switch state, the caller's own
		// username and the three counts. A failed read is a logged 500 like the muted
		// count: a switch drawn off when it is on would hide that others can follow.
		var own service.OwnProfileState
		var counts service.PeopleCounts
		mutedPeopleCount := 0
		if people != nil {
			own, err = people.MyProfileState(r.Context(), acc.ID)
			if err == nil {
				counts, err = people.Counts(r.Context(), acc.ID)
			}
			if err == nil {
				// Phase 9 plan 20 (D-12): the Muted people count, read the same way.
				mutedPeopleCount, err = people.MutedPeopleCount(r.Context(), acc.ID)
			}
			if err != nil {
				log.Printf("handlers: SettingsPage: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		// The alert distance. A failed read is a logged 500 like the profile read.
		alerts, err := accounts.AlertSettings(r.Context(), acc.ID)
		if err != nil {
			log.Printf("handlers: SettingsPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		chrome := cfg.Chrome.withDefaults()
		vm := settingsViewModel{
			AlertRadiusKm:    alerts.AlertRadiusKm,
			HasHomeArea:      alerts.HasHomeArea,
			AlertRadii:       service.AlertRadii,
			MutedCount:       mutedCount,
			ProfilePublic:    own.ProfilePublic,
			MyUsername:       own.Username,
			FollowersCount:   counts.Followers,
			BlockedCount:     counts.Blocked,
			MutedPeopleCount: mutedPeopleCount,
			PushPublicKey:    cfg.PushPublicKey,
			AssetVersion:     cfg.AssetVersion,
			Chrome:           chrome,
			AccountID:        acc.ID,
			Nav:              navFor(NavSettings, navTitle("Settings"), navBack("/")),
			Legal:            legalTextFor(chrome),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, settingsTemplateName, vm); err != nil {
			log.Printf("handlers: SettingsPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
