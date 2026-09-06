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

	desc := strings.TrimSpace(in.Description)
	if len(desc) < 10 {
		return ValidationError{"description", "Add a short description (at least 10 characters)."}
	}
	if len(desc) > 1000 {
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
// needs — small enough to fake in a test without a real Postgres.
type Querier interface {
	InsertReport(ctx context.Context, arg sqlcgen.InsertReportParams) (sqlcgen.InsertReportRow, error)
}

// ReportService validates, computes derived fields, and persists reports.
type ReportService struct {
	q Querier
}

// NewReportService constructs a ReportService backed by q.
func NewReportService(q Querier) *ReportService {
	return &ReportService{q: q}
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
