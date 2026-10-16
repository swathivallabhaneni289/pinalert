package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"pinalert/internal/account"
	"pinalert/web"
)

// templateName is the file executed by Page's ExecuteTemplate call.
// template.ParseFS names a parsed template after its base filename.
const templateName = "index.html.tmpl"

// PageConfig carries the values the app shell's view-model needs, kept
// configurable server-side (see cmd/server/main.go) rather than hard-coded
// in JS.
type PageConfig struct {
	FallbackLat     float64
	FallbackLon     float64
	DefaultRadiusKm float64
	// VoteRadiusKm is the distance rule's radius, taken from
	// service.MaxVoteDistanceKm in cmd/server/main.go. It is rendered as the
	// shell's vote radius attribute so votes.js can mark far reports and build
	// the too far sentence from the server's own number (D-22). The server
	// still refuses far votes on its own (D-01); this only feeds a hint.
	VoteRadiusKm float64
	// AssetVersion is appended as a "?v=" query string to every local
	// static asset the template links (CSS, JS — not the third-party CDN
	// tags, which are already content-hashed by their pinned version and
	// SRI integrity attribute). //go:embed bakes web/static into the
	// binary at compile time, and the file server sends a 1-hour
	// Cache-Control on top of that (see router.go's staticFileServer) —
	// without a version bump in the URL itself, a browser that already
	// cached the old bytes has no reason to ever re-fetch a changed CSS/JS
	// file after a restart, even a hard reload. cmd/server/main.go sets
	// this once at process start so every restart naturally busts it.
	AssetVersion string
	// Chrome is filled by api.NewRouter from Deps.Chrome. A zero value renders
	// the default numbers through withDefaults.
	Chrome Chrome
}

// pageViewModel is exactly what index.html.tmpl reads and nothing more.
type pageViewModel struct {
	FallbackLat     float64
	FallbackLon     float64
	DefaultRadiusKm float64
	VoteRadiusKm    float64
	AssetVersion    string
	// Email is the verified caller's account email, read per request from
	// the gate's context (DEC-P) — never from PageConfig, which is
	// built once at process start and cannot carry a per-request
	// identity. Rendered by web/templates/account_header.html.tmpl's
	// email row.
	Email string
	// AccountID is the verified caller's numeric account id, read per request
	// from the gate's context like Email. It is rendered only as the
	// data-account-id attribute of the page shell, so queue.js binds saved
	// reports to this account (D-20, T-04-14). It is never shown and never the
	// email.
	AccountID int64
	// Chrome is the shared bar data, already defaulted.
	Chrome Chrome
	// Nav is the data of the two app bars (Phase 3.4, D-10): the main places
	// with the current one marked, and the phone top bar title. It is built by
	// navFor and holds constants only (T-34-13).
	Nav NavView
	// Legal is the text of the in app reader window (D-31): the same values the
	// public /disclaimer and /policy pages render, so the window cannot drift from
	// them. It holds nothing about the account (R-07, T-34-34).
	Legal LegalText
}

// EndedNearbyPath is where the finished reports live since Phase 3.4: My
// activity, part Ended nearby. It is the constant target of the redirects for
// the old Past addresses (view=past, view=resolved).
const EndedNearbyPath = "/profile?part=ended"

// endedNearbyTarget answers the redirect target for a main page address that
// asks for the finished reports, or false for any other address. The view must
// be exactly past or resolved. A report parameter is carried over only when it
// is all ASCII digits and parses to a positive int64; the id is written back
// from the parsed integer, so nothing from the request reaches the target
// except a number (T-34-49).
func endedNearbyTarget(r *http.Request) (string, bool) {
	q := r.URL.Query()
	view := q.Get("view")
	if view != "past" && view != "resolved" {
		return "", false
	}
	target := EndedNearbyPath
	raw := q.Get("report")
	if raw == "" {
		return target, true
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return target, true
		}
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return target, true
	}
	return target + "&report=" + strconv.FormatInt(id, 10), true
}

// ParsePageTemplate parses the embedded app shell template from
// pinalert/web's embedded filesystem. html/template's contextual
// auto-escaping is what keeps every server-injected value safe against the
// "template data → rendered HTML" trust boundary — it does not, however,
// cover anything JavaScript later builds from a fetched JSON response (see
// web/static/js/app.js's Pinalert.setText for that boundary).
func ParsePageTemplate() (*template.Template, error) {
	return template.ParseFS(web.TemplatesFS, "templates/*.tmpl")
}

// Page renders the html/template app shell. tmpl must have been parsed via
// ParsePageTemplate (or an equivalent call against the same embedded
// filesystem) so templateName resolves to a defined template.
//
// Page is only ever reached through internal/api's gated r.Group
// (requireVerifiedAccount), so the request context always carries an
// account by the time this handler runs — a lookup miss here means the
// gate failed to do its job, which is a routing bug, not a condition a
// redirect should paper over; it is logged and answered with a 500 rather
// than silently rendering an empty email row (DEC-P).
//
// Before it renders, Page follows the remembered Low data choice (D-17): a
// bare address with the pinalert_low_data cookie answers a redirect to the Low
// data view, and the address with full=1 clears the cookie and goes to the main
// page. A main page address with any other query string is never redirected,
// so a shared deep link opens where it says, and every target is a constant
// (T-04-13).
func Page(tmpl *template.Template, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: Page: no verified account in request context — the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		// D-17: the remembered choice of the Low data view. ?full=1 is checked
		// first, clears the choice and goes to the main page. Otherwise a bare
		// address (no query string at all) with the cookie goes to the Low data
		// view, before any heavy asset is requested. Any other query string is
		// never redirected, so a shared deep link opens where it says. Every
		// target is a constant (T-04-13), so no loop and no open redirect exist.
		// Phase 3.4 (D-18): the Past tab left Nearby. An address that asked for
		// the finished reports (view past or resolved) goes to My activity,
		// Ended nearby, and carries a report id along only when it is plain
		// digits. The target is a constant plus a re-formatted integer, never a
		// string taken from the request (T-34-49). It comes before the Low data
		// handling so an old bookmark lands in the same place whatever else the
		// address holds.
		if target, ok := endedNearbyTarget(r); ok {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		if r.URL.Query().Get("full") == "1" {
			clearLowDataCookie(w, r)
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		if r.URL.RawQuery == "" {
			if c, err := r.Cookie(LowDataCookie); err == nil && c.Value == "1" {
				w.Header().Set("Cache-Control", "no-store")
				http.Redirect(w, r, LowDataPath, http.StatusFound)
				return
			}
		}
		chrome := cfg.Chrome.withDefaults()
		vm := pageViewModel{
			FallbackLat:     cfg.FallbackLat,
			FallbackLon:     cfg.FallbackLon,
			DefaultRadiusKm: cfg.DefaultRadiusKm,
			VoteRadiusKm:    cfg.VoteRadiusKm,
			AssetVersion:    cfg.AssetVersion,
			Email:           acc.Email,
			AccountID:       acc.ID,
			Chrome:          chrome,
			Nav:             navFor(NavNearby),
			Legal:           legalTextFor(chrome),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// DEC-Q: the shell now renders a person's email address into every
		// response, and a shared/public device is exactly the scenario
		// D-09's logout exists for — an intermediary or the browser's own
		// cache must never keep serving a stale, personally-identifying
		// copy of this page.
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, templateName, vm); err != nil {
			log.Printf("handlers: Page: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
