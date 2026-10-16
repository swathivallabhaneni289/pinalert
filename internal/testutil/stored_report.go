package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// StoredReport is one report row to write straight into the database. The
// caller supplies the timestamps, because testutil never imports the service
// package and so cannot compute an expiry.
type StoredReport struct {
	SessionID       string
	Category        string
	Severity        string
	Description     string
	Latitude        float64
	Longitude       float64
	CreatedAt       time.Time
	ExpiresAt       time.Time
	ModerationState string
}

// InsertStoredReport writes one report row through the same sqlc query the
// submit path uses and returns the new id. It exists because the posting path
// no longer accepts rescue_needed, and tests of the rules that apply to rows
// already stored need such rows. An empty ModerationState becomes "clear".
func InsertStoredReport(t *testing.T, pool *pgxpool.Pool, r StoredReport) int64 {
	t.Helper()

	state := r.ModerationState
	if state == "" {
		state = "clear"
	}
	row, err := sqlcgen.New(pool).InsertReportIdempotent(context.Background(), sqlcgen.InsertReportIdempotentParams{
		SessionID:       r.SessionID,
		Category:        r.Category,
		Severity:        r.Severity,
		Description:     r.Description,
		Latitude:        r.Latitude,
		Longitude:       r.Longitude,
		Geohash:         geohash.EncodeWithPrecision(r.Latitude, r.Longitude, geohashPrecision),
		CreatedAt:       r.CreatedAt,
		ExpiresAt:       r.ExpiresAt,
		ModerationState: state,
	})
	if err != nil {
		t.Fatalf("inserting stored %s report: %v", r.Category, err)
	}
	return row.ID
}
