// This file seeds the Phase 5 official alert rows for store, service and
// handler tests: one alert with its areas, and the state of a feed. It writes
// with plain SQL so a test can state any stored fact, including ones the
// refresher would never produce.
package testutil

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// officialSeedCounter makes every default guid unique across a test binary.
var officialSeedCounter atomic.Int64

// OfficialAlertSeed is one stored alert. Zero fields take a sensible default:
// a unique 16 digit guid, a Moderate Alert from IMD-Chennai sent at SentAt and
// ending two hours later, unplaced.
type OfficialAlertSeed struct {
	GUID          string
	CapIdentifier string // defaults to "IN-<GUID>_1"
	Sender        string // defaults to IMD-Chennai
	Issuer        string // defaults to the sender
	MsgType       string // defaults to Alert
	Severity      string // defaults to Moderate
	Event         string // defaults to Moderate Rain
	HeadlineEN    string
	AreaDesc      string
	Tier          string // district, state or unplaced; defaults to unplaced
	DistrictCodes []int  // stored as district rows
	StateCodes    []int  // stored as state rows
	SentAt        time.Time
	ExpiresAt     time.Time
	CancelledAt   *time.Time
	SupersededAt  *time.Time
	SourcePubDate *time.Time
}

// SeedOfficialAlert inserts the alert and its area rows and returns the alert
// id. Each district code is stored as a row of its own and each state code as a
// row of its own, the way the refresher stores a placement.
func SeedOfficialAlert(t *testing.T, pool *pgxpool.Pool, s OfficialAlertSeed) int64 {
	t.Helper()

	if s.GUID == "" {
		s.GUID = fmt.Sprintf("1792%012d", officialSeedCounter.Add(1))
	}
	if s.CapIdentifier == "" {
		s.CapIdentifier = "IN-" + s.GUID + "_1"
	}
	if s.Sender == "" {
		s.Sender = "IMD-Chennai"
	}
	if s.Issuer == "" {
		s.Issuer = s.Sender
	}
	if s.MsgType == "" {
		s.MsgType = "Alert"
	}
	if s.Severity == "" {
		s.Severity = "Moderate"
	}
	if s.Event == "" {
		s.Event = "Moderate Rain"
	}
	if s.Tier == "" {
		s.Tier = "unplaced"
	}
	if s.SentAt.IsZero() {
		s.SentAt = time.Now().UTC().Add(-time.Hour)
	}
	if s.ExpiresAt.IsZero() {
		s.ExpiresAt = s.SentAt.Add(2 * time.Hour)
	}

	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO official_alerts (
		     guid, cap_identifier, source_pubdate, sender, issuer, msg_type, event, severity,
		     sent_at, expires_at, headline_en, area_desc, placement_tier,
		     cancelled_at, superseded_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $9, $9)
		 RETURNING id`,
		s.GUID, s.CapIdentifier, s.SourcePubDate, s.Sender, s.Issuer, s.MsgType, s.Event, s.Severity,
		s.SentAt, s.ExpiresAt, s.HeadlineEN, s.AreaDesc, s.Tier, s.CancelledAt, s.SupersededAt,
	).Scan(&id); err != nil {
		t.Fatalf("seeding official alert %q: %v", s.GUID, err)
	}

	for _, d := range s.DistrictCodes {
		if _, err := pool.Exec(ctx,
			`INSERT INTO official_alert_areas (alert_id, district_lgd) VALUES ($1, $2)`, id, d); err != nil {
			t.Fatalf("seeding district %d for official alert %d: %v", d, id, err)
		}
	}
	for _, st := range s.StateCodes {
		if _, err := pool.Exec(ctx,
			`INSERT INTO official_alert_areas (alert_id, state_lgd) VALUES ($1, $2)`, id, st); err != nil {
			t.Fatalf("seeding state %d for official alert %d: %v", st, id, err)
		}
	}
	return id
}

// SetOfficialFeedStatus sets the attempt and success times and the item count
// of one feed ("sachet" or "gdacs"); nil clears a time. It leaves the error
// word and the failure count alone. The row exists after NewTestDB and
// Truncate.
func SetOfficialFeedStatus(t *testing.T, pool *pgxpool.Pool, feed string, lastAttempt, lastSuccess *time.Time, itemsSeen int) {
	t.Helper()

	tag, err := pool.Exec(context.Background(),
		`UPDATE official_feed_status
		 SET last_attempt_at = $2, last_success_at = $3, items_seen = $4
		 WHERE feed = $1`,
		feed, lastAttempt, lastSuccess, itemsSeen)
	if err != nil {
		t.Fatalf("setting feed status for %q: %v", feed, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("setting feed status for %q: %d rows changed, want 1", feed, tag.RowsAffected())
	}
}
