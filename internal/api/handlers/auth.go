package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/auth"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
)

// maxAuthBodyBytes caps the request-link POST body, matching
// maxSubmitBodyBytes's DoS-conscious cap on POST /api/reports (threat T-01-53).
const maxAuthBodyBytes = 64 * 1024

// loginGateTemplateName is the file executed by LoginGate's ExecuteTemplate
// call.
const loginGateTemplateName = "login_gate.html.tmpl"

// AuthConfig carries the values the login gate's view-model needs.
type AuthConfig struct {
	AssetVersion string
}

// loginGateViewModel is exactly what login_gate.html.tmpl (and the
// check_inbox.html.tmpl partial it includes) reads and nothing more.
// ResendCooldownSeconds and LinkTTLSeconds are two independent durations —
// D-03's resend cooldown and D-02's link expiry — sourced from
// service.ResendCooldown and auth.TokenTTL respectively so the browser
// never hard-codes a duration the server could change (UI-SPEC item 9: the
// two timers are independent and must never be derived from one another).
type loginGateViewModel struct {
	AssetVersion          string
	Email                 string
	Sent                  bool
	ResendCooldownSeconds int
	LinkTTLSeconds        int
}

// LoginGate renders the globe login gate at GET /login. A `sent` query
// parameter is honored only when it parses as a bare email address via
// net/mail — the same non-display-name rule service.RequestLink enforces —
// so a manual "Check again" reload (or a bookmarked/shared URL) re-renders
// the waiting state server-side without depending on JavaScript having run
// first (UI-SPEC item 11).
func LoginGate(tmpl *template.Template, cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vm := loginGateViewModel{
			AssetVersion:          cfg.AssetVersion,
			ResendCooldownSeconds: int(service.ResendCooldown.Seconds()),
			LinkTTLSeconds:        int(auth.TokenTTL.Seconds()),
		}

		if raw := r.URL.Query().Get("sent"); raw != "" {
			if addr, err := mail.ParseAddress(raw); err == nil && addr.Name == "" {
				vm.Sent = true
				vm.Email = addr.Address
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, loginGateTemplateName, vm); err != nil {
			log.Printf("handlers: LoginGate: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// RequestLinkRequest is the raw JSON shape POST /api/auth/request-link
// accepts. Decoded with DisallowUnknownFields so an unexpected field is a
// loud 400, matching SubmitReportRequest's convention.
type RequestLinkRequest struct {
	// Email is the address to verify. Server-side validation (RFC 5322
	// address parsing, no display-name form) happens in
	// service.AuthService.RequestLink — never trust client-side validation.
	Email string `json:"email" example:"visitor@example.com"`
}

// RequestLinkResponse confirms the normalized address a verification link
// was requested for. It never names a session identifier (T-01-02's
// published-schema counterpart, enforced by
// handlers_test.TestSwaggerSpecCoversRoutes).
type RequestLinkResponse struct {
	Email string `json:"email" example:"visitor@example.com"`
}

// RequestLink handles POST /api/auth/request-link: decode, validate and
// mint a token (via svc), and hand it to the configured Mailer. A
// service.ErrRateLimited (D-03/D-04's per-address resend cooldown) maps to
// 429 with the same generic message and field name the per-IP limiter's own
// refusal uses (internal/ratelimit), so a caller cannot tell which limiter
// tripped or whether the address has an account. A service.ValidationError
// maps to 400 naming the offending field; any other error maps to a generic
// 500 with the detail logged server-side only, so a mailer/database failure
// never reaches a client (threat T-01-55).
//
// @Summary      Request a magic-link verification email
// @Description  Validates the given email address, mints a single-use 5-minute magic-link
// @Description  token, persists its hash, and hands the raw link to the configured mailer.
// @Description  The response never carries a session identifier.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      RequestLinkRequest  true  "Email to verify"
// @Success      200   {object}  RequestLinkResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      429   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /auth/request-link [post]
func RequestLink(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req RequestLinkRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		normalizedEmail, err := svc.RequestLink(r.Context(), req.Email)
		if err != nil {
			if errors.Is(err, service.ErrRateLimited) {
				writeFieldError(w, http.StatusTooManyRequests, "email", "Too many requests — try again in a minute.")
				return
			}
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: RequestLink: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, RequestLinkResponse{Email: normalizedEmail})
	}
}

// verifyOutcomeTemplateName is the file executed by Verify's
// ExecuteTemplate call.
const verifyOutcomeTemplateName = "verify_outcome.html.tmpl"

// verifyOutcomeViewModel is exactly what verify_outcome.html.tmpl reads and
// nothing more. Email is the newly-verified address (OutcomeVerified);
// CurrentEmail is the session's existing verified address, shown only on
// the OutcomeConflict screen ("You're signed in as {{.CurrentEmail}}").
type verifyOutcomeViewModel struct {
	AssetVersion string
	Outcome      string
	Email        string
	CurrentEmail string
}

// Verify handles GET /auth/verify: reads the current session id from
// context (never from a query parameter — the token names the account, the
// signed cookie names the browser being bound), delegates the whole
// decision to svc.VerifyToken, and renders the one shared outcome template.
// Only GET is ever registered for this route (see router.go) — chi does
// not auto-map HEAD onto a GET-only handler the way net/http.ServeMux
// does, which cheaply sidesteps one class of mail-security-scanner
// prefetch (threat T-01-62).
//
// Deliberately carries no swag annotation and so has no entry in
// docs/swagger.json: @BasePath is /api (see router.go's package-level swag
// block) and this route lives outside that base path — it's an HTML
// landing page a mail client's browser navigates to, not a JSON API
// operation a client is meant to call programmatically. Do not "fix" this
// by adding an annotation; TestSwaggerSpecCoversRoutes does not assert its
// presence, and doing so would misrepresent the operation's shape (it
// returns text/html, not any of this package's JSON response types).
func Verify(svc *service.AuthService, tmpl *template.Template, cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := session.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		token := r.URL.Query().Get("token")
		outcome, email, err := svc.VerifyToken(r.Context(), sessionID, token)
		if err != nil {
			log.Printf("handlers: Verify: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		vm := verifyOutcomeViewModel{
			AssetVersion: cfg.AssetVersion,
			Outcome:      string(outcome),
		}
		if outcome == service.OutcomeVerified {
			vm.Email = email
		}
		if outcome == service.OutcomeConflict {
			vm.CurrentEmail = email
		}

		// This response reflects an authentication state change (a session
		// either just became verified, or the request revealed which
		// account the browser is currently signed in as) — it must never
		// be served from a shared cache or restored from the
		// back-forward cache (threat T-01-64).
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, verifyOutcomeTemplateName, vm); err != nil {
			log.Printf("handlers: Verify: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// profileTemplateName is the file executed by Profile's ExecuteTemplate call.
const profileTemplateName = "profile.html.tmpl"

// categoryLabels mirrors web/static/js/app.js's CATEGORY_LABELS (D-01) for
// this page's server-rendered listing. Presentation-layer only — kept here
// rather than promoted into internal/service, which has no reason to know
// about human-readable copy.
var categoryLabels = map[service.Category]string{
	service.CategoryFlood:        "Flood",
	service.CategoryEarthquake:   "Earthquake",
	service.CategoryFire:         "Fire",
	service.CategoryStormCyclone: "Storm/Cyclone damage",
	service.CategoryRoadBlocked:  "Road blocked",
	service.CategoryPowerOutage:  "Power outage",
	service.CategoryShelterOpen:  "Shelter open",
	service.CategoryRescueNeeded: "Rescue needed",
	service.CategoryOther:        "Other",
}

// profileReport is exactly what profile.html.tmpl's Activity section reads
// for one row — Phase 1's existing .report-row markup and classes, reused
// verbatim rather than reinvented for this account-scoped listing
// (01.1-07-PLAN.md Task 2, UI-SPEC item 16).
type profileReport struct {
	CategoryGlyph string
	SeverityClass string
	Description   string
	MetaText      string
}

// profileViewModel is exactly what profile.html.tmpl reads and nothing more.
type profileViewModel struct {
	AssetVersion string
	Email        string
	Reports      []profileReport
}

// profileRelativeTime mirrors web/static/js/app.js's relativeTime: the same
// coarse buckets (just now / N min ago / N hours ago / N days ago), computed
// once server-side at render time since this page carries no client-side JS
// that refreshes it on an interval the way the live feed does.
func profileRelativeTime(t, now time.Time) string {
	diff := now.Sub(t)
	if diff < 0 {
		diff = 0
	}
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Round(time.Minute) / time.Minute)
		if mins < 1 {
			mins = 1
		}
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Round(time.Hour) / time.Hour)
		if hours < 1 {
			hours = 1
		}
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(diff.Round(24*time.Hour) / (24 * time.Hour))
		if days < 1 {
			days = 1
		}
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// capitalizeASCII upper-cases only the first byte of s — sufficient for the
// four lowercase, ASCII shelter capacity status slugs
// (service.CapacityStatuses), matching feed.js's own CAPACITY_LABELS
// derivation (capitalizing the raw slug rather than a separate label map).
func capitalizeASCII(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// newProfileReport maps one ReportsByAccount row into the view struct
// profile.html.tmpl renders. category/severity fall back to "other"/"low"
// for any value outside the canonical enum — the same validate-before-
// building-a-className discipline app.js's iconClass/severityClass apply
// client-side (T-01-17), applied here since this handler builds the
// className string directly rather than leaving it to client JS.
func newProfileReport(row sqlcgen.ReportsByAccountRow, now time.Time) profileReport {
	category := service.Category(row.Category)
	if !category.Valid() {
		category = service.CategoryOther
	}
	severity := service.Severity(row.Severity)
	if !severity.Valid() {
		severity = service.SeverityLow
	}

	meta := categoryLabels[category] + " · " + profileRelativeTime(row.CreatedAt, now)
	if category == service.CategoryShelterOpen {
		if row.ShelterCapacityStatus != nil && *row.ShelterCapacityStatus != "" {
			meta += " · " + capitalizeASCII(*row.ShelterCapacityStatus)
		}
		if row.ShelterHeadcount != nil {
			n := *row.ShelterHeadcount
			unit := "people"
			if n == 1 {
				unit = "person"
			}
			meta += fmt.Sprintf(" · %d %s", n, unit)
		}
	}

	return profileReport{
		CategoryGlyph: "icon-glyph--" + string(category),
		SeverityClass: "sev-" + string(severity),
		Description:   row.Description,
		MetaText:      meta,
	}
}

// Profile handles GET /profile: every report the caller's verified account
// has ever filed, across every session/device it has ever verified on
// (ReportsByAccount, Task 1), rendered inside one merged Activity section
// (D-13). Profile is only ever reached through internal/api's gated
// r.Group (requireVerifiedAccount), so the request context always carries
// an account by the time this handler runs — a lookup miss here means the
// gate failed to do its job, a routing bug rather than a condition a
// redirect should paper over (the same discipline handlers.Page's DEC-P
// already applies).
//
// Deliberately carries no swag annotation and so has no entry in
// docs/swagger.json, the same reasoning Verify's doc comment above records:
// this route serves text/html, not a JSON API operation, and lives outside
// @BasePath /api (see router.go's package-level swag block).
func Profile(svc *service.AuthService, tmpl *template.Template, cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: Profile: no verified account in request context — the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		rows, err := svc.ReportsForAccount(r.Context(), acc.ID)
		if err != nil {
			log.Printf("handlers: Profile: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		now := time.Now()
		reports := make([]profileReport, 0, len(rows))
		for _, row := range rows {
			reports = append(reports, newProfileReport(row, now))
		}

		vm := profileViewModel{
			AssetVersion: cfg.AssetVersion,
			Email:        acc.Email,
			Reports:      reports,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// DEC-T: this page renders the verified address alongside the
		// account's entire report history — the most sensitive of the
		// three no-store responses in this phase (the gate's own redirect
		// and 01.1-06's app shell being the other two), and a shared or
		// public device is exactly the scenario D-09's logout exists for.
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, profileTemplateName, vm); err != nil {
			log.Printf("handlers: Profile: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
