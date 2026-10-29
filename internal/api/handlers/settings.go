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
	// Username is the stored username as a plain string, empty when none is
	// saved (only an account that has not finished the fill in has none, and
	// the gate sends it to /welcome first). Added by Phase 7.1 plan 10 (D-07):
	// the Settings page shows it and offers the change window, drawn by plan
	// 15. html/template escapes it.
	Username string
	// Legal is the text of the in app reader window opened from the About
	// Pinalert rows (D-31). It holds nothing about the account (R-07).
	Legal LegalText
	// MutedCount is how many reports the account muted. It is drawn by plan 12's
	// Muted reports row in the Notifications section (Phase 3.2).
	MutedCount int
	// AlertsOn, AlertRadiusKm, HasHomeArea and AlertRadii are the Reliable
	// reporter alert of NOTIF-05 (Phase 3.2, plan 13). Plan 15 draws the two
	// Phone notifications rows from them: the switch, the distance picker over
	// AlertRadii, and the "Add a home area first." state when HasHomeArea is
	// false. The home area cell never enters the page, only this boolean.
	AlertsOn      bool
	AlertRadiusKm int
	HasHomeArea   bool
	AlertRadii    []int
	// CriticalPushOn and ChangesPushOn are the two phone switches of Phase 3.3
	// (PWA-03, PWA-06): Critical reports near the home area (on by default) and
	// changes to reports the person posted or voted on (off by default). PushPublicKey
	// is the VAPID public key. Plan 14 draws the rows from these and renders the key
	// as data-push-key. The key is public by design; the home cell and every
	// coordinate stay out of this model (T-03.2-14). There is no value here for
	// Someone I follow posts (Phase 9).
	//
	// OfficialPushOn is Phase 5's switch "Severe official alerts for my area"
	// (D-11), on by default; a boolean only, like the two above.
	CriticalPushOn bool
	ChangesPushOn  bool
	OfficialPushOn bool
	PushPublicKey  string
	// ProfilePublic, FollowingCount, FollowersCount, BlockedCount and MyUsername are
	// Phase 9 plan 12 (D-04): the Let others follow me switch state, the three counts
	// of the People and reports rows, and the caller's own username, which the
	// list windows need to read your own lists. All are the caller's own; nothing
	// about anyone else enters this model. html/template escapes the username.
	ProfilePublic  bool
	FollowingCount int
	FollowersCount int
	BlockedCount   int
	MyUsername     string
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
func SettingsPage(tmpl *template.Template, accounts *service.AccountService, mutes *service.MuteService, people *service.PeopleService, cfg PageConfig) http.HandlerFunc {
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

		// The people rows (Phase 9 plan 12, D-04): the switch state, the caller's own
		// username and the three counts. A failed read is a logged 500 like the muted
		// count: a switch drawn off when it is on would hide that others can follow.
		var own service.OwnProfileState
		var counts service.PeopleCounts
		if people != nil {
			own, err = people.MyProfileState(r.Context(), acc.ID)
			if err == nil {
				counts, err = people.Counts(r.Context(), acc.ID)
			}
			if err != nil {
				log.Printf("handlers: SettingsPage: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		// The alert switch and distance. A failed read is a logged 500 like the
		// profile read: a switch drawn off when it is on would let the person
		// believe they are not being alerted.
		alerts, err := accounts.AlertSettings(r.Context(), acc.ID)
		if err != nil {
			log.Printf("handlers: SettingsPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		chrome := cfg.Chrome.withDefaults()
		vm := settingsViewModel{
			AlertsOn:       alerts.ReliableAlertsOn,
			AlertRadiusKm:  alerts.AlertRadiusKm,
			HasHomeArea:    alerts.HasHomeArea,
			AlertRadii:     service.AlertRadii,
			MutedCount:     mutedCount,
			ProfilePublic:  own.ProfilePublic,
			MyUsername:     own.Username,
			FollowingCount: counts.Following,
			FollowersCount: counts.Followers,
			BlockedCount:   counts.Blocked,
			// Plan 14 draws these; see the field comments. Booleans and the public
			// key only.
			CriticalPushOn: alerts.CriticalPushOn,
			ChangesPushOn:  alerts.ChangesPushOn,
			OfficialPushOn: alerts.OfficialPushOn,
			PushPublicKey:  cfg.PushPublicKey,
			AssetVersion:   cfg.AssetVersion,
			Chrome:         chrome,
			AccountID:      acc.ID,
			Nav:            navFor(NavSettings, navTitle("Settings"), navBack("/")),
			Profile:        profile,
			Legal:          legalTextFor(chrome),
		}
		if profile.DisplayName != nil {
			vm.DisplayName = *profile.DisplayName
		}
		if profile.HomeAreaName != nil {
			vm.HomeAreaName = *profile.HomeAreaName
		}
		if profile.Username != nil {
			vm.Username = *profile.Username
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, settingsTemplateName, vm); err != nil {
			log.Printf("handlers: SettingsPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
