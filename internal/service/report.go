// Package service holds Pinalert's report business logic: the category,
// severity, and shelter-capacity enums; server-side validation; severity-
// tiered expiry computation; and the bounding-box math the nearby-reports
// query depends on. Nothing here trusts a client-supplied value for
// anything the server itself is authoritative over (expiry, geohash,
// creation time).
package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

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

// Category is one of the nine report categories (D-01). The exact slugs and
// their order are a contract: plan 01-05 renders the category grid from
// Categories and plan 01-07 documents the enum from it.
type Category string

const (
	CategoryFlood        Category = "flood"
	CategoryEarthquake   Category = "earthquake"
	CategoryFire         Category = "fire"
	CategoryStormCyclone Category = "storm_cyclone"
	CategoryRoadBlocked  Category = "road_blocked"
	CategoryPowerOutage  Category = "power_outage"
	CategoryShelterOpen  Category = "shelter_open"
	CategoryRescueNeeded Category = "rescue_needed"
	CategoryOther        Category = "other"
)

// Categories is the ordered, canonical list of every valid category.
var Categories = []Category{
	CategoryFlood,
	CategoryEarthquake,
	CategoryFire,
	CategoryStormCyclone,
	CategoryRoadBlocked,
	CategoryPowerOutage,
	CategoryShelterOpen,
	CategoryRescueNeeded,
	CategoryOther,
}

// Valid reports whether c is one of the nine canonical categories.
func (c Category) Valid() bool {
	for _, v := range Categories {
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
type SubmitInput struct {
	Category              Category
	Severity              Severity
	Description           string
	Latitude              float64
	Longitude             float64
	ShelterCapacityStatus *CapacityStatus
	ShelterHeadcount      *int
}

// Report is a fully materialized, server-computed report, returned by
// Submit and Nearby. It carries no session identifier — see the threat
// register's T-01-02 (Information Disclosure).
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
	// DistanceKm is populated by Nearby only; Submit's result leaves it nil,
	// since a freshly submitted report has no query point to be distant from.
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
	Category        Category
}

// ValidateSubmitInput enforces every server-side rule a report submission
// must satisfy. Validation is unconditional and server-side: a client can
// send anything, so nothing here trusts client-side form validation to have
// already run.
func ValidateSubmitInput(in SubmitInput) error {
	if !in.Category.Valid() {
		return ValidationError{"category", "Choose a category to continue."}
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
		return ValidationError{"latitude", "Set a location by dragging the pin or allowing location access."}
	}
	if in.Longitude < -180 || in.Longitude > 180 {
		return ValidationError{"longitude", "Set a location by dragging the pin or allowing location access."}
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
// is the bounded history read the feed needs to compose reporter signals.
type Querier interface {
	InsertReport(ctx context.Context, arg sqlcgen.InsertReportParams) (sqlcgen.InsertReportRow, error)
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
// the read path only: Submit keeps the wall clock, because vote times and the
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

// Submit validates in, computes expires_at (from severity, per D-14) and
// geohash server-side, and persists the report under sessionID. now is
// captured once, server-side, as the authoritative clock — never a
// client-supplied timestamp.
func (s *ReportService) Submit(ctx context.Context, sessionID string, in SubmitInput) (Report, error) {
	if err := ValidateSubmitInput(in); err != nil {
		return Report{}, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ExpiryDuration(in.Severity))
	gh := geohash.EncodeWithPrecision(in.Latitude, in.Longitude, geohashPrecision)
	desc := strings.TrimSpace(in.Description)

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

	row, err := s.q.InsertReport(ctx, sqlcgen.InsertReportParams{
		SessionID:             sessionID,
		Category:              string(in.Category),
		Severity:              string(in.Severity),
		Description:           desc,
		Latitude:              in.Latitude,
		Longitude:             in.Longitude,
		Geohash:               gh,
		ShelterCapacityStatus: capacityStatus,
		ShelterHeadcount:      headcount,
		CreatedAt:             now,
		ExpiresAt:             expiresAt,
	})
	if err != nil {
		return Report{}, err
	}

	return reportFromInsertRow(row), nil
}

func reportFromInsertRow(row sqlcgen.InsertReportRow) Report {
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
	}
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
		if !vis.ListableInFeed(q.IncludeDisputed) {
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

		views = append(views, ReportView{
			Report:           r,
			Visibility:       vis,
			VisibilityReason: reason,
			View:             view,
			ViewerVote:       ViewerContentVote(reportVotes, r.ID, q.ViewerAccountID),
			ViewerResolution: ViewerResolution(reportVotes, r.ID, q.ViewerAccountID),
			IsOwnReport:      reporterID != nil && *reporterID == q.ViewerAccountID,
			Trust: BuildTrust(TrustInput{
				Rows:              reportVotes,
				ReportID:          r.ID,
				ReporterAccountID: reporterID,
				Meta:              meta,
				Visibility:        vis,
				CreatedAt:         r.CreatedAt,
				ExpiresAt:         r.ExpiresAt,
				Now:               now,
				Reporter:          reporterSignalRef(signals, reporterID),
			}),
			// Display only, set after Resolve and never read by it (R-05).
			WrittenAt:     WrittenAtForDisplay(r.CreatedAt, writtenAtPtr(ex.ClientWrittenAt)),
			ViewerFlagged: ex.ViewerFlagged,
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
