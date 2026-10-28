package handlers

import (
	"html/template"
	"log"
	"net/http"

	"pinalert/internal/account"
)

// alertsTemplateName is the file executed by AlertsPage's ExecuteTemplate call.
const alertsTemplateName = "alerts.html.tmpl"

// alertsViewModel is exactly what alerts.html.tmpl reads. The page holds no
// alert data of its own: the shell is server rendered, and official-alerts.js
// asks GET /api/official/alerts for the rest (Phase 5 plan 09), so the
// template carries the account id and the empty blocks the script fills.
type alertsViewModel struct {
	AssetVersion string
	Chrome       Chrome
	// AccountID is the verified account's numeric id, rendered as the
	// data-account-id attribute of the page like the other account pages. It
	// is never the email and is never shown.
	AccountID int64
	// Nav is the data of the two app bars (D-10), built by navFor from
	// constants only (T-34-13). Official alerts is a main place, so it is the
	// current item and its label is the phone top bar title.
	Nav NavView
}

// AlertsPage handles GET /alerts: the Official alerts place (Phase 3.4, D-22),
// filled by Phase 5 plan 09. It renders the shell only: the top bars, the status
// line, the blocks for every state and the attribution. The server authors the
// state of every answer (GET /api/official/alerts says fresh, stale or
// unavailable, whether the area is known and which groups of alerts apply), and
// the page script writes the sentences from that state alone.
//
// The page must never say that no alert is in force unless the feed is fresh,
// the area is known and every group is empty: anywhere else it would be false
// reassurance in an emergency (T-34-17, ALERT-04). So the template carries no
// state sentence at all, and web/official_alerts_test.mjs pins every state
// against the claim.
//
// Like Profile it is only reached through the gated r.Group, so a missing
// account in the context is a routing bug, logged and answered with a 500.
// The response is no-store like the other account pages.
//
// Deliberately carries no swag annotation: it serves text/html and lives
// outside @BasePath /api, the same reasoning Profile's doc comment records.
func AlertsPage(tmpl *template.Template, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: AlertsPage: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		vm := alertsViewModel{
			AssetVersion: cfg.AssetVersion,
			Chrome:       cfg.Chrome.withDefaults(),
			AccountID:    acc.ID,
			Nav:          navFor(NavAlerts),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, alertsTemplateName, vm); err != nil {
			log.Printf("handlers: AlertsPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
