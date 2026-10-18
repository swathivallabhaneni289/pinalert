// This file serves the Low data view of D-13 and D-14: a text only page that
// lists Active reports with no map and no map library, for a person on a weak
// connection. It also holds the remembered choice of D-17, a small preference
// cookie that makes a bare visit to the main page open this view instead. The
// cookie carries no account data: it holds the digit 1 and nothing else.

package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"pinalert/internal/account"
)

// LowDataPath is the address of the Low data view.
const LowDataPath = "/low"

// LowDataCookie names the cookie that remembers a device's choice of the Low
// data view (D-17).
const LowDataCookie = "pinalert_low_data"

// lowDataCookieMaxAge is one year in seconds.
const lowDataCookieMaxAge = 365 * 24 * 60 * 60

// lowTemplateName is the file executed by LowPage's ExecuteTemplate call.
const lowTemplateName = "low.html.tmpl"

// lowViewModel is exactly what low.html.tmpl and the partials it calls read.
type lowViewModel struct {
	FallbackLat     float64
	FallbackLon     float64
	DefaultRadiusKm float64
	VoteRadiusKm    float64
	AssetVersion    string
	// Email and AccountID come from the gate's context, never from PageConfig
	// (DEC-P). The id is rendered as a number, never the address.
	Email     string
	AccountID int64
	Chrome    Chrome
}

// requestIsHTTPS reports whether the request reached the host over TLS, either
// directly or through the hosting proxy that terminates it. The result only
// decides the Secure flag of a preference cookie. It is never used for any
// authority decision, so a forged header only changes the forger's own cookie.
func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// setLowDataCookie remembers the Low data view on this device for a year.
func setLowDataCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     LowDataCookie,
		Value:    "1",
		Path:     "/",
		MaxAge:   lowDataCookieMaxAge,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		// A device preference read only by the server and listed on the Policy
		// page, so it is not HttpOnly.
		HttpOnly: false,
	})
}

// clearLowDataCookie forgets the choice. MaxAge -1 sends Max-Age=0.
func clearLowDataCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     LowDataCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		HttpOnly: false,
	})
}

// LowPage renders the Low data view. Like Page it is only reached through the
// gated group, so a missing account is a routing bug and answers 500 before
// any cookie is set.
func LowPage(tmpl *template.Template, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: LowPage: no verified account in request context, the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		chrome := cfg.Chrome.withDefaults()
		chrome.LowDataPage = true
		// The Low data view has no map, so only the centre matters, not the zoom.
		startLat, startLon, _ := startCentre(r.Context(), cfg, acc.ID)
		vm := lowViewModel{
			FallbackLat:     startLat,
			FallbackLon:     startLon,
			DefaultRadiusKm: cfg.DefaultRadiusKm,
			VoteRadiusKm:    cfg.VoteRadiusKm,
			AssetVersion:    cfg.AssetVersion,
			Email:           acc.Email,
			AccountID:       acc.ID,
			Chrome:          chrome,
		}
		setLowDataCookie(w, r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The page renders the email and the account id (DEC-Q).
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, lowTemplateName, vm); err != nil {
			log.Printf("handlers: LowPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
