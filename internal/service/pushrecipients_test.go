// pushrecipients_test.go proves the two reading halves of the phone alert
// dispatcher (plan 03.3-09, task 1): the fresh report facts, which must come
// from the same resolver the feed uses, and the inverse home cell lookup, which
// selects accounts by a stored cell and never by anyone's position.

package service

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// fakePushFacts answers the two reads loadPushFacts makes.
type fakePushFacts struct {
	row       sqlcgen.PushReportFactsRow
	rowErr    error
	votes     []sqlcgen.CurrentVotesForReportsRow
	voteErr   error
	voteReads [][]int64
}

func (f *fakePushFacts) PushReportFacts(_ context.Context, _ int64) (sqlcgen.PushReportFactsRow, error) {
	return f.row, f.rowErr
}

func (f *fakePushFacts) CurrentVotesForReports(_ context.Context, ids []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	f.voteReads = append(f.voteReads, ids)
	return f.votes, f.voteErr
}

var pushFactsNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// pushVote builds one current vote row.
func pushVote(reportID, account int64, kind, value, cell string) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    reportID,
		AccountID:   account,
		Kind:        kind,
		Value:       value,
		GeohashCell: cell,
		CreatedAt:   pushFactsNow.Add(-10 * time.Minute),
	}
}

func pushFactsRow(severity, category, moderation string) sqlcgen.PushReportFactsRow {
	reporter := int64(900)
	happening := "water_rising"
	area := "Central Market"
	return sqlcgen.PushReportFactsRow{
		ID:                42,
		Severity:          severity,
		Category:          category,
		Happening:         &happening,
		AreaName:          &area,
		Latitude:          12.9716,
		Longitude:         77.5946,
		CreatedAt:         pushFactsNow.Add(-10 * time.Minute),
		ExpiresAt:         pushFactsNow.Add(5 * time.Hour),
		ModerationState:   moderation,
		ReporterAccountID: &reporter,
	}
}

func TestLoadPushFactsUsesResolve(t *testing.T) {
	confirm := func(n int) []sqlcgen.CurrentVotesForReportsRow {
		cells := []string{"tdr1v8y", "tdr1vc3", "tdr1vd1", "tdr1vf2"}
		var rows []sqlcgen.CurrentVotesForReportsRow
		for i := 0; i < n; i++ {
			rows = append(rows, pushVote(42, int64(100+i), "content", "confirm", cells[i]))
		}
		return rows
	}
	dispute := func(n int) []sqlcgen.CurrentVotesForReportsRow {
		cells := []string{"tdr1w01", "tdr1w02", "tdr1w03", "tdr1w04"}
		var rows []sqlcgen.CurrentVotesForReportsRow
		for i := 0; i < n; i++ {
			rows = append(rows, pushVote(42, int64(200+i), "content", "dispute", cells[i]))
		}
		return rows
	}

	cases := []struct {
		name          string
		severity      string
		moderation    string
		votes         []sqlcgen.CurrentVotesForReportsRow
		wantVis       Visibility
		wantView      FeedView
		wantConfirmed bool
	}{
		{"critical with no votes is live, active and unconfirmed", "critical", "clear", nil, VisibilityLive, FeedViewActive, false},
		{"critical with two confirming cells is confirmed", "critical", "clear", confirm(2), VisibilityLive, FeedViewActive, true},
		{"critical with one confirming cell is not confirmed", "critical", "clear", confirm(1), VisibilityLive, FeedViewActive, false},
		{"removed critical is withheld", "critical", "removed", nil, VisibilityWithheld, FeedViewNone, false},
		{"pending critical is published while its check retries", "critical", "pending", nil, VisibilityLive, FeedViewActive, false},
		{"pending low is withheld", "low", "pending", nil, VisibilityWithheld, FeedViewNone, false},
		{"low with no votes is provisional", "low", "clear", nil, VisibilityProvisional, FeedViewActive, false},
		{"low with two confirming cells is live and confirmed", "low", "clear", confirm(2), VisibilityLive, FeedViewActive, true},
		// N3: the critical bypass sits above the dispute rung, so the report
		// stays Live and listed, and ConfirmedByPeople alone keeps it unconfirmed.
		{"critical disputed by most cells stays live and unconfirmed", "critical", "clear", append(confirm(1), dispute(3)...), VisibilityLive, FeedViewActive, false},
		{"critical confirmed twice but disputed by more stays unconfirmed", "critical", "clear", append(confirm(2), dispute(3)...), VisibilityLive, FeedViewActive, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakePushFacts{row: pushFactsRow(c.severity, "flood", c.moderation), votes: c.votes}
			facts, ok, err := loadPushFacts(context.Background(), f, 42, pushFactsNow)
			if err != nil || !ok {
				t.Fatalf("loadPushFacts = ok %v, err %v", ok, err)
			}
			if facts.Vis != c.wantVis || facts.View != c.wantView || facts.Confirmed != c.wantConfirmed {
				t.Errorf("vis %q view %q confirmed %v, want %q %q %v",
					facts.Vis, facts.View, facts.Confirmed, c.wantVis, c.wantView, c.wantConfirmed)
			}
			if facts.Meta.Moderation != ModerationState(c.moderation) {
				t.Errorf("Meta.Moderation = %q, want the stored state %q", facts.Meta.Moderation, c.moderation)
			}
		})
	}

	t.Run("a missing report is ok false with no error", func(t *testing.T) {
		f := &fakePushFacts{rowErr: pgx.ErrNoRows}
		_, ok, err := loadPushFacts(context.Background(), f, 42, pushFactsNow)
		if ok || err != nil {
			t.Fatalf("ok %v err %v, want false and nil", ok, err)
		}
		if len(f.voteReads) != 0 {
			t.Error("votes were read for a report that does not exist")
		}
	})

	t.Run("a read error is returned", func(t *testing.T) {
		boom := errors.New("boom")
		if _, _, err := loadPushFacts(context.Background(), &fakePushFacts{rowErr: boom}, 42, pushFactsNow); !errors.Is(err, boom) {
			t.Errorf("fact read error = %v, want boom", err)
		}
		f := &fakePushFacts{row: pushFactsRow("critical", "flood", "clear"), voteErr: boom}
		if _, _, err := loadPushFacts(context.Background(), f, 42, pushFactsNow); !errors.Is(err, boom) {
			t.Errorf("vote read error = %v, want boom", err)
		}
	})

	t.Run("the facts carry the row", func(t *testing.T) {
		f := &fakePushFacts{row: pushFactsRow("critical", "flood", "clear")}
		facts, ok, err := loadPushFacts(context.Background(), f, 42, pushFactsNow)
		if err != nil || !ok {
			t.Fatalf("loadPushFacts = ok %v, err %v", ok, err)
		}
		if facts.ID != 42 || facts.AreaName != "Central Market" || facts.Lat != 12.9716 || facts.Lon != 77.5946 {
			t.Errorf("facts = %+v", facts)
		}
		if facts.Happening == nil || *facts.Happening != HappeningWaterRising {
			t.Errorf("Happening = %v", facts.Happening)
		}
		if facts.ReporterAccountID == nil || *facts.ReporterAccountID != 900 {
			t.Errorf("ReporterAccountID = %v", facts.ReporterAccountID)
		}
		if len(f.voteReads) != 1 || len(f.voteReads[0]) != 1 || f.voteReads[0][0] != 42 {
			t.Errorf("vote reads = %v, want one read of the one report id", f.voteReads)
		}
	})

	t.Run("an expired report is past, not active", func(t *testing.T) {
		row := pushFactsRow("critical", "flood", "clear")
		row.ExpiresAt = pushFactsNow.Add(-time.Minute)
		facts, ok, err := loadPushFacts(context.Background(), &fakePushFacts{row: row}, 42, pushFactsNow)
		if err != nil || !ok {
			t.Fatalf("loadPushFacts = ok %v, err %v", ok, err)
		}
		if facts.View != FeedViewPast || facts.Confirmed {
			t.Errorf("view %q confirmed %v, want past and false", facts.View, facts.Confirmed)
		}
	})
}

func TestCandidateCellsCoverRadius(t *testing.T) {
	const lat, lon = 12.9716, 77.5946
	cells := candidateCells(lat, lon, PushCandidateMaxRadiusKm)

	if len(cells) < 20 || len(cells) > 150 {
		t.Fatalf("candidateCells returned %d cells, want 20 to 150", len(cells))
	}
	seen := map[string]bool{}
	for _, c := range cells {
		if len(c) != 5 {
			t.Fatalf("cell %q has length %d, want 5", c, len(c))
		}
		if seen[c] {
			t.Fatalf("cell %q is listed twice", c)
		}
		seen[c] = true
	}

	// 500 random points within the radius plus the cell half diagonal: each
	// one's own precision 5 cell must be in the list, because a home cell's
	// centre can be that far from the point of a home that is within range.
	rng := rand.New(rand.NewSource(33))
	const reach = float64(PushCandidateMaxRadiusKm) + homeCellHalfDiagonalKm
	for i := 0; i < 500; i++ {
		dist := reach * math.Sqrt(rng.Float64())
		bearing := rng.Float64() * 2 * math.Pi
		dLat := dist * math.Cos(bearing) / 111.045
		dLon := dist * math.Sin(bearing) / (111.045 * math.Cos(lat*math.Pi/180))
		cell := geohash.EncodeWithPrecision(lat+dLat, lon+dLon, 5)
		if !seen[cell] {
			t.Fatalf("point %d at %.2f km, bearing %.2f has cell %q, which is not a candidate", i, dist, bearing, cell)
		}
	}

	t.Run("nearest first", func(t *testing.T) {
		own := geohash.EncodeWithPrecision(lat, lon, 5)
		if cells[0] != own {
			t.Fatalf("first cell = %q, want the report's own cell %q", cells[0], own)
		}
		prev := -1.0
		for i, c := range cells[1:] {
			clat, clon := geohash.DecodeCenter(c)
			d := haversineKm(lat, lon, clat, clon)
			if d < prev-1e-9 {
				t.Fatalf("cell %d (%q) is %.4f km away, after one %.4f km away", i+1, c, d, prev)
			}
			prev = d
		}
	})

	t.Run("a smaller radius lists fewer cells and still starts at the own cell", func(t *testing.T) {
		small := candidateCells(lat, lon, 1)
		if len(small) >= len(cells) {
			t.Errorf("1 km lists %d cells, 10 km lists %d", len(small), len(cells))
		}
		if small[0] != geohash.EncodeWithPrecision(lat, lon, 5) {
			t.Errorf("first small cell = %q", small[0])
		}
	})

	t.Run("the poles and the date line do not break the lattice", func(t *testing.T) {
		for _, p := range [][2]float64{{89.99, 10}, {-89.99, -170}, {0, 179.99}, {0, -179.99}} {
			got := candidateCells(p[0], p[1], PushCandidateMaxRadiusKm)
			if len(got) == 0 || got[0] != geohash.EncodeWithPrecision(p[0], p[1], 5) {
				t.Errorf("candidateCells(%v) = %d cells, first %q", p, len(got), got)
			}
		}
	})
}

// homeRowAt builds the candidate row of an account whose home cell is the
// precision 5 cell of the point dLonCells cell widths east of the report.
func homeRowAt(account int64, lat, lon float64, dLonCells int, radius int16, critical, reliable bool) sqlcgen.PushHomeCandidatesRow {
	cell := geohash.EncodeWithPrecision(lat, lon+float64(dLonCells)*0.0439, 5)
	return sqlcgen.PushHomeCandidatesRow{
		AccountID:        account,
		HomeAreaCell:     cell,
		AlertRadiusKm:    radius,
		CriticalPushOn:   critical,
		ReliableAlertsOn: reliable,
	}
}

func TestFilterRecipientsByDistanceAndSwitch(t *testing.T) {
	const lat, lon = 12.9716, 77.5946
	wantCritical := func(r sqlcgen.PushHomeCandidatesRow) bool { return r.CriticalPushOn }

	rows := []sqlcgen.PushHomeCandidatesRow{
		homeRowAt(1, lat, lon, 0, 1, true, false),  // own cell, radius 1
		homeRowAt(2, lat, lon, 1, 1, true, false),  // next cell, radius 1: out
		homeRowAt(3, lat, lon, 1, 10, true, false), // next cell, radius 10: in
		homeRowAt(4, lat, lon, 2, 5, true, false),  // two cells east, radius 5
		homeRowAt(5, lat, lon, 0, 5, false, false), // switch off
		homeRowAt(6, lat, lon, 0, 5, true, false),  // the author
		{AccountID: 7, HomeAreaCell: "BADCELL", AlertRadiusKm: 10, CriticalPushOn: true},
		{AccountID: 8, HomeAreaCell: "", AlertRadiusKm: 10, CriticalPushOn: true},
		homeRowAt(9, lat, lon, 3, 10, true, false), // three cells east, radius 10
	}
	author := int64(6)

	// Expected, from the stated rule: the distance from the middle of the home
	// cell to the report point is at most the account's own radius.
	var want []int64
	dists := map[int64]float64{}
	for _, r := range rows {
		alat, alon, ok := HomeAreaAnchor(r.HomeAreaCell)
		if !ok || !r.CriticalPushOn || r.AccountID == author {
			continue
		}
		d := haversineKm(lat, lon, alat, alon)
		dists[r.AccountID] = d
		if d <= float64(r.AlertRadiusKm) {
			want = append(want, r.AccountID)
		}
	}
	sort.Slice(want, func(i, j int) bool {
		if dists[want[i]] != dists[want[j]] {
			return dists[want[i]] < dists[want[j]]
		}
		return want[i] < want[j]
	})
	if len(want) == 0 || len(want) == len(dists) {
		t.Fatalf("the fixture is not meaningful: %d of %d candidates are in range", len(want), len(dists))
	}

	got := filterRecipients(rows, lat, lon, &author, wantCritical)
	var gotIDs []int64
	for _, g := range got {
		gotIDs = append(gotIDs, g.AccountID)
		if math.Abs(g.DistanceKm-dists[g.AccountID]) > 1e-9 {
			t.Errorf("account %d distance %.6f, want %.6f", g.AccountID, g.DistanceKm, dists[g.AccountID])
		}
	}
	if len(gotIDs) != len(want) {
		t.Fatalf("recipients = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("recipients = %v, want %v (nearest first)", gotIDs, want)
		}
	}

	t.Run("a nil author drops nobody", func(t *testing.T) {
		with := filterRecipients(rows, lat, lon, nil, wantCritical)
		found := false
		for _, r := range with {
			if r.AccountID == author {
				found = true
			}
		}
		if !found {
			t.Error("the account that would be the author was dropped with no author")
		}
	})

	t.Run("a nil want keeps every valid row in range", func(t *testing.T) {
		all := filterRecipients(rows, lat, lon, &author, nil)
		if len(all) < len(want) {
			t.Errorf("nil want kept %d, fewer than the %d with the switch on", len(all), len(want))
		}
	})

	t.Run("at most PushRecipientsPerReport, nearest first", func(t *testing.T) {
		var many []sqlcgen.PushHomeCandidatesRow
		for i := 0; i < PushRecipientsPerReport+100; i++ {
			many = append(many, homeRowAt(int64(1000+i), lat, lon, 0, 10, true, false))
		}
		// One account in a farther cell: it must be the one dropped.
		many = append(many, homeRowAt(5000, lat, lon, 1, 10, true, false))
		out := filterRecipients(many, lat, lon, nil, wantCritical)
		if len(out) != PushRecipientsPerReport {
			t.Fatalf("kept %d recipients, want %d", len(out), PushRecipientsPerReport)
		}
		for _, r := range out {
			if r.AccountID == 5000 {
				t.Error("the farthest account was kept under the cap")
			}
		}
	})
}
