// Package service holds Pinalert's report business logic: the category,
// severity, and shelter-capacity enums; server-side validation; severity-
// tiered expiry computation; and the bounding-box math the nearby-reports
// query depends on. Nothing here trusts a client-supplied value for
// anything the server itself is authoritative over (expiry, geohash,
// creation time).
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// geohashPrecision matches internal/testutil/seed.go and 01-RESEARCH.md's
// Open Questions recommendation: finer than any plausible Phase 3
// independence-counting cell size, since a finer value is always safely
// truncatable later and a coarser one would force a backfill.
const geohashPrecision = 8

// Severity-tiered expiry durations (D-14): a flat Phase 1 default, keyed on
// severity rather than category. Per-category and trust-weighted tuning
// arrive once confirm/dispute exists in Phase 2/3.
const (
	criticalExpiry = 24 * time.Hour
	defaultExpiry  = 8 * time.Hour
)

// maxHeadcount is the upper bound on shelter_headcount — large enough for
// any real shelter, small enough to reject a garbage/overflow value.
const maxHeadcount = 100_000

// Category is a report category (D-01). The exact slugs and their order are a
// contract: the report form renders its category grid from the postable list
// and the Swagger enums are documented from the two lists below.
type Category string

const (
	CategoryFlood        Category = "flood"
	CategoryEarthquake   Category = "earthquake"
	CategoryFire         Category = "fire"
	CategoryStormCyclone Category = "storm_cyclone"
	CategoryRoadBlocked  Category = "road_blocked"
	CategoryPowerOutage  Category = "power_outage"
	CategoryShelterOpen  Category = "shelter_open"
	CategoryTraffic      Category = "traffic"
	CategoryRescueNeeded Category = "rescue_needed"
	CategoryOther        Category = "other"
)

// RescueNeededRetiredMessage is the 400 message for a new report that still
// sends rescue_needed. Plain words, no dashes.
const RescueNeededRetiredMessage = "Rescue needed is no longer a report category. Choose another one."

// Categories is every category a stored report may carry, in order. Valid and
// every read path (the category filter, the feed, the Activity page) use this
// list, so a report stored as rescue_needed still reads, filters and renders.
var Categories = []Category{
	CategoryFlood,
	CategoryEarthquake,
	CategoryFire,
	CategoryStormCyclone,
	CategoryRoadBlocked,
	CategoryPowerOutage,
	CategoryShelterOpen,
	CategoryTraffic,
	CategoryRescueNeeded,
	CategoryOther,
}

// PostableCategories is the nine categories a new report may use, in order.
// The report form renders its tiles from it and the POST enum documents it.
// rescue_needed was retired for new reports on 2026-10-16 by owner decision,
// and Traffic took its place. Stored rescue_needed rows keep their rules.
var PostableCategories = []Category{
	CategoryFlood,
	CategoryEarthquake,
	CategoryFire,
	CategoryStormCyclone,
	CategoryRoadBlocked,
	CategoryPowerOutage,
	CategoryShelterOpen,
	CategoryTraffic,
	CategoryOther,
}

// Valid reports whether c is one of the known categories, rescue_needed
// included, because a stored report may still carry it.
func (c Category) Valid() bool {
	for _, v := range Categories {
		if v == c {
			return true
		}
	}
	return false
}

// Postable reports whether a new report may use c.
func (c Category) Postable() bool {
	for _, v := range PostableCategories {
		if v == c {
			return true
		}
	}
	return false
}

// Severity is one of the three report severities.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityCritical Severity = "critical"
)

// Severities is the ordered, canonical list of every valid severity.
var Severities = []Severity{SeverityLow, SeverityMedium, SeverityCritical}

// Valid reports whether s is one of the three canonical severities.
// Comparison is exact — no case-insensitive coercion.
func (s Severity) Valid() bool {
	for _, v := range Severities {
		if v == s {
			return true
		}
	}
	return false
}

// Happening is the What is happening pick on a Critical report (D-24, D-24a):
// one of seven closed values the report form offers. It is optional, accepted
// only together with severity critical, and stored in reports.happening. It is
// display and routing data for later phases: Resolve never reads it.
type Happening string

const (
	HappeningWaterRising       Happening = "water_rising"
	HappeningPeopleTrapped     Happening = "people_trapped"
	HappeningFire              Happening = "fire"
	HappeningBuildingCollapsed Happening = "building_collapsed"
	HappeningLandslide         Happening = "landslide"
	HappeningMedicalEmergency  Happening = "medical_emergency"
	HappeningOther             Happening = "other"
)

// Happenings is the ordered, canonical list of the seven picks, in the order the
// report form shows them.
var Happenings = []Happening{
	HappeningWaterRising,
	HappeningPeopleTrapped,
	HappeningFire,
	HappeningBuildingCollapsed,
	HappeningLandslide,
	HappeningMedicalEmergency,
	HappeningOther,
}

// Valid reports whether h is one of the seven picks. Comparison is exact, with
// no case folding and no trimming.
func (h Happening) Valid() bool {
	for _, v := range Happenings {
		if v == h {
			return true
		}
	}
	return false
}

// CapacityStatus is a shelter-open report's occupancy status.
type CapacityStatus string

const (
	CapacityAvailable CapacityStatus = "available"
	CapacityLimited   CapacityStatus = "limited"
	CapacityFull      CapacityStatus = "full"
	CapacityClosed    CapacityStatus = "closed"
)

// CapacityStatuses is the ordered, canonical list of every valid capacity status.
var CapacityStatuses = []CapacityStatus{CapacityAvailable, CapacityLimited, CapacityFull, CapacityClosed}

// Valid reports whether cs is one of the four canonical capacity statuses.
func (cs CapacityStatus) Valid() bool {
	for _, v := range CapacityStatuses {
		if v == cs {
			return true
		}
	}
	return false
}

// ValidationError names the single input field that failed validation and a
// user-safe message compatible with 01-UI-SPEC.md's Copywriting Contract, so
// a server rejection never contradicts the client's own inline copy.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// SubmitInput is every client-settable field of a report submission.
// expires_at, created_at, geohash, and id are deliberately absent — they
// are computed server-side and can never be set from a request body.
// RequestID is the optional retry key ("" is absent) that makes a resend after
// a lost answer return the stored report instead of storing a second one (R-05).
// WrittenAt is the display only time the device wrote the report, clamped and
// never used for created_at, expires_at, the order or the age stage (R-05).
type SubmitInput struct {
	Category              Category
	Severity              Severity
	Description           string
	Latitude              float64
	Longitude             float64
	ShelterCapacityStatus *CapacityStatus
	ShelterHeadcount      *int
	// Happening is optional and accepted only with severity critical (D-24a: the
	// picks show for Critical only). nil is absent. Stored, never read by Resolve.
	Happening *Happening
	RequestID string
	WrittenAt *time.Time
}

// WrittenAtMaxAge is how far before created_at a stored written time may lie.
// An older value is clamped to exactly this age, never dropped, so a queued
// report never looks fresher than it is (R-05).
const WrittenAtMaxAge = 14 * 24 * time.Hour

// ClampWrittenAt returns a UTC copy of w limited to the window from
// WrittenAtMaxAge before now up to now: a later value becomes now and an older
// one becomes now minus WrittenAtMaxAge. Nil stays nil. The result is display
// only. The handler applies it to the parsed request value and SubmitChecked
// applies it again against the same now that stamps created_at, which is the
// authority (R-05, T-04-17).
func ClampWrittenAt(w *time.Time, now time.Time) *time.Time {
	if w == nil {
		return nil
	}
	out := w.UTC()
	switch {
	case out.After(now):
		out = now.UTC()
	case now.Sub(out) > WrittenAtMaxAge:
		out = now.Add(-WrittenAtMaxAge).UTC()
	}
	return &out
}

// happeningParam converts the optional pick to the nullable column value: nil
// stays NULL.
func happeningParam(h *Happening) *string {
	if h == nil {
		return nil
	}
	s := string(*h)
	return &s
}

// writtenAtParam converts the clamped written time to the nullable column
// value, the inverse of writtenAtPtr: nil gives the zero value, so the column
// stays NULL.
func writtenAtParam(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// RequestIDInvalidMessage is the 400 message for a request id that is not 16 to
// 64 letters, digits or hyphens. It is the one server authored copy of R-05.
const RequestIDInvalidMessage = "This report couldn't be sent as it was. Reload the page and post it again."

// requestIDPattern is the closed shape of a request id.
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{16,64}$`)

// Report is a fully materialized, server-computed report, returned by
// Nearby. It carries no session identifier (see the threat
// register T-01-02, Information Disclosure).
type Report struct {
	ID                    int64
	Category              Category
	Severity              Severity
	Description           string
	Latitude              float64
	Longitude             float64
	Geohash               string
	ShelterCapacityStatus *CapacityStatus
	ShelterHeadcount      *int
	CreatedAt             time.Time
	ExpiresAt             time.Time
	// DistanceKm is populated by Nearby only; the report just stored (CreatedReport) has none,
	// since it has no query point to be distant from.
	DistanceKm *float64
}

// NearbyQuery is the caller's proximity search: a center point and a
// radius. Defaults and bounds on RadiusKm (10 default, 0.1 min, 50 max) are
// the handler's responsibility, not this type's. ViewerAccountID comes from
// the verified-account gate's request context, never from a query string —
// a client cannot ask "what did account 12 vote". 0 means "no identified
// viewer", which yields an empty ViewerVote and a false IsOwnReport for
// every report. IncludeDisputed is D-10/D-11's "Show disputed" toggle,
// gating Hidden reports into the result for both the list and the map pins
// at once; it never reveals Retracted reports (D-12). Category narrows the
// list to one category and "" means all. The filter runs in Go after Resolve,
// never in SQL, and it is the reader's own request: a chip never hides a report
// the reader did not ask to hide (D-17).
type NearbyQuery struct {
	Latitude        float64
	Longitude       float64
	RadiusKm        float64
	ViewerAccountID int64
	IncludeDisputed bool
	// All is the Show menu's All (D-14): Live, Provisional and Hidden reports in
	// one read, each with its own view word, so the client can list the disputed
	// ones last. It takes precedence over IncludeDisputed, never reveals a
	// Retracted, expired, held or removed report, and changes no count.
	All      bool
	Category Category
}

// ValidateSubmitInput enforces every server-side rule a report submission
// must satisfy. Validation is unconditional and server-side: a client can
// send anything, so nothing here trusts client-side form validation to have
// already run.
func ValidateSubmitInput(in SubmitInput) error {
	if !in.Category.Valid() {
		return ValidationError{"category", "Choose a category to continue."}
	}
	if !in.Category.Postable() {
		return ValidationError{"category", RescueNeededRetiredMessage}
	}
	if !in.Severity.Valid() {
		return ValidationError{"severity", "Pick a severity level."}
	}

	// The description is counted in characters (Unicode code points), not
	// bytes, because Telugu, Hindi and Tamil letters take 3 bytes each and the
	// messages promise characters (R-06).
	desc := strings.TrimSpace(in.Description)
	n := utf8.RuneCountInString(desc)
	if n < 10 {
		return ValidationError{"description", "Add a short description (at least 10 characters)."}
	}
	if n > 1000 {
		return ValidationError{"description", "Description must be 1000 characters or fewer."}
	}

	if in.Latitude < -90 || in.Latitude > 90 {
		return ValidationError{"latitude", "Set a location by searching, tapping the map, or allowing location access."}
	}
	if in.Longitude < -180 || in.Longitude > 180 {
		return ValidationError{"longitude", "Set a location by searching, tapping the map, or allowing location access."}
	}

	if in.Category == CategoryShelterOpen {
		if in.ShelterCapacityStatus == nil || !in.ShelterCapacityStatus.Valid() {
			return ValidationError{"shelter_capacity_status", "Choose a shelter capacity status."}
		}
	} else {
		if in.ShelterCapacityStatus != nil {
			return ValidationError{"shelter_capacity_status", "Capacity status only applies to shelter_open reports."}
		}
		if in.ShelterHeadcount != nil {
			return ValidationError{"shelter_headcount", "Headcount only applies to shelter_open reports."}
		}
	}

	if in.ShelterHeadcount != nil {
		if *in.ShelterHeadcount < 0 || *in.ShelterHeadcount > maxHeadcount {
			return ValidationError{"shelter_headcount", "Headcount must be a realistic number of people."}
		}
	}

	// The What is happening pick: a known value, and only for a Critical report
	// (D-24, D-24a). An empty string is not absent: absent is nil.
	if in.Happening != nil {
		if !in.Happening.Valid() {
			return ValidationError{"happening", "Choose what is happening from the list."}
		}
		if in.Severity != SeverityCritical {
			return ValidationError{"happening", "What is happening only applies to Critical reports."}
		}
	}

	// The retry key is checked last and never trimmed or rewritten (R-05).
	if in.RequestID != "" && !requestIDPattern.MatchString(in.RequestID) {
		return ValidationError{"request_id", RequestIDInvalidMessage}
	}

	return nil
}

// ExpiryDuration returns D-14's two-tier default: 24 hours for critical, 8
// hours for low and medium. This is explicitly a Phase 1 default; per-
// category and trust-weighted tuning arrive once confirm/dispute exists.
func ExpiryDuration(s Severity) time.Duration {
	if s == SeverityCritical {
		return criticalExpiry
	}
	return defaultExpiry
}

// BoundingBox converts a center point and radius into a lat/lon box for the
// nearby-reports query's indexed prefilter. Latitude is clamped to +/-90;
// near the poles, where the longitude-degree distance collapses toward
// zero, the full longitude range is returned instead of an unbounded delta.
func BoundingBox(lat, lon, radiusKm float64) (latMin, latMax, lonMin, lonMax float64) {
	const kmPerDegreeLat = 111.045

	latDelta := radiusKm / kmPerDegreeLat
	latMin = clamp(lat-latDelta, -90, 90)
	latMax = clamp(lat+latDelta, -90, 90)

	cosLat := math.Cos(lat * math.Pi / 180)
	if math.Abs(cosLat) < 1e-9 {
		return latMin, latMax, -180, 180
	}

	lonDelta := radiusKm / (kmPerDegreeLat * cosLat)
	lonMin = clamp(lon-lonDelta, -180, 180)
	lonMax = clamp(lon+lonDelta, -180, 180)
	return latMin, latMax, lonMin, lonMax
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Querier is the subset of the sqlc-generated Queries type ReportService
// needs — small enough to fake in a test without a real Postgres. The read
// path now needs the current votes and the reporter identity for the page
// it is returning: CurrentVotesForReports and ReporterAccountsForReports.
// VotingQuerier in trust.go declares CurrentVotesForReports too — two
// consumer-defined interfaces naming one generated method is idiomatic Go
// and deliberately not deduplicated into a shared interface, because each
// stays small enough to fake without a real Postgres. ReporterHistoryReports
// is the bounded history read the feed needs to compose reporter signals. The
// submit path writes the decided moderation state in the same insert and
// records a check row through InsertModerationCheck.
type Querier interface {
	InsertReportIdempotent(ctx context.Context, arg sqlcgen.InsertReportIdempotentParams) (sqlcgen.InsertReportIdempotentRow, error)
	// ReportByRequest is the replay lookup, always scoped to the caller's
	// account (T-04-05).
	ReportByRequest(ctx context.Context, arg sqlcgen.ReportByRequestParams) (sqlcgen.ReportByRequestRow, error)
	InsertModerationCheck(ctx context.Context, arg sqlcgen.InsertModerationCheckParams) error
	NearbyReports(ctx context.Context, arg sqlcgen.NearbyReportsParams) ([]sqlcgen.NearbyReportsRow, error)
	CurrentVotesForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReporterAccountsForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error)
	ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error)
	// ExpiredReportsInWindow is the expired candidate set of the Past read.
	ExpiredReportsInWindow(ctx context.Context, arg sqlcgen.ExpiredReportsInWindowParams) ([]sqlcgen.ExpiredReportsInWindowRow, error)
	// OpenReportsInArea is the unexpired candidate set that may have been closed.
	OpenReportsInArea(ctx context.Context, arg sqlcgen.OpenReportsInAreaParams) ([]sqlcgen.OpenReportsInAreaRow, error)
	// ReportForFocus reads one report by id for a report link.
	ReportForFocus(ctx context.Context, arg sqlcgen.ReportForFocusParams) (sqlcgen.ReportForFocusRow, error)
	// ResolutionUndoCounts is the viewer's take back and reopen count per report.
	ResolutionUndoCounts(ctx context.Context, arg sqlcgen.ResolutionUndoCountsParams) ([]sqlcgen.ResolutionUndoCountsRow, error)
	// ReportExtrasForReports is the one batched read of the moderation state,
	// the display only written time and the viewer's flag for a page of
	// reports. It is held off NearbyReports so that query stays as shipped.
	ReportExtrasForReports(ctx context.Context, arg sqlcgen.ReportExtrasForReportsParams) ([]sqlcgen.ReportExtrasForReportsRow, error)
}

// ReportService validates, computes derived fields, and persists reports.
type ReportService struct {
	q        Querier
	now      func() time.Time
	screener TextScreener
}

// ReportOption configures a ReportService at construction time.
type ReportOption func(*ReportService)

// WithReportClock overrides the clock the read path uses. It is named apart
// from WithClock, which already returns AuthOption (Pitfall 6). It governs
// the read path only: SubmitChecked keeps the wall clock, because vote times and the
// NearbyReports expiry filter come from the database clock, and report times
// written on a test clock would drift from both.
func WithReportClock(fn func() time.Time) ReportOption {
	return func(s *ReportService) {
		s.now = fn
	}
}

// WithScreener sets the screener for the inline text check. Without it every
// text that passes the gibberish prefilter is clear and nothing is sent
// anywhere, which is what every existing test harness gets.
func WithScreener(s TextScreener) ReportOption {
	return func(rs *ReportService) {
		rs.screener = s
	}
}

// NewReportService constructs a ReportService backed by q. The read path
// clock defaults to time.Now unless overridden with WithReportClock.
func NewReportService(q Querier, opts ...ReportOption) *ReportService {
	s := &ReportService{q: q, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreatedReport is a freshly stored report as SubmitChecked returns it. It
// carries no session identifier (T-01-02) and no distance, since a new report
// has no query point to be distant from. Moderation is the state the insert
// wrote.
type CreatedReport struct {
	ID                    int64
	Category              Category
	Severity              Severity
	Description           string
	Latitude              float64
	Longitude             float64
	Geohash               string
	ShelterCapacityStatus *CapacityStatus
	ShelterHeadcount      *int
	CreatedAt             time.Time
	ExpiresAt             time.Time
	Moderation            ModerationState
}

// SubmitResult is the answer of SubmitChecked. Held is true exactly when the
// stored row is withheld (ReportMeta.Withheld). Replayed is set only by the
// request id replay (04-17). Check is the inline text check as it ran.
type SubmitResult struct {
	Report   CreatedReport
	Held     bool
	Replayed bool
	Check    TextCheck
}

// ErrTextRejected is the sentinel a refused text wraps. Nothing was stored.
var ErrTextRejected = errors.New("text rejected by the text check")

// TextRejectedError is returned by SubmitChecked when the text check refused
// the description. Reason names why, and RejectMessage(Reason) is the message
// the author sees.
type TextRejectedError struct {
	Reason RejectReason
}

func (e TextRejectedError) Error() string {
	return "text rejected: " + string(e.Reason)
}

// Unwrap lets errors.Is(err, ErrTextRejected) match.
func (e TextRejectedError) Unwrap() error {
	return ErrTextRejected
}

// The moderation_checks.status values (D-04), reused by the background worker.
const (
	moderationCheckPending = "pending"
	moderationCheckDone    = "done"
)

func createdReportFromInsertRow(row sqlcgen.InsertReportIdempotentRow) CreatedReport {
	var capacityStatus *CapacityStatus
	if row.ShelterCapacityStatus != nil {
		cs := CapacityStatus(*row.ShelterCapacityStatus)
		capacityStatus = &cs
	}
	var headcount *int
	if row.ShelterHeadcount != nil {
		h := int(*row.ShelterHeadcount)
		headcount = &h
	}
	return CreatedReport{
		ID:                    row.ID,
		Category:              Category(row.Category),
		Severity:              Severity(row.Severity),
		Description:           row.Description,
		Latitude:              row.Latitude,
		Longitude:             row.Longitude,
		Geohash:               row.Geohash,
		ShelterCapacityStatus: capacityStatus,
		ShelterHeadcount:      headcount,
		CreatedAt:             row.CreatedAt,
		ExpiresAt:             row.ExpiresAt,
		Moderation:            ModerationState(row.ModerationState),
	}
}

// replayResult builds the answer for a request id this account already used.
// The row has the same twelve columns as the insert row, so the one mapper
// serves both, and Held is the same predicate a first send uses, so a first
// answer and a repeat can never disagree.
func replayResult(row sqlcgen.ReportByRequestRow) SubmitResult {
	created := createdReportFromInsertRow(sqlcgen.InsertReportIdempotentRow(row))
	held := ReportMeta{Severity: created.Severity, Category: created.Category, Moderation: created.Moderation}.Withheld()
	return SubmitResult{Report: created, Held: held, Replayed: true}
}

// FindReplay is step 3 of the pinned POST /api/reports order, called by the
// handler before any allowance is reserved or the text check runs. It reports
// whether accountID already stored a report under requestID, and returns that
// report. The account comes from the verified account in the request context,
// never the body, the session or chi's request id (T-04-05, Pitfall 9). An
// empty request id is never a replay and costs no query.
func (s *ReportService) FindReplay(ctx context.Context, accountID int64, requestID string) (SubmitResult, bool, error) {
	if requestID == "" {
		return SubmitResult{}, false, nil
	}
	row, err := s.q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: accountID, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return SubmitResult{}, false, nil
	}
	if err != nil {
		return SubmitResult{}, false, err
	}
	return replayResult(row), true, nil
}

// textCheckRow builds the moderation_checks row for a check outcome. An
// unfinished check gives a pending row whose next attempt honours the
// provider's wait; a borderline text gives a done row flagged for review that
// holds the verdict, scores and model but never the text. Any other outcome
// keeps no row and returns false.
func textCheckRow(reportID int64, check TextCheck, now time.Time) (sqlcgen.InsertModerationCheckParams, bool) {
	switch check.Outcome {
	case TextUnfinished:
		return sqlcgen.InsertModerationCheckParams{
			ReportID:      reportID,
			Status:        moderationCheckPending,
			NextAttemptAt: now.Add(check.RetryAfter),
		}, true
	case TextBorderline:
		verdict := string(VerdictBorderline)
		model := check.Model
		scores, err := json.Marshal(check.Scores)
		if err != nil {
			scores = nil
		}
		return sqlcgen.InsertModerationCheckParams{
			ReportID:      reportID,
			Status:        moderationCheckDone,
			NextAttemptAt: now,
			Verdict:       &verdict,
			Scores:        scores,
			Model:         &model,
			CheckedAt:     pgtype.Timestamptz{Time: now, Valid: true},
			NeedsReview:   true,
		}, true
	}
	return sqlcgen.InsertModerationCheckParams{}, false
}

// SubmitChecked validates in, screens the description, computes expires_at
// (from severity, per D-14) and the geohash server side, and stores the report
// under sessionID. The order is the specification:
//
//  1. ValidateSubmitInput, returning its ValidationError unchanged. The handler
//     already validated; a public method never trusts its caller.
//  2. CheckText on the trimmed description. Only the text reaches the screener.
//  3. A rejected text returns TextRejectedError before any write: nothing is
//     stored (D-03).
//  4. The moderation state is pending for an unfinished check and clear
//     otherwise, at every severity. A serious pending report is published by
//     Resolve's critical bypass and stays pending so a late verdict can still
//     remove it (D-01, R-03). The state is decided before the insert and
//     written by it, so a held report is never visible for a moment.
//  5. now is captured once from the wall clock, never the injectable report
//     clock, whose test times would drift from the database clock. created_at
//     and expires_at are stamped from it, and the display only written time is
//     clamped against the same now, so it is never later than created_at nor
//     more than WrittenAtMaxAge before it. No stamp ever reads the written time
//     (R-05).
//  6. One InsertReportIdempotent. accountID is the gate's verified account and,
//     with a request id, is stored beside it. When the insert returns no row for
//     a request id, a racing send stored it first: the account scoped lookup
//     returns that report with Replayed true, before any check row or other
//     write (R-05).
//  7. A pending or borderline check row, tried at most twice. If both writes
//     fail the report stays stored and the error is logged with the numeric id
//     only: a 500 would invite a second post of the same report, and a held
//     report staying held is the fail closed side.
//  8. Held is ReportMeta.Withheld of the stored row.
func (s *ReportService) SubmitChecked(ctx context.Context, sessionID string, accountID int64, in SubmitInput) (SubmitResult, error) {
	if err := ValidateSubmitInput(in); err != nil {
		return SubmitResult{}, err
	}

	desc := strings.TrimSpace(in.Description)
	check := CheckText(ctx, s.screener, desc)
	if check.Outcome == TextRejected {
		return SubmitResult{Check: check}, TextRejectedError{Reason: check.Reason}
	}

	state := ModerationClear
	if check.Outcome == TextUnfinished {
		state = ModerationPending
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ExpiryDuration(in.Severity))
	writtenAt := ClampWrittenAt(in.WrittenAt, now)
	gh := geohash.EncodeWithPrecision(in.Latitude, in.Longitude, geohashPrecision)

	var capacityStatus *string
	if in.ShelterCapacityStatus != nil {
		v := string(*in.ShelterCapacityStatus)
		capacityStatus = &v
	}
	var headcount *int32
	if in.ShelterHeadcount != nil {
		v := int32(*in.ShelterHeadcount)
		headcount = &v
	}

	// The request id and its account are set together or not at all (the
	// reports_request_id_needs_account check).
	var requestID *string
	var requestAccountID *int64
	if in.RequestID != "" {
		id := in.RequestID
		acct := accountID
		requestID = &id
		requestAccountID = &acct
	}

	row, err := s.q.InsertReportIdempotent(ctx, sqlcgen.InsertReportIdempotentParams{
		SessionID:             sessionID,
		Category:              string(in.Category),
		Severity:              string(in.Severity),
		Description:           desc,
		Latitude:              in.Latitude,
		Longitude:             in.Longitude,
		Geohash:               gh,
		ShelterCapacityStatus: capacityStatus,
		ShelterHeadcount:      headcount,
		Happening:             happeningParam(in.Happening),
		CreatedAt:             now,
		ExpiresAt:             expiresAt,
		ModerationState:       string(state),
		RequestID:             requestID,
		RequestAccountID:      requestAccountID,

		// Display only, never read by any stamp (R-05).
		ClientWrittenAt: writtenAtParam(writtenAt),
	})
	if errors.Is(err, pgx.ErrNoRows) && requestID != nil {
		// A racing send stored this id first. Return the stored report before
		// any check row or other write (R-05).
		stored, lookupErr := s.q.ReportByRequest(ctx, sqlcgen.ReportByRequestParams{AccountID: accountID, RequestID: *requestID})
		if lookupErr != nil {
			return SubmitResult{}, lookupErr
		}
		return replayResult(stored), nil
	}
	if err != nil {
		return SubmitResult{}, err
	}
	created := createdReportFromInsertRow(row)

	if checkRow, ok := textCheckRow(created.ID, check, now); ok {
		var checkErr error
		for attempt := 0; attempt < 2; attempt++ {
			if checkErr = s.q.InsertModerationCheck(ctx, checkRow); checkErr == nil {
				break
			}
		}
		if checkErr != nil {
			log.Printf("service: SubmitChecked: recording the text check for report %d: %v", created.ID, checkErr)
		}
	}

	held := ReportMeta{Severity: created.Severity, Category: created.Category, Moderation: created.Moderation}.Withheld()
	return SubmitResult{Report: created, Held: held, Check: check}, nil
}

// reporterHistoryParams builds the one history read of a request. It is the
// one place the D-13 window and cap reach the query, shared by all three read
// paths: the window starts ReliabilityWindow before now and the cap is
// ReliabilityHistoryReadCap (neutral rows are only known after the votes are
// read, so the read leaves room for them and Go caps the judged rows). A report is judged once it expired at or before this same
// now (R-05), so the caller passes the clock it also hands to ReporterSignals.
func reporterHistoryParams(accountIDs []int64, now time.Time) sqlcgen.ReporterHistoryReportsParams {
	return sqlcgen.ReporterHistoryReportsParams{
		Now:         now,
		WindowStart: now.Add(-ReliabilityWindow),
		RowCap:      int32(ReliabilityHistoryReadCap),
		AccountIds:  accountIDs,
	}
}

// voteReadIDs returns the ids for a request's one vote read: the page ids in
// order, then each history report id not already present, in history order.
// The history votes ride in the same read, so a request keeps one vote read
// (V-29).
func voteReadIDs(pageIDs []int64, history []sqlcgen.ReporterHistoryReportsRow) []int64 {
	out := make([]int64, 0, len(pageIDs)+len(history))
	seen := make(map[int64]struct{}, len(pageIDs)+len(history))
	for _, id := range pageIDs {
		out = append(out, id)
		seen[id] = struct{}{}
	}
	for _, row := range history {
		if _, ok := seen[row.ReportID]; ok {
			continue
		}
		seen[row.ReportID] = struct{}{}
		out = append(out, row.ReportID)
	}
	return out
}

// MaxReporterAccountsPerRead is the most distinct reporter accounts a feed
// read loads history for, counted over the reports in the order NearbyReports
// returns them, which is nearest first. A report whose reporter is past the cap
// carries the none signal and is still listed, in its place (RESOLVED Q7). The
// cap bounds the history read and the vote read that a 50 km feed can trigger
// on every 30 second poll (T-03-05). Only Nearby can reach it, because
// CastVote and ActivityForAccount read one account.
const MaxReporterAccountsPerRead = 200

// pageReporterAccounts collects each non nil reporter account of the page
// once, in the order the ids are given and of first appearance. It stops once
// MaxReporterAccountsPerRead accounts are collected. An account already
// collected still maps to its signal on a farther report, because
// reporterSignalRef looks signals up by account.
func pageReporterAccounts(ids []int64, reporterByReport map[int64]*int64) []int64 {
	var accounts []int64
	seen := make(map[int64]struct{})
	for _, id := range ids {
		reporterID := reporterByReport[id]
		if reporterID == nil {
			continue
		}
		if _, ok := seen[*reporterID]; ok {
			continue
		}
		if len(accounts) >= MaxReporterAccountsPerRead {
			break
		}
		seen[*reporterID] = struct{}{}
		accounts = append(accounts, *reporterID)
	}
	return accounts
}

// reporterSignalRef looks up the signal of one reporter account. It returns
// nil when accountID is nil or has no entry, and a pointer to a copy of the
// entry otherwise. Nil yields the none signal in BuildTrust, which is the rule
// for accountless reports and for reports past the account cap (A47, Q7).
func reporterSignalRef(signals map[int64]ReporterSignal, accountID *int64) *ReporterSignal {
	if accountID == nil {
		return nil
	}
	signal, ok := signals[*accountID]
	if !ok {
		return nil
	}
	return &signal
}

// voteRowsByReport groups vote rows by report id once, keeping input order.
// The vote read now also carries history votes, so per report work reads only
// its own rows instead of scanning the whole result (T-03-05).
func voteRowsByReport(rows []sqlcgen.CurrentVotesForReportsRow) map[int64][]sqlcgen.CurrentVotesForReportsRow {
	byReport := make(map[int64][]sqlcgen.CurrentVotesForReportsRow)
	for _, row := range rows {
		byReport[row.ReportID] = append(byReport[row.ReportID], row)
	}
	return byReport
}

// Nearby runs the indexed bounding-box prefilter followed by exact
// Haversine distance (see internal/store/queries/reports.sql's
// NearbyReports), loads the page's reporter identities, the reporters' bounded
// history and the current votes in batched calls, asks Resolve once per
// report, and filters on the
// answer in Go. This remains the ONE read path serving both the map and the
// list — handlers.NearbyReports is the only route that calls it and there
// is no map-only or list-only query (01-RESEARCH.md Pattern 3, TRUST-02).
// Visibility is decided by Resolve and filtered in Go over its output,
// never by a WHERE clause — internal/store/queries/reports.sql deliberately
// carries no visibility, vote or votes-table predicate, and its being
// byte-for-byte what Phase 1 shipped is the evidence (ARCHITECTURE.md
// Anti-Pattern 1, threat T-02-04). The two batched reads are what keep a
// page of N reports at a constant number of round trips (BuildVoteTally's
// own doc comment names this plan as the reason its reportID parameter
// exists). Appending in the order produced by the bounding-box query
// preserves ascending distance, so removing a report can never reorder the
// survivors — severity-first triage ordering is applied client-side by
// web/static/js/feed.js to whatever this returns, which is TRUST-06's
// separation of "where a report sits" from "whether a report is shown."
// The order below is part of the specification:
//
//  1. Run the bounding-box/Haversine call and map with reportFromNearbyRow.
//  2. Empty result -> return immediately; no batched query runs.
//  3. Collect ids.
//  4. Capture now once from the service clock, which WithReportClock can pin,
//     because the history read takes it as a parameter.
//  5. ReporterAccountsForReports once for the whole page.
//  6. Collect the distinct reporter accounts in the order the reports came
//     back, nearest first, capped at MaxReporterAccountsPerRead (Q7).
//  7. When there is at least one account, ReporterHistoryReports once for
//     those accounts, keyed by verified account and never by session (D-10),
//     with the window and cap built by reporterHistoryParams (D-13).
//  8. CurrentVotesForReports once, over the page ids and then the history
//     report ids that are not on the page (V-29).
//     8b. ReportExtrasForReports once for the page ids with the viewer's
//     account id: moderation state, display only written time and the viewer's
//     flag. A report it did not return is not listed (R-01, D-19).
//  9. ReporterSignals once for all collected accounts, at the same now (R-05).
//  10. Group the vote rows by report once.
//  11. Per report: build the tally, resolve with the report's moderation state
//     in ReportMeta (a withheld report never lists), filter, then build the trust
//     object from the same batched rows for each report that is kept (D-21),
//     and append. The reporter signal is read after Resolve and never feeds
//     visibility, filtering or order (D-14). A report with no reporter
//     account, or one past the account cap, carries the none signal. A
//     non empty Category skips a report of another category here, and each
//     kept report gets its View from ViewOf and the viewer's own standing
//     resolution from the same rows (D-17, D-22).
//  12. attachUndos over the kept views: one ResolutionUndoCounts read, and only
//     when the viewer has a resolution row on a kept report or posted one of
//     them. An ordinary page costs no extra read (D-22).
func (s *ReportService) Nearby(ctx context.Context, q NearbyQuery) ([]ReportView, error) {
	latMin, latMax, lonMin, lonMax := BoundingBox(q.Latitude, q.Longitude, q.RadiusKm)

	rows, err := s.q.NearbyReports(ctx, sqlcgen.NearbyReportsParams{
		Lat:      q.Latitude,
		Lon:      q.Longitude,
		LatMin:   latMin,
		LatMax:   latMax,
		LonMin:   lonMin,
		LonMax:   lonMax,
		RadiusKm: q.RadiusKm,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		// Deliberate and asserted (TestNearbyBatchesVoteReadsOnce): an
		// empty page costs zero extra round trips.
		return []ReportView{}, nil
	}

	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	// now is captured once, before the batched reads, because the history
	// read takes it as a parameter and ReporterSignals must see the same one.
	now := s.now().UTC()

	reporterRows, err := s.q.ReporterAccountsForReports(ctx, ids)
	if err != nil {
		return nil, err
	}
	reporterByReport := make(map[int64]*int64, len(reporterRows))
	for _, rr := range reporterRows {
		reporterByReport[rr.ReportID] = rr.ReporterAccountID
	}

	accounts := pageReporterAccounts(ids, reporterByReport)
	var history []sqlcgen.ReporterHistoryReportsRow
	if len(accounts) > 0 {
		history, err = s.q.ReporterHistoryReports(ctx, reporterHistoryParams(accounts, now))
		if err != nil {
			return nil, err
		}
	}

	voteRows, err := s.q.CurrentVotesForReports(ctx, voteReadIDs(ids, history))
	if err != nil {
		return nil, err
	}
	signals := ReporterSignals(accounts, history, voteRows, now)
	votesByReport := voteRowsByReport(voteRows)

	// The moderation state, the display only written time and the viewer's flag
	// of the page, in one batched read over the page's own ids (R-01, D-19).
	extraRows, err := s.q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{ReportIds: ids, ViewerAccountID: q.ViewerAccountID})
	if err != nil {
		return nil, err
	}
	extrasByReport := make(map[int64]sqlcgen.ReportExtrasForReportsRow, len(extraRows))
	for _, ex := range extraRows {
		extrasByReport[ex.ReportID] = ex
	}

	views := make([]ReportView, 0, len(rows))
	for _, row := range rows {
		r := reportFromNearbyRow(row)
		reporterID := reporterByReport[r.ID]
		reportVotes := votesByReport[r.ID]

		// A report the extras read did not return is not shown: fail closed
		// (R-01, T-04-16).
		ex, ok := extrasByReport[r.ID]
		if !ok {
			continue
		}

		tally := BuildVoteTally(reportVotes, r.ID, reporterID)
		meta := ReportMeta{Severity: r.Severity, Category: r.Category, Moderation: ModerationState(ex.ModerationState)}
		vis, reason := Resolve(meta, tally, now)
		if q.All {
			if !vis.ListableInAll() {
				continue
			}
		} else if !vis.ListableInFeed(q.IncludeDisputed) {
			continue
		}
		if q.Category != "" && r.Category != q.Category {
			continue
		}
		// The rows were selected with the database clock and are judged here with
		// the service clock. A report that expired in the gap between the two would
		// come back with a finished view, no history and its exact point, which is
		// the one thing a finished Rescue report must never carry (IN-01, W1). Only
		// an active or disputed report belongs in this answer; the next read lists
		// the one that just finished in Past, blurred.
		view := ViewOf(vis, r.ExpiresAt, now)
		if view != FeedViewActive && view != FeedViewDisputed {
			continue
		}

		isOwn := reporterID != nil && *reporterID == q.ViewerAccountID
		// Built first so the reliable tick reads the tag from the same trust
		// object the report carries.
		trust := BuildTrust(TrustInput{
			Rows:              reportVotes,
			ReportID:          r.ID,
			ReporterAccountID: reporterID,
			Meta:              meta,
			Visibility:        vis,
			CreatedAt:         r.CreatedAt,
			ExpiresAt:         r.ExpiresAt,
			Now:               now,
			Reporter:          reporterSignalRef(signals, reporterID),
		})
		views = append(views, ReportView{
			Report:           r,
			Visibility:       vis,
			VisibilityReason: reason,
			View:             view,
			ViewerVote:       ViewerContentVote(reportVotes, r.ID, q.ViewerAccountID),
			ViewerResolution: ViewerResolution(reportVotes, r.ID, q.ViewerAccountID),
			IsOwnReport:      isOwn,
			// The reader's own follow and mute state, from rows already read
			// (D-09, D-10, D-11). Never says who else follows or muted.
			ViewerFollows: ViewerFollows(reportVotes, r.ID, q.ViewerAccountID, isOwn),
			ViewerMuted:   ex.ViewerMuted,
			Trust:         trust,
			// Display only, set after Resolve and never read by it (R-05).
			WrittenAt:     WrittenAtForDisplay(r.CreatedAt, writtenAtPtr(ex.ClientWrittenAt)),
			ViewerFlagged: ex.ViewerFlagged,
			// Server authored Show Confirmed membership, beside ViewOf and from the
			// same tally. Resolve never reads it (TRUST-02).
			ConfirmedByPeople: ConfirmedByPeople(vis, view, tally),
			// Server authored Only Reliable reporters membership, from the tag on
			// this report's trust object and the same meta (NOTIF-04, D-26).
			KeptByReliableFilter: KeptByReliableFilter(trust.Reporter.Tag, meta),
		})
	}
	if err := s.attachUndos(ctx, q.ViewerAccountID, views, now); err != nil {
		return nil, err
	}
	return views, nil
}

// writtenAtPtr converts the nullable client_written_at column to the pointer
// WrittenAtForDisplay takes: nil for a NULL column, otherwise a pointer to a
// copy. The address of t.Time is never taken directly, because a NULL column
// would then become a non nil pointer to the zero time.
func writtenAtPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	w := t.Time
	return &w
}

func reportFromNearbyRow(row sqlcgen.NearbyReportsRow) Report {
	var capacityStatus *CapacityStatus
	if row.ShelterCapacityStatus != nil {
		cs := CapacityStatus(*row.ShelterCapacityStatus)
		capacityStatus = &cs
	}
	var headcount *int
	if row.ShelterHeadcount != nil {
		h := int(*row.ShelterHeadcount)
		headcount = &h
	}
	distance := row.DistanceKm
	return Report{
		ID:                    row.ID,
		Category:              Category(row.Category),
		Severity:              Severity(row.Severity),
		Description:           row.Description,
		Latitude:              row.Latitude,
		Longitude:             row.Longitude,
		Geohash:               row.Geohash,
		ShelterCapacityStatus: capacityStatus,
		ShelterHeadcount:      headcount,
		CreatedAt:             row.CreatedAt,
		ExpiresAt:             row.ExpiresAt,
		DistanceKm:            &distance,
	}
}
