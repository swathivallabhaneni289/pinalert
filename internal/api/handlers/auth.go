package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/auth"
	"pinalert/internal/service"
	"pinalert/internal/session"
)

// loginGateTemplateName is the file executed by LoginGate's ExecuteTemplate
// call.
const loginGateTemplateName = "login_gate.html.tmpl"

// AuthConfig carries the values the login gate's view-model needs.
type AuthConfig struct {
	AssetVersion string
	// Chrome is filled by api.NewRouter from Deps.Chrome. A zero value renders
	// the default numbers through withDefaults.
	Chrome Chrome
	// DefaultRadiusKm is the list radius, filled by api.NewRouter from the page
	// config so My activity reads Ended nearby with the same radius as Nearby
	// (Phase 3.4, D-18). It is a display setting for the browser's read and is
	// not a trust figure.
	DefaultRadiusKm float64
	// Login answers the three questions the sign in page asks about the browser
	// (Phase 7.1, plan 07). It is set by api.NewRouter only when the router has a
	// CredentialService, so a nil value means the credential flow is not wired.
	// LoginGate reads it from plan 11.
	Login LoginState
	// Session is the session manager, set by api.NewRouter. LoginGate reads it
	// from plan 11 so that, once it has shown the notice of an ended session, it
	// can clear that session's cookie through the one cookie writer (T-01-85).
	Session *session.Manager
}

// LoginState is what the sign in page reads about the browser asking for it:
// whether it is already signed in with a password, whether a code screen should
// be shown again after a reload, and whether its session was ended by the switch
// to passwords. service.CredentialService implements it. api.NewRouter assigns
// it only when the service exists, because a typed nil pointer stored in an
// interface is not a nil interface.
type LoginState interface {
	SignedIn(ctx context.Context, sessionID string) (bool, error)
	LiveCode(ctx context.Context, sessionID string) (service.LiveCode, bool, error)
	EndedByPasswordSwitch(ctx context.Context, sessionID string) (bool, error)
	// CodeTTL and ResendCooldown are the numbers the service enforces, so the
	// page counts down the same ones (review IN-S02).
	CodeTTL() time.Duration
	ResendCooldown() time.Duration
}

// loginGateViewModel is exactly what login_gate.html.tmpl reads and nothing
// more. The code screen fields (CodeMode, CodeEmail, CodeSecondsLeft) are set
// only when this browser has a live code row; the durations and the digit
// count travel to the page as data attributes so the script never hard codes a
// number the server could change (Phase 7.1 plan 11, UI-SPEC Surfaces 1 and 3).
type loginGateViewModel struct {
	AssetVersion string
	Chrome       Chrome
	// LowDataDefault is true when this device's remembered choice is the Low
	// data view; the row under the card then offers Turn off low data view and
	// the status line says the choice is on (D-17, P4-A18, UI-SPEC 1e).
	LowDataDefault bool
	// DoneLine is the one status line above the heading after an account action
	// (Phase 3.2, UI-SPEC E3; Phase 7.1 adds old-link and the switch notice). It
	// is set only from closed lists and is never built from the address value.
	DoneLine string
	// StartPanel is the panel the card opens on: "create", "signin" or "code".
	StartPanel string
	// CodeMode is "create" or "reset" and is read only when StartPanel is code.
	CodeMode string
	// CodeEmail is the address a live code was sent to. It is page text only,
	// never a URL, a form value or an attribute (D-05).
	CodeEmail       string
	CodeSecondsLeft int
	// CodeDigits, CodeTTLSeconds and ResendSeconds are the numbers the script
	// reads from data attributes. ResendSeconds is the full resend cooldown
	// when there is no live code and the seconds left of it when there is one
	// (review WR-B02); CodeSecondsLeft is the seconds left of a live code.
	// ResendCooldownSeconds is always the full resend cooldown, the wait the
	// script starts after every send, so a code screen reloaded with less left
	// never takes the leftover as the next whole wait (review fix round 2,
	// skeptic SK-04).
	CodeDigits            int
	CodeTTLSeconds        int
	ResendSeconds         int
	ResendCooldownSeconds int
	// PasswordMin and PasswordMax feed minlength and maxlength on the new
	// password fields and the rule line.
	PasswordMin int
	PasswordMax int
}

// The start panels of the card and the two code modes.
const (
	loginStartCreate = "create"
	loginStartSignIn = "signin"
	loginStartCode   = "code"
	loginCodeCreate  = "create"
	loginCodeReset   = "reset"
)

// The done words of GET /login?done= and the lines they show (Phase 3.2,
// P32-C27 and P32-C28; Phase 7.1 adds old-link, D-12). Settings lands on
// all-devices after Log out of all devices and on deleted after Delete my
// account; an old emailed link lands on old-link. The list is closed: any
// other value sets nothing and is never written into the page.
const (
	loginDoneAllDevices     = "all-devices"
	loginDoneDeleted        = "deleted"
	loginDoneOldLink        = "old-link"
	loginDoneAllDevicesLine = "You are logged out of all devices."
	loginDoneDeletedLine    = "Your account has been deleted."
	loginDoneOldLinkLine    = "That sign in link no longer works. Choose Create account with the same email to set a password."
	// loginSwitchNoticeLine is shown once to a browser whose own session the
	// switch to passwords ended (UI-SPEC Surface 6, route 2).
	loginSwitchNoticeLine = "Pinalert now uses passwords. Choose Create account with the same email to set yours. Your reports stay with you."
)

// lowDataChosen reports whether the request carries the remembered choice of
// the Low data view: the cookie LowDataCookie with the value 1 and nothing else
// (D-17). It reads no account data.
func lowDataChosen(r *http.Request) bool {
	c, err := r.Cookie(LowDataCookie)
	return err == nil && c.Value == "1"
}

// LoginGate renders the sign in page at GET /login. It reads no address from
// the URL (D-05): a browser that reloads while it holds a live code is shown the
// code screen again from the server's own row for its session, with the address
// as page text only. The page address never holds an email (T-07.1-08).
//
// The order is fixed. The `low` query parameter is handled first, exactly as
// before (D-17, P4-A18): exactly 1 remembers the Low data view and exactly 0
// forgets it, and every other value changes nothing and is never written into
// the page (T-04-13). Then a person who is already signed in is sent to / (or
// the Low data page when its cookie is on) instead of seeing the form (D-10,
// research Pitfall 16); the targets are constants, never a request value
// (T-07.1-23). Then the done word is compared exactly with the closed list
// (T-07.1-20), then a live code row restores the code screen, and only when
// neither applies a browser whose own session the switch to passwords ended
// sees the switch notice once, in the same response that replaces its cookie
// with a fresh id (review WR-S01; a Set-Cookie after the body is silently
// dropped, so SetCookie runs before the template; D-12, UI-SPEC Surfaces 3, 6
// and 9). A LoginState error is
// logged without the address and the page renders the entry panels. The page
// varies per device and sets cookies, so every response is no-store (T-04-12),
// and Referrer-Policy same-origin keeps the page address out of the globe's
// tile requests (plan 05-07, D-13).
func LoginGate(tmpl *template.Template, cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vm := loginGateViewModel{
			AssetVersion:   cfg.AssetVersion,
			Chrome:         cfg.Chrome.withDefaults(),
			StartPanel:     loginStartCreate,
			CodeDigits:     auth.CodeDigits,
			CodeTTLSeconds: int(service.DefaultCredentialLimits.CodeTTL.Seconds()),
			ResendSeconds:  int(service.DefaultCredentialLimits.SendCooldown.Seconds()),
			PasswordMin:    auth.PasswordMinRunes,
			PasswordMax:    auth.PasswordMaxRunes,
		}
		// The service's own numbers when there is one (review IN-S02). With no
		// live code, data-resend-seconds is the full wait after the first send,
		// which the page script counts from; with a live code it becomes the
		// seconds left (below).
		if cfg.Login != nil {
			vm.CodeTTLSeconds = int(cfg.Login.CodeTTL() / time.Second)
			vm.ResendSeconds = int(cfg.Login.ResendCooldown() / time.Second)
		}
		vm.ResendCooldownSeconds = vm.ResendSeconds

		vm.LowDataDefault = lowDataChosen(r)
		switch r.URL.Query().Get("low") {
		case "1":
			setLowDataCookie(w, r)
			vm.LowDataDefault = true
		case "0":
			clearLowDataCookie(w, r)
			vm.LowDataDefault = false
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "same-origin")

		sessionID, haveSession := session.FromContext(r.Context())
		login := cfg.Login
		if login != nil && haveSession {
			signedIn, err := login.SignedIn(r.Context(), sessionID)
			if err != nil {
				log.Printf("handlers: LoginGate: reading sign in state: %v", err)
			} else if signedIn {
				target := "/"
				if vm.LowDataDefault {
					target = LowDataPath
				}
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}
		}

		// The done word is compared exactly with the closed list; any other
		// value sets nothing and is never written into the page (T-03.2-15).
		switch r.URL.Query().Get("done") {
		case loginDoneAllDevices:
			vm.DoneLine = loginDoneAllDevicesLine
			vm.StartPanel = loginStartSignIn
		case loginDoneDeleted:
			vm.DoneLine = loginDoneDeletedLine
		case loginDoneOldLink:
			vm.DoneLine = loginDoneOldLinkLine
		}

		if login != nil && haveSession {
			live, found, err := login.LiveCode(r.Context(), sessionID)
			if err != nil {
				log.Printf("handlers: LoginGate: reading live code: %v", err)
			} else if found {
				vm.StartPanel = loginStartCode
				vm.CodeMode = loginCodeCreate
				if live.Purpose == service.PurposeReset {
					vm.CodeMode = loginCodeReset
				}
				vm.CodeEmail = live.Email
				// Review WR-B02: both clocks are the integer seconds left,
				// clamped at 0, from the stored send and expiry, so a reload
				// or a return from the mail app never restarts the full wait.
				vm.CodeSecondsLeft = live.SecondsLeft
				vm.ResendSeconds = live.ResendSecondsLeft
			} else if vm.DoneLine == "" {
				ended, err := login.EndedByPasswordSwitch(r.Context(), sessionID)
				if err != nil {
					log.Printf("handlers: LoginGate: reading password switch: %v", err)
				} else if ended {
					vm.DoneLine = loginSwitchNoticeLine
					vm.StartPanel = loginStartCreate
					if cfg.Session != nil {
						// A fresh valid cookie, never a cleared one (review
						// WR-S01): the next Create account then has a device
						// bucket of its own, and the next visit no longer shows
						// the notice because the new id is unbound.
						cfg.Session.SetCookie(w, cfg.Session.NewID())
					}
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, loginGateTemplateName, vm); err != nil {
			log.Printf("handlers: LoginGate: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// retiredLinkTarget is where an old emailed link lands: the sign in page with the
// old-link done word, a constant with no address and no token in it (Phase 7.1
// plan 14, D-12, UI-SPEC Surface 9).
const retiredLinkTarget = "/login?done=" + loginDoneOldLink

// RetiredLink handles GET /auth/verify, the path of the emailed sign in link
// Pinalert used before passwords (Phase 7.1 plan 14, D-12). A link already sitting
// in somebody's inbox still reaches this path, so the path stays public and answers
// 303 to the constant retiredLinkTarget, where the sign in page shows the plain
// old-link line. It reads no query value, checks no token, binds no session and
// signs nobody in (T-07.1-31), and the Location holds no address (T-07.1-08).
// no-store keeps the redirect out of shared caches and the back-forward cache.
// Only GET is registered; chi does not map HEAD onto it.
//
// Deliberately carries no swag annotation: the route sits outside the /api base
// path and is an HTML redirect, not a JSON operation.
func RetiredLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, retiredLinkTarget, http.StatusSeeOther)
	}
}

// LoginFormFallback handles POST /login (Phase 7.1 review WR-T04). The card's
// panels are real POST forms to /login (D-10), so a person on a slow phone who
// taps a button before auth.js has loaded posts the form here. The answer is a
// fixed 303 to /login with no-store, where the page comes back with the script
// loaded. The body carries a plaintext password, so it is never read, parsed or
// logged, and nothing from the request goes into the answer.
//
// Deliberately carries no swag annotation: the route sits outside the /api base
// path and is an HTML redirect, not a JSON operation.
func LoginFormFallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Location", "/login")
		w.WriteHeader(http.StatusSeeOther)
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
	service.CategoryTraffic:      "Traffic",
	service.CategoryRescueNeeded: "Rescue needed",
	service.CategoryOther:        "Other",
}

// profileVisibilityStateClassPrefix mirrors visibility.js's own
// VISIBILITY_CLASS_PREFIX (T-01-17) — the server-rendered class and the
// client-rendered one must use the identical prefix so a poll-driven
// client update and a fresh page load agree on what the class is called.
const profileVisibilityStateClassPrefix = "vis-"

// profileExpiredDisplayState is the fifth display state, the one visibility.js
// displayState writes after load for an expired report that is not Retracted
// (A57). It is never a resolver visibility, so the row's own Visibility field
// keeps the resolver's state.
const profileExpiredDisplayState = "expired"

// profileModerationHeld and profileModerationRemoved are the two data-moderation
// values of an Activity row (plan 04-16, P4-A9). They are also the display
// states visibility.js displayState returns for such a row, so the class the
// server writes on first paint and the class written after load agree.
const (
	profileModerationHeld    = "held"
	profileModerationRemoved = "removed"
)

// policyPageLinkText is the words of the Policy link inside the removed row's
// pointer sentence, and writtenLinePrefix opens the Written line of a late
// report (D-24, P4-A8).
const (
	policyPageLinkText = "Policy page"
	writtenLinePrefix  = "Written "
)

// profileModeration says whether an Activity row is held or removed, and it
// reads the one resolver answer, never the raw moderation column: a pending
// serious report is published (D-01, D-02) and must not be labelled held. An
// empty string means the row is not moderated. A withheld row with any reason
// other than removal reads held, which fails closed: it still shows no trust
// and no vote controls (R-01).
func profileModeration(ar service.ActivityReport) string {
	if ar.Visibility != service.VisibilityWithheld {
		return ""
	}
	if ar.Reason == service.ReasonRemovedAfterReview {
		return profileModerationRemoved
	}
	return profileModerationHeld
}

// removedPolicyPointerParts splits the removed row's pointer sentence around
// the words of the link, so the template can write the link as a real anchor
// while the sentence itself stays the one Go constant (D-24). When the link
// words are not found the whole sentence comes back as the lead and the end to
// end test catches the missing link.
func removedPolicyPointerParts() (lead, link, tail string) {
	lead, tail, found := strings.Cut(service.RemovedPolicyPointer, policyPageLinkText)
	if !found {
		return service.RemovedPolicyPointer, "", ""
	}
	return lead, policyPageLinkText, tail
}

// severityWord is the Activity page's visible severity word. The Go side owns
// it because the app.js word table is not reachable from the template (A58).
// The low fallback matches the one ActivityForAccount applies when it
// validates severity.
func severityWord(s service.Severity) string {
	switch s {
	case service.SeverityCritical:
		return "Critical"
	case service.SeverityMedium:
		return "Medium"
	default:
		return "Low"
	}
}

// The words of an approximate voted card (plan 03.1-07, C17 and C18): the title
// a finished Rescue report of someone else gets in place of its description,
// and the detail words that say the point is blurred.
const (
	profileApproxTitle    = "Rescue needed"
	profileApproxLocation = "Approximate location"
)

// profileReport is exactly what profile.html.tmpl's Activity section reads
// for one row — Phase 1's existing .report-row markup and classes, reused
// verbatim rather than reinvented for this account-scoped listing
// (01.1-07-PLAN.md Task 2, UI-SPEC item 16). ReportID, Visibility,
// VisibilityReason, VisibilityClass and CanReopen are 02-07's additions:
// the first four give activity.js the hooks it needs to render the trust
// chip; CanReopen (Task 3) gates the Reopen control to rows that are both
// retracted AND not yet expired. SeverityWord, SeverityLevel, TrustJSON and
// Expired are plan 03-15's (D-20, A44, A58): the visible severity word, its
// level for the label modifier class, the shared trust object as JSON, and
// the server's own expiry decision.
type profileReport struct {
	ReportID         int64
	CategoryGlyph    string
	SeverityClass    string
	SeverityWord     string
	SeverityLevel    string
	Description      string
	Visibility       string
	VisibilityReason string
	VisibilityClass  string
	TrustJSON        string
	Expired          bool
	CanReopen        bool
	// UndosUsed is true on a closed, unexpired own post whose two undos are
	// both used (plan 03.1-07, D-22). The template writes it as
	// data-undos-used and activity.js shows the C52 sentence in place of the
	// Reopen control. It is never true together with CanReopen.
	UndosUsed bool

	// Moderation is held or removed for a report the author cannot see in any
	// feed (D-02, D-08), empty otherwise. ModerationNote is the Go constant
	// sentence for it, empty for a held row past its expiry. PolicyPointer is
	// true for a removed row, whose sentence may continue with the Policy link
	// when a contact address is set (D-24). WrittenText is the Written line of
	// a report that reached the server late (P4-A8, D-19).
	Moderation     string
	ModerationNote string
	PolicyPointer  bool
	WrittenText    string

	// The card fields filled by buildActivityPage. Kind is post, vote or dispute
	// and picks the part the card is listed in; Tag and Action are the words of
	// the card's kind; When is the relative time and WhenText the When cell's
	// words ("Posted 12 min ago"); Detail is the category label with the shelter
	// words; IconGlyph is the round icon's glyph class.
	Kind      string
	Tag       string
	Action    string
	When      string
	WhenText  string
	Detail    string
	IconGlyph string

	// StatusWord is the Status chip's words (Phase 3.4, UI-SPEC P34-C12):
	// Ongoing, Resolved, Disputed, Expired, or one of the two moderation words.
	// It follows the same precedence as the row class: removed, then held while
	// unexpired, then resolved, then expired, then disputed.
	StatusWord string
	// Lat and Lon are the point the Distance cell is measured from. They are the
	// row's own point, which is already the coarse cell centre for a finished
	// Rescue report of someone else (Approximate), so the page never carries an
	// exact point the card hides (T-34-31).
	Lat         float64
	Lon         float64
	Approximate bool
}

// profileRecord is the reporter record card of the phone Activity page: the
// account's own reporter signal as the server wrote it. Label is the signal's
// everyday word copied unchanged, and Counts is built from the real judged and
// corroborated numbers, so a new reporter reads 0 of 0 confirmed and nothing is
// invented for it.
type profileRecord struct {
	Label  string
	Counts string
}

// newProfileRecord builds the record card from one reporter signal. A signal
// with no label (the none tag, an account with nothing to rate) gives no card.
// It computes nothing: the label and both counts come from the signal as
// service.BuildTrust already wrote them.
func newProfileRecord(sig service.ReporterSignal) *profileRecord {
	if sig.Label == "" {
		return nil
	}
	return &profileRecord{
		Label:  sig.Label,
		Counts: fmt.Sprintf("%d of %d confirmed", sig.CorroboratedReports, sig.JudgedReports),
	}
}

// profileViewModel is exactly what profile.html.tmpl reads and nothing more.
type profileViewModel struct {
	AssetVersion string
	// Parts are the five parts of My activity with their cards and counts
	// (Phase 3.4, D-18). CurrentPart is the key the address asked for, empty
	// for the list.
	Parts       []activityPart
	CurrentPart string
	// Record is the reporter record card. It is nil for an account with no
	// posts, and the template then renders no card.
	Record *profileRecord
	Chrome Chrome
	// The three parts of the removed row's Policy pointer sentence, cut around
	// the link words (D-24).
	PolicyPointerLead string
	PolicyPointerLink string
	PolicyPointerTail string
	// LowDataDefault is true when this device's remembered choice is the Low
	// data view. The page has no back link now, but the saved reports strip
	// still sends a resend to the Low data view for such a device (P4-A21), so
	// the page writes the address as data-resend-page.
	LowDataDefault bool
	// AccountID is the verified account's numeric id. It is rendered only on
	// the account's own no-store Activity page as data-account-id, so queue.js
	// binds saved reports to the account that wrote them. It is never the
	// email and is never shown.
	AccountID int64
	// Nav is the data of the two app bars (Phase 3.4, D-10), built by navFor
	// from constants only (T-34-13).
	Nav NavView
	// DefaultRadiusKm is the radius the browser sends when it reads Ended nearby
	// (Phase 3.4, D-18), written as data-default-radius-km on the page. The
	// reader's position is not here: it stays in the browser.
	DefaultRadiusKm float64
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

// profileDetail builds the detail words of one card: the category label, plus
// the shelter capacity and headcount for a shelter report, the way the meta
// line always built them, without the time. One helper serves both the meta
// line and the sub line of the six tab page, so the two can never drift.
func profileDetail(capacity *string, headcount *int32, category service.Category) string {
	detail := categoryLabels[category]
	if category == service.CategoryShelterOpen {
		if capacity != nil && *capacity != "" {
			detail += " · " + capitalizeASCII(*capacity)
		}
		if headcount != nil {
			n := *headcount
			unit := "people"
			if n == 1 {
				unit = "person"
			}
			detail += fmt.Sprintf(" · %d %s", n, unit)
		}
	}
	return detail
}

// newProfileReport maps one service.ActivityReport into the view struct
// profile.html.tmpl renders. Category and severity are already validated
// by ActivityForAccount — this handler reads those values rather than
// re-validating the raw row, so the resolver's inputs and the rendered
// class names can never disagree (T-01-17): two copies of the same
// fallback could drift apart, and a report would render one category's
// colour against another category's trust state.
//
// CanReopen (Task 3, D-16 amended) gates the Reopen control to rows that
// are BOTH retracted AND not yet past their own expires_at: 02-03a's
// expiry rejection (step 4) is not scoped by vote kind the way its
// self-vote block is, so a reopen on an expired report would 409 — and
// even a successful one would restore nothing, since NearbyReports filters
// on expires_at regardless of visibility. Resolve is deliberately
// expiry-blind (expiry is not one of the four states), which is why this
// clause lives here rather than in the resolver, and why it gates the
// BUTTON alone, never the "Resolved" label.
//
// TrustJSON is the same object the feed and the vote response carry, from the
// same mapper, never re derived here (D-20, V-33). Expired is the only expiry
// rule the Activity page has, so activity.js compares no dates (A44). An
// expired report that is not Retracted gets the expired row class on first
// paint; a Retracted one keeps its retracted class, so Resolved wins over
// Expired (State Matrix 10a and 10b).
func newProfileReport(ar service.ActivityReport, now time.Time) (profileReport, error) {
	row := ar.Row
	category := ar.Category
	severity := ar.Severity

	detail := profileDetail(row.ShelterCapacityStatus, row.ShelterHeadcount, category)

	// Resolve cannot return an invalid Visibility, so this guard is
	// defence in depth (T-01-17) rather than a real branch — but the
	// fallback direction is a decision with a trust consequence and must
	// match visibility.js's own fallback: provisional, never live. A page
	// that renders "trusted" for a value it could not validate overstates
	// corroboration, which is the one thing this product exists not to do.
	visibility := ar.Visibility
	if !visibility.Valid() {
		visibility = service.VisibilityProvisional
	}

	expired := !row.ExpiresAt.After(now)
	// The poster's Reopen spends the same two undos as the take back (D-22),
	// and the server refuses a third, so the card stops offering a button that
	// could only fail. UndosLeft is the service's own count from the votes log;
	// a closed unexpired post with none left says so in words instead.
	closedOpen := visibility == service.VisibilityRetracted && !expired
	canReopen := closedOpen && ar.UndosLeft > 0
	undosUsed := closedOpen && ar.UndosLeft <= 0

	// An approximate card is a finished Rescue report of someone else (D-27,
	// D-23): it shows the coarse words and never the description text.
	description := row.Description
	if ar.Approximate {
		description = profileApproxTitle
		detail += " · " + profileApproxLocation
	}

	// The row class precedence of P4-A9, the same one visibility.js displayState
	// applies after load: removed, then held only while the report has not
	// expired, then retracted, then expired, then the vote state.
	moderation := profileModeration(ar)
	moderationNote := ""
	visibilityClass := profileVisibilityStateClassPrefix + string(visibility)
	// The Status chip word follows the same precedence as the row class, so the
	// word and the class never disagree (UI-SPEC P34-C12).
	statusWord := statusOngoing
	switch visibility {
	case service.VisibilityRetracted:
		statusWord = statusResolved
	case service.VisibilityHidden:
		statusWord = statusDisputed
	}
	switch {
	case moderation == profileModerationRemoved:
		visibilityClass = profileVisibilityStateClassPrefix + profileModerationRemoved
		moderationNote = service.RemovedSentence
		statusWord = statusRemoved
	case moderation == profileModerationHeld && !expired:
		visibilityClass = profileVisibilityStateClassPrefix + profileModerationHeld
		moderationNote = service.HeldSentence
		statusWord = statusHeld
	case expired && visibility != service.VisibilityRetracted:
		visibilityClass = profileVisibilityStateClassPrefix + profileExpiredDisplayState
		statusWord = statusExpired
	}

	// A moderated row carries no trust object: the author sees the reason it is
	// in no feed, not a score for it (P4-A10, R-01).
	trustJSON := []byte{}
	if moderation == "" {
		var err error
		trustJSON, err = json.Marshal(trustToResponse(ar.Trust))
		if err != nil {
			return profileReport{}, fmt.Errorf("handlers: encoding trust for report %d: %w", row.ID, err)
		}
	}

	writtenText := ""
	if ar.WrittenAt != nil {
		writtenText = writtenLinePrefix + profileRelativeTime(*ar.WrittenAt, now)
	}

	return profileReport{
		ReportID:         row.ID,
		CategoryGlyph:    "icon-glyph--" + string(category),
		SeverityClass:    "sev-" + string(severity),
		SeverityWord:     severityWord(severity),
		SeverityLevel:    string(severity),
		Description:      description,
		Detail:           detail,
		Visibility:       string(visibility),
		VisibilityReason: string(ar.Reason),
		VisibilityClass:  visibilityClass,
		TrustJSON:        string(trustJSON),
		Expired:          expired,
		CanReopen:        canReopen,
		UndosUsed:        undosUsed,
		Moderation:       moderation,
		ModerationNote:   moderationNote,
		PolicyPointer:    moderation == profileModerationRemoved,
		WrittenText:      writtenText,
		StatusWord:       statusWord,
		Lat:              row.Latitude,
		Lon:              row.Longitude,
		Approximate:      ar.Approximate,
	}, nil
}

// Profile handles GET /profile: every report the caller's verified account
// has ever filed, across every session/device it has ever verified on
// (ReportsByAccount, Task 1), plus every report it currently confirms or
// disputes (VotedReportsByAccount, quick task 261013-wm5), rendered inside one
// merged Activity section (D-13) with six tabs. The tab value in the address
// only picks which tab is selected; it never reaches a query (T-wm5-03).
// Profile is only ever reached through internal/api's gated
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
func Profile(svc *service.AuthService, updates *service.UpdatesService, tmpl *template.Template, cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: Profile: no verified account in request context — the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// One call per request, keyed by the gate's account id and nothing
		// from the request: the account's posts and the reports it currently
		// confirms or disputes (T-wm5-01).
		activityPage, err := svc.ActivityPageForAccount(r.Context(), acc.ID)
		if err != nil {
			log.Printf("handlers: Profile: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// The part and the old tab value only pick which screen renders; both are
		// matched against constants and never reach a query or a link (T-34-30).
		part := parseActivityPart(r.URL.Query().Get("part"), r.URL.Query().Get("tab"))
		now := time.Now()
		view, err := buildActivityPage(activityPage, part, now)
		if err != nil {
			log.Printf("handlers: Profile: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// The Updates part reads the same List the bell reads (Phase 3.2, D-05),
		// keyed by the gate's account id. There is no Refresh here: the bell poll
		// is the producer (research Pattern 8). A failed read renders the page
		// with the part's error state instead of a 500. The read mark stays with
		// the browser (plan 08).
		if updates != nil {
			updatesPage, uerr := updates.List(r.Context(), acc.ID, service.ActivityUpdatesListSize)
			if uerr != nil {
				log.Printf("handlers: Profile: updates: %v", uerr)
			}
			fillUpdatesPart(&view, updatesPage, uerr != nil, now)
		}

		// The record card reads the account's own reporter signal from its
		// own posts, never from the merged card list (W-10). An account with
		// no posts gets no card, and a signal with no label gives none either.
		policyLead, policyLink, policyTail := removedPolicyPointerParts()
		vm := profileViewModel{
			AssetVersion:      cfg.AssetVersion,
			Parts:             view.Parts,
			CurrentPart:       view.CurrentPart,
			Record:            view.Record,
			Chrome:            cfg.Chrome.withDefaults(),
			PolicyPointerLead: policyLead,
			PolicyPointerLink: policyLink,
			PolicyPointerTail: policyTail,
			LowDataDefault:    lowDataChosen(r),
			AccountID:         acc.ID,
			Nav:               navFor(NavActivity),
			DefaultRadiusKm:   cfg.DefaultRadiusKm,
		}
		// On a part screen the phone top bar names the part and its back arrow
		// returns to the list (D-19). The list keeps the plain My activity title.
		if name := view.CurrentName(); name != "" {
			vm.Nav = navFor(NavActivity, navTitle(name), navBack(activityListHref))
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// DEC-T: this page renders the account's entire report history, the
		// most sensitive of the three no-store responses in this phase (the
		// gate's own redirect and 01.1-06's app shell being the other two), and
		// a shared or public device is exactly the scenario D-09's logout
		// exists for. The verified address is no longer printed on it.
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, profileTemplateName, vm); err != nil {
			log.Printf("handlers: Profile: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// Logout handles POST /auth/logout — D-09's explicit log-out action, which
// exists so a shared or public device can be handed back: clear the
// session cookie (mgr.ClearCookie), then redirect to /login with
// StatusSeeOther, the status that turns the POST into a GET on the
// follow-up so a browser reload never re-submits the logout. This response
// itself reflects an authentication-state change (the browser's session
// cookie is gone) and so carries Cache-Control: no-store, the same
// discipline Verify's and Profile's responses already apply.
//
// sessions.account_id is deliberately left intact (DEC-K): that row's
// binding is what still connects reports filed from this session to the
// account, which is exactly what the profile page reads — unbinding it
// here would silently erase a person's own history from their own
// profile. Its on-screen control already exists: 01.1-06 shipped the
// "Log out" item in the shared header's account menu as a real POST form
// submit one wave earlier; this handler is what stops that submit from
// 404ing. This page's own template adds no logout control of its own
// (see profile.html.tmpl) — the control and the CSRF property DEC-L
// records (SameSite=Lax withholding the session cookie from a cross-site
// POST) both live with 01.1-06's shared header, not here.
//
// Phase 3.3, decision 11 and UI-SPEC O-3: before the cookie is cleared the
// push subscriptions this session registered are deleted, so the next person
// who signs in on a shared phone never receives the previous person's alerts.
// DEC-K still holds, the session row stays bound to its account. Amended
// 2026-10-18 by Phase 3.3 plan 10 (decision 11, UI-SPEC O-3): plain Log out is
// no longer free of side effects for the push subscription.
//
// A failure to delete is logged without any endpoint and stops the log out
// with the usual 500, keeping the cookie, so the person can press Log out
// again. There is no backstop to lean on: plain Log out never revokes the
// session row (DEC-K), so the send path's live-session join would still match
// and the device would keep receiving this account's alerts. Amended
// 2026-10-25 (review S-03); before, the failure was logged and the person was
// signed out anyway.
func Logout(mgr *session.Manager, subs *service.PushSubscriptionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionID, ok := session.FromContext(r.Context()); ok && sessionID != "" && subs != nil {
			if err := subs.DeleteForSession(r.Context(), sessionID); err != nil {
				log.Printf("handlers: Logout: deleting this device's push subscriptions: %v", err)
				w.Header().Set("Cache-Control", "no-store")
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		mgr.ClearCookie(w)
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
