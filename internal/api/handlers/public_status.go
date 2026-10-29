// This file serves the public status page a shared link opens (Phase 8 plan 04,
// COORD-08, D-05, D-06, SC5). GET /r/{token} is the one route of Phase 8 that
// opens without an account: a small server rendered page for one report with
// no script, no session and no cookie. It shows the category and the rough
// area, whether the report is active, disputed, resolved or ended, the server's
// own confirmed or disputed sentence, the severity, and when. It never shows
// the reporter's words, a name, a comment, a distance, a map or the exact spot,
// because the view model below has no field that could hold one.

package handlers

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/service"
)

// publicStatusTemplate is the name the page template defines.
const publicStatusTemplate = "share_status"

// publicStatusOGImagePath is the app mark served by the PWA assets. It is the
// only image a link preview ever shows: never a per report image, never a count
// (UI-SPEC 6a). It is a 512 pixel PNG well under the 600 KB WhatsApp allows.
const publicStatusOGImagePath = "/icons/app-512.png"

// publicStatusRetryAfter is the Retry-After header of a refused request, in
// seconds. It is a minute, the wait the page's own line asks for
// (service.PublicTooManyLine, review IN-T04).
const publicStatusRetryAfter = "60"

// publicStatusViewModel is exactly what the public page template reads. It has
// no field for a description, a coordinate, a distance, a username, a comment
// or the trust facts, so none of them can reach the page (T-08-19). Every
// string is escaped by html/template, attributes included (T-08-16).
type publicStatusViewModel struct {
	AssetVersion string
	Chrome       Chrome

	// State is the service's state word; the template does not branch on it
	// beyond Title and Chip being empty or not.
	State service.PublicState
	// Title is the h1, and the og:title and the document title's text.
	Title string
	// OGDescription is the fixed line, the same bytes in every state.
	OGDescription string
	// OGImageURL and CanonicalURL are absolute and rooted in configuration, never
	// in the request's Host header (T-08-17).
	OGImageURL   string
	CanonicalURL string

	// Chip is the neutral status word; Sentence the server's box line.
	Chip     string
	Sentence string
	// SeverityWord and SeverityClass draw the one accent, the severity dot.
	SeverityWord  string
	SeverityClass string
	// Happening is the What is happening value, only for a Critical report with
	// a pick.
	Happening string
	// PostedText is the value of the Posted row.
	PostedText string
	// EndedLabel is Resolved or Ended and EndedText when, for the two finished
	// states only.
	EndedLabel string
	EndedText  string

	// TooMany marks the rate limited page: the h1, the line and the buttons only.
	TooMany     bool
	TooManyLine string
}

// PublicStatusPage serves GET /r/{token}. allow is the per client IP limit: it
// returns false for a request that should be refused (the router builds it
// only when CLIENT_IP_HEADER is set, and admits every request otherwise). base
// is the configured public address that roots og:url and og:image.
//
// The status code follows the state: 200 for active, disputed, resolved and
// ended, 410 for a report a moderator removed, 404 for a held, unknown, bad or
// too old link, and 429 with Retry-After for a refusal. Every answer is HTML,
// no-store, noindex and sets no cookie; the handler never reads a session and
// never reads r.Host.
func PublicStatusPage(tmpl *template.Template, cfg AuthConfig, svc *service.ShareService, base string, allow func(*http.Request) bool) http.HandlerFunc {
	trimmedBase := strings.TrimRight(base, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		chrome := cfg.Chrome.withDefaults()
		vm := publicStatusViewModel{
			AssetVersion:  cfg.AssetVersion,
			Chrome:        chrome,
			OGDescription: service.ShareFixedLine,
			OGImageURL:    trimmedBase + publicStatusOGImagePath,
			CanonicalURL:  trimmedBase + "/",
		}
		status := http.StatusOK

		if allow != nil && !allow(r) {
			vm.TooMany = true
			vm.Title = service.PublicTooManyTitle
			vm.TooManyLine = service.PublicTooManyLine
			writePublicStatus(w, tmpl, vm, http.StatusTooManyRequests, true)
			return
		}

		token := chi.URLParam(r, "token")
		if service.ValidShareToken(token) {
			vm.CanonicalURL = PublicShareURL(base, token)
		}
		st, err := svc.PublicStatus(r.Context(), token)
		if err != nil {
			// The token is a capability: it is never written to a log.
			log.Printf("handlers: PublicStatusPage: public status failed")
			writePublicStatusError(w)
			return
		}

		vm.State = st.State
		vm.Title = st.Title
		switch st.State {
		case service.PublicRemoved:
			status = http.StatusGone
		case service.PublicUnavailable:
			status = http.StatusNotFound
		default:
			now := time.Now()
			vm.Chip = st.Chip
			vm.Sentence = st.Sentence
			vm.SeverityWord = st.SeverityWord
			vm.SeverityClass = "sev-" + string(st.Severity)
			vm.Happening = st.Happening
			vm.PostedText = service.PublicAgo(st.PostedAt, now)
			if st.EndedLabel != "" {
				vm.EndedLabel = st.EndedLabel
				if st.EndedAt != nil {
					vm.EndedText = service.PublicAgo(*st.EndedAt, now)
				}
			}
		}
		writePublicStatus(w, tmpl, vm, status, false)
	}
}

// writePublicStatus renders the page into a buffer, so a template failure is a
// clean 500 and never a half page, then writes the headers every answer of the
// page carries.
func writePublicStatus(w http.ResponseWriter, tmpl *template.Template, vm publicStatusViewModel, status int, retry bool) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, publicStatusTemplate, vm); err != nil {
		log.Printf("handlers: PublicStatusPage: %v", err)
		writePublicStatusError(w)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	setPublicStatusPrivacyHeaders(h)
	if retry {
		h.Set("Retry-After", publicStatusRetryAfter)
	}
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// setPublicStatusPrivacyHeaders sets the three headers every answer of the page
// carries, a 500 included: never cached, never indexed, and no referrer sent
// onward, so the link's token never leaves in a Referer header.
func setPublicStatusPrivacyHeaders(h http.Header) {
	h.Set("Cache-Control", "no-store")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Referrer-Policy", "no-referrer")
}

// writePublicStatusError is the page's one 500: a plain text error with the
// same privacy headers as every other answer (review IN-S02), so a crawler can
// neither cache nor index the error.
func writePublicStatusError(w http.ResponseWriter) {
	setPublicStatusPrivacyHeaders(w.Header())
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
