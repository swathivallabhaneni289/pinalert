package store_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

var officialNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// TestOfficialQueries covers the Phase 5 statements on a real database: the
// feed status marks, the claim of a refresh run, the upsert of a series, the
// cancel and supersede marks, and the in force read.
func TestOfficialQueries(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	t.Run("feed status", func(t *testing.T) {
		testutil.Truncate(t, pool)

		st, err := q.GetFeedStatus(ctx, "sachet")
		if err != nil {
			t.Fatalf("GetFeedStatus: %v", err)
		}
		if st.LastSuccessAt.Valid || st.LastAttemptAt.Valid || st.ConsecutiveFailures != 0 {
			t.Fatalf("a fresh row should be empty, got %+v", st)
		}

		// The first claim wins; a claim inside the gap changes nothing.
		cutoff := officialNow.Add(-4 * time.Minute)
		n, err := q.MarkFeedAttempt(ctx, sqlcgen.MarkFeedAttemptParams{Feed: "sachet", Now: officialNow, Cutoff: cutoff})
		if err != nil || n != 1 {
			t.Fatalf("first MarkFeedAttempt = %d, %v; want 1", n, err)
		}
		n, err = q.MarkFeedAttempt(ctx, sqlcgen.MarkFeedAttemptParams{
			Feed: "sachet", Now: officialNow.Add(3 * time.Minute), Cutoff: officialNow.Add(3*time.Minute - 4*time.Minute)})
		if err != nil || n != 0 {
			t.Fatalf("MarkFeedAttempt inside the gap = %d, %v; want 0", n, err)
		}
		n, err = q.MarkFeedAttempt(ctx, sqlcgen.MarkFeedAttemptParams{
			Feed: "sachet", Now: officialNow.Add(4 * time.Minute), Cutoff: officialNow})
		if err != nil || n != 1 {
			t.Fatalf("MarkFeedAttempt at the gap = %d, %v; want 1", n, err)
		}

		if err := q.MarkFeedSuccess(ctx, sqlcgen.MarkFeedSuccessParams{
			Feed: "sachet", Now: officialNow, Etag: `W/"1"`, LastModified: "Sun, 25 Oct 2026 09:15:09 GMT", ItemsSeen: 12}); err != nil {
			t.Fatalf("MarkFeedSuccess: %v", err)
		}
		// A failure leaves the success time, the validators and the item count.
		if err := q.MarkFeedFailure(ctx, sqlcgen.MarkFeedFailureParams{Feed: "sachet", ErrorClass: "timeout"}); err != nil {
			t.Fatalf("MarkFeedFailure: %v", err)
		}
		if err := q.MarkFeedFailure(ctx, sqlcgen.MarkFeedFailureParams{Feed: "sachet", ErrorClass: "status"}); err != nil {
			t.Fatalf("MarkFeedFailure again: %v", err)
		}
		st, err = q.GetFeedStatus(ctx, "sachet")
		if err != nil {
			t.Fatalf("GetFeedStatus: %v", err)
		}
		if !st.LastSuccessAt.Valid || !st.LastSuccessAt.Time.Equal(officialNow) {
			t.Errorf("last_success_at = %v, want %v after two failures", st.LastSuccessAt, officialNow)
		}
		if st.ConsecutiveFailures != 2 || st.LastError == nil || *st.LastError != "status" {
			t.Errorf("failures %d error %v, want 2 and status", st.ConsecutiveFailures, st.LastError)
		}
		if st.LastEtag != `W/"1"` || st.ItemsSeen != 12 {
			t.Errorf("validators %q items %d changed by a failure", st.LastEtag, st.ItemsSeen)
		}

		if err := q.MarkFeedSuccess(ctx, sqlcgen.MarkFeedSuccessParams{
			Feed: "sachet", Now: officialNow.Add(time.Hour), Etag: `W/"2"`, ItemsSeen: 3}); err != nil {
			t.Fatalf("MarkFeedSuccess again: %v", err)
		}
		st, _ = q.GetFeedStatus(ctx, "sachet")
		if st.ConsecutiveFailures != 0 || st.LastError != nil {
			t.Errorf("a success should reset the failures and the error, got %d %v", st.ConsecutiveFailures, st.LastError)
		}

		// The gdacs row is untouched by all of it.
		g, _ := q.GetFeedStatus(ctx, "gdacs")
		if g.LastAttemptAt.Valid || g.ConsecutiveFailures != 0 {
			t.Errorf("gdacs row changed: %+v", g)
		}
	})

	t.Run("upsert keeps first seen and replaces the rest", func(t *testing.T) {
		testutil.Truncate(t, pool)

		params := sqlcgen.UpsertOfficialAlertParams{
			Guid: "1792919196941008", CapIdentifier: "IN-1792919196941008_8", Sender: "IMD-Chennai", Issuer: "IMD Chennai",
			MsgType: "Alert", Event: "Lightning", Severity: "Severe", SentAt: officialNow,
			ExpiresAt: officialNow.Add(3 * time.Hour), HeadlineEn: "first", PlacementTier: "unplaced", Now: officialNow,
		}
		first, err := q.UpsertOfficialAlert(ctx, params)
		if err != nil {
			t.Fatalf("first upsert: %v", err)
		}
		params.CapIdentifier = "IN-1792919196941008_9"
		params.HeadlineEn = "second"
		params.Now = officialNow.Add(10 * time.Minute)
		second, err := q.UpsertOfficialAlert(ctx, params)
		if err != nil {
			t.Fatalf("second upsert: %v", err)
		}
		if second.ID != first.ID {
			t.Errorf("upsert by guid made a new row: %d then %d", first.ID, second.ID)
		}
		if second.HeadlineEn != "second" || second.CapIdentifier != "IN-1792919196941008_9" {
			t.Errorf("row not replaced: %+v", second)
		}
		if !second.FirstSeenAt.Equal(officialNow) || !second.LastSeenAt.Equal(officialNow.Add(10*time.Minute)) {
			t.Errorf("first_seen_at %v last_seen_at %v, want %v and +10m", second.FirstSeenAt, second.LastSeenAt, officialNow)
		}
		if n := pushCount(t, pool, `SELECT count(*) FROM official_alerts`); n != 1 {
			t.Errorf("%d rows, want 1", n)
		}
	})

	t.Run("cancel and supersede by identifier", func(t *testing.T) {
		testutil.Truncate(t, pool)
		a := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{GUID: "1792000000000001", CapIdentifier: "IN-1792000000000001_5", SentAt: officialNow.Add(-time.Hour), ExpiresAt: officialNow.Add(time.Hour)})
		b := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{GUID: "1792000000000002", CapIdentifier: "IN-1792000000000002_5", SentAt: officialNow.Add(-time.Hour), ExpiresAt: officialNow.Add(time.Hour)})
		c := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{GUID: "1792000000000003", CapIdentifier: "IN-1792000000000003_5", SentAt: officialNow.Add(-time.Hour), ExpiresAt: officialNow.Add(time.Hour)})

		n, err := q.CancelOfficialAlertsByIdentifier(ctx, sqlcgen.CancelOfficialAlertsByIdentifierParams{
			Now: officialNow, Identifiers: []string{"IN-1792000000000001_5", "IN-nothing_1"}})
		if err != nil || n != 1 {
			t.Fatalf("cancel = %d, %v; want 1", n, err)
		}
		n, err = q.SupersedeOfficialAlertsByIdentifier(ctx, sqlcgen.SupersedeOfficialAlertsByIdentifierParams{
			Now: officialNow, Identifiers: []string{"IN-1792000000000002_5", "IN-1792000000000003_5"}, Guid: "1792000000000003"})
		if err != nil || n != 1 {
			t.Fatalf("supersede = %d, %v; want 1 (the update's own guid is left out)", n, err)
		}
		var cancelled, superseded []int64
		rows, err := pool.Query(ctx, `SELECT id, cancelled_at IS NOT NULL, superseded_at IS NOT NULL FROM official_alerts ORDER BY id`)
		if err != nil {
			t.Fatalf("reading marks: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var can, sup bool
			if err := rows.Scan(&id, &can, &sup); err != nil {
				t.Fatalf("scanning: %v", err)
			}
			if can {
				cancelled = append(cancelled, id)
			}
			if sup {
				superseded = append(superseded, id)
			}
		}
		if len(cancelled) != 1 || cancelled[0] != a {
			t.Errorf("cancelled %v, want [%d]", cancelled, a)
		}
		if len(superseded) != 1 || superseded[0] != b {
			t.Errorf("superseded %v, want [%d] and not %d", superseded, b, c)
		}
		// The rows stay.
		if n := pushCount(t, pool, `SELECT count(*) FROM official_alerts`); n != 3 {
			t.Errorf("%d rows after the marks, want 3", n)
		}
	})

	t.Run("stored pub dates and the areas", func(t *testing.T) {
		testutil.Truncate(t, pool)
		pub := officialNow.Add(-30 * time.Minute)
		id := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
			GUID: "1792000000000010", Tier: "district", DistrictCodes: []int{568, 574}, StateCodes: []int{33}, SourcePubDate: &pub})
		testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{GUID: "1792000000000011"})

		rows, err := q.StoredPubDates(ctx, []string{"1792000000000010", "1792000000000011", "1792000000000012"})
		if err != nil {
			t.Fatalf("StoredPubDates: %v", err)
		}
		got := map[string]pgtype.Timestamptz{}
		for _, r := range rows {
			got[r.Guid] = r.SourcePubdate
		}
		if len(got) != 2 {
			t.Errorf("%d stored guids, want 2", len(got))
		}
		if p := got["1792000000000010"]; !p.Valid || !p.Time.Equal(pub) {
			t.Errorf("pub date %v, want %v", p, pub)
		}
		if got["1792000000000011"].Valid {
			t.Errorf("a seeded alert with no pub date should read NULL")
		}

		areas, err := q.OfficialAlertAreas(ctx, id)
		if err != nil {
			t.Fatalf("OfficialAlertAreas: %v", err)
		}
		var districts, states []int
		for _, a := range areas {
			switch {
			case a.DistrictLgd != nil && a.StateLgd == nil:
				districts = append(districts, int(*a.DistrictLgd))
			case a.StateLgd != nil && a.DistrictLgd == nil:
				states = append(states, int(*a.StateLgd))
			default:
				t.Errorf("area %+v, want a district row or a state row", a)
			}
		}
		sort.Ints(districts)
		if len(districts) != 2 || districts[0] != 568 || districts[1] != 574 {
			t.Errorf("districts %v, want [568 574]", districts)
		}
		if len(states) != 1 || states[0] != 33 {
			t.Errorf("states %v, want [33]", states)
		}

		if err := q.DeleteOfficialAlertAreas(ctx, id); err != nil {
			t.Fatalf("DeleteOfficialAlertAreas: %v", err)
		}
		d := int32(568)
		for i := 0; i < 2; i++ { // the second insert is a no-op, not an error
			if err := q.InsertOfficialAlertArea(ctx, sqlcgen.InsertOfficialAlertAreaParams{AlertID: id, DistrictLgd: &d}); err != nil {
				t.Fatalf("InsertOfficialAlertArea: %v", err)
			}
		}
		if n := pushCount(t, pool, `SELECT count(*) FROM official_alert_areas WHERE alert_id = $1`, id); n != 1 {
			t.Errorf("%d area rows, want 1", n)
		}
	})

	t.Run("in force", func(t *testing.T) {
		testutil.Truncate(t, pool)
		seed := func(guid, tier string, districts, states []int, edit func(*testutil.OfficialAlertSeed)) {
			s := testutil.OfficialAlertSeed{
				GUID: guid, Tier: tier, DistrictCodes: districts, StateCodes: states,
				SentAt: officialNow.Add(-time.Hour), ExpiresAt: officialNow.Add(time.Hour),
			}
			if edit != nil {
				edit(&s)
			}
			testutil.SeedOfficialAlert(t, pool, s)
		}
		past := officialNow.Add(-time.Minute)
		seed("1792000000000101", "district", []int{568, 574}, []int{33}, nil)                             // matches on 568
		seed("1792000000000102", "district", []int{480}, []int{27}, nil)                                  // other district
		seed("1792000000000103", "state", nil, []int{33}, nil)                                            // state alert, state passed
		seed("1792000000000104", "state", nil, []int{28}, nil)                                            // another state
		seed("1792000000000105", "unplaced", nil, nil, nil)                                               // always
		seed("1792000000000106", "district", []int{568}, []int{33}, func(s *testutil.OfficialAlertSeed) { // expired
			s.ExpiresAt = officialNow
		})
		seed("1792000000000107", "district", []int{568}, []int{33}, func(s *testutil.OfficialAlertSeed) { // cancelled
			s.CancelledAt = &past
		})
		seed("1792000000000108", "unplaced", nil, nil, func(s *testutil.OfficialAlertSeed) { // superseded
			s.SupersededAt = &past
		})

		rows, err := q.OfficialAlertsInForce(ctx, sqlcgen.OfficialAlertsInForceParams{
			Now: officialNow, DistrictCodes: []int32{568}, StateCodes: []int32{33}})
		if err != nil {
			t.Fatalf("OfficialAlertsInForce: %v", err)
		}
		var guids []string
		for _, r := range rows {
			guids = append(guids, r.Guid)
			if r.Guid == "1792000000000101" {
				if len(r.DistrictCodes) != 2 || r.DistrictCodes[0] != 568 || r.DistrictCodes[1] != 574 || len(r.StateCodes) != 1 || r.StateCodes[0] != 33 {
					t.Errorf("area arrays %v %v, want [568 574] [33]", r.DistrictCodes, r.StateCodes)
				}
			}
		}
		sort.Strings(guids)
		want := []string{"1792000000000101", "1792000000000103", "1792000000000105"}
		if len(guids) != len(want) {
			t.Fatalf("in force %v, want %v", guids, want)
		}
		for i := range want {
			if guids[i] != want[i] {
				t.Fatalf("in force %v, want %v", guids, want)
			}
		}

		// A reader with no known area still gets the unplaced alert and no more.
		rows, err = q.OfficialAlertsInForce(ctx, sqlcgen.OfficialAlertsInForceParams{Now: officialNow})
		if err != nil {
			t.Fatalf("OfficialAlertsInForce with no codes: %v", err)
		}
		if len(rows) != 1 || rows[0].Guid != "1792000000000105" {
			t.Errorf("no codes: got %d rows, want only the unplaced alert", len(rows))
		}

		// An alert is in force exactly until its expiry.
		rows, err = q.OfficialAlertsInForce(ctx, sqlcgen.OfficialAlertsInForceParams{
			Now: officialNow.Add(time.Hour), DistrictCodes: []int32{568}, StateCodes: []int32{33}})
		if err != nil {
			t.Fatalf("OfficialAlertsInForce at expiry: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("%d alerts in force at their expiry time, want 0", len(rows))
		}
	})
}
