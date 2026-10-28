package demo

import (
	"context"
	"errors"
	"testing"
	"time"

	"pinalert/internal/service"
)

// submitFlood posts a medium flood report as account acct through the real
// ReportService of the world and returns its id.
func submitFlood(t *testing.T, w *World, acct int64, lat, lon float64) int64 {
	t.Helper()
	res, err := w.Reports.SubmitChecked(context.Background(), w.SessionFor(acct), acct, service.SubmitInput{
		Category:    service.CategoryFlood,
		Severity:    service.SeverityMedium,
		Description: "Water is knee deep across the lane",
		Latitude:    lat,
		Longitude:   lon,
	})
	if err != nil {
		t.Fatalf("SubmitChecked: %v", err)
	}
	return res.Report.ID
}

func castContent(w *World, acct, reportID int64, value service.VoteValue, lat, lon float64) (service.CastVoteResult, error) {
	return w.Votes.CastVote(context.Background(), service.CastVoteInput{
		ReportID: reportID, AccountID: acct, Kind: service.VoteKindContent, Value: value, Latitude: lat, Longitude: lon,
	})
}

func TestCastVoteRunsRealRules(t *testing.T) {
	w := NewWorld("abcdefghijklmnopqrstuv", time.Now)
	id := submitFlood(t, w, VisitorAccountID, baseLat, baseLon)
	neighbour := NeighbourAccountBase

	// Too far: 1.5 km is past service.MaxVoteDistanceKm. Nothing is exempted.
	if _, err := castContent(w, neighbour, id, service.VoteConfirm, northKm(baseLat, 1.5), baseLon); !errors.Is(err, service.ErrVoterTooFar) {
		t.Fatalf("confirm from 1.5 km: err=%v, want ErrVoterTooFar", err)
	}

	// The report's own account cannot confirm it.
	if _, err := castContent(w, VisitorAccountID, id, service.VoteConfirm, baseLat, baseLon); !errors.Is(err, service.ErrCannotVoteOwnReport) {
		t.Fatalf("own confirm: err=%v, want ErrCannotVoteOwnReport", err)
	}

	// A neighbour within range is accepted and the response carries the real trust object.
	res, err := castContent(w, neighbour, id, service.VoteConfirm, northKm(baseLat, 0.3), baseLon)
	if err != nil {
		t.Fatalf("confirm from 300 m: %v", err)
	}
	if res.Trust.ConfirmedPlaces != 1 {
		t.Fatalf("ConfirmedPlaces = %d, want 1", res.Trust.ConfirmedPlaces)
	}

	// Unknown report: not found, through the pgx.ErrNoRows lookalike.
	if _, err := castContent(w, neighbour, id+99, service.VoteConfirm, baseLat, baseLon); !errors.Is(err, service.ErrReportNotFound) {
		t.Fatalf("unknown report: err=%v, want ErrReportNotFound", err)
	}

	// Expired: post a medium report 9 hours back on the world clock (its life is
	// 8 hours), then return to now.
	w.SetClockOffset(-9 * time.Hour)
	oldID := submitFlood(t, w, VisitorAccountID, baseLat, baseLon)
	w.SetClockOffset(0)
	if _, err := castContent(w, neighbour, oldID, service.VoteConfirm, baseLat, baseLon); !errors.Is(err, service.ErrReportExpired) {
		t.Fatalf("confirm on an expired report: err=%v, want ErrReportExpired", err)
	}
}

func TestResolveAndReopenByReporter(t *testing.T) {
	w := NewWorld("abcdefghijklmnopqrstuv", time.Now)
	id := submitFlood(t, w, VisitorAccountID, baseLat, baseLon)
	ctx := context.Background()

	// The reporter's own resolve needs no position (the reporter exemption).
	res, err := w.Votes.CastVote(ctx, service.CastVoteInput{
		ReportID: id, AccountID: VisitorAccountID, Kind: service.VoteKindResolution, Value: service.VoteResolve, LocationMissing: true,
	})
	if err != nil {
		t.Fatalf("reporter resolve: %v", err)
	}
	if res.Visibility != service.VisibilityRetracted {
		t.Fatalf("after the reporter resolved, visibility = %v, want retracted", res.Visibility)
	}
	res, err = w.Votes.CastVote(ctx, service.CastVoteInput{
		ReportID: id, AccountID: VisitorAccountID, Kind: service.VoteKindResolution, Value: service.VoteReopen, LocationMissing: true,
	})
	if err != nil {
		t.Fatalf("reporter reopen: %v", err)
	}
	if res.Visibility == service.VisibilityRetracted {
		t.Fatalf("a reopened report is still retracted")
	}
}

func TestWorldClockOffset(t *testing.T) {
	wall := time.Now().UTC()
	w := NewWorld("abcdefghijklmnopqrstuv", func() time.Time { return wall })
	w.SetClockOffset(-20 * time.Minute)
	if got := w.Now(); !got.Equal(wall.Add(-20 * time.Minute)) {
		t.Fatalf("Now() = %v, want wall minus 20 minutes", got)
	}

	id := submitFlood(t, w, VisitorAccountID, baseLat, baseLon)
	views, err := w.Reports.Nearby(context.Background(), service.NearbyQuery{
		Latitude: baseLat, Longitude: baseLon, RadiusKm: 5, ViewerAccountID: VisitorAccountID, All: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].ID != id {
		t.Fatalf("views = %+v", views)
	}
	created := views[0].CreatedAt
	// SubmitChecked stamps from the real wall clock (time.Now); the world moves
	// that stamp onto its own clock. Allow a few seconds of test run time.
	if d := created.Sub(w.Now()); d > 5*time.Second || d < -5*time.Second {
		t.Fatalf("created_at %v is not on the world clock %v (diff %v)", created, w.Now(), d)
	}
	if life := views[0].ExpiresAt.Sub(created); life != service.ExpiryDuration(service.SeverityMedium) {
		t.Fatalf("lifetime = %v, want %v", life, service.ExpiryDuration(service.SeverityMedium))
	}

	// A vote is stamped from the same clock.
	if _, err := castContent(w, NeighbourAccountBase, id, service.VoteConfirm, northKm(baseLat, 0.3), baseLon); err != nil {
		t.Fatal(err)
	}
	votes, err := w.q.CurrentVotesForReports(context.Background(), []int64{id})
	if err != nil || len(votes) != 1 {
		t.Fatalf("votes=%v err=%v", votes, err)
	}
	if !votes[0].CreatedAt.Equal(w.Now().Truncate(time.Microsecond)) {
		t.Fatalf("vote stamped %v, world clock %v", votes[0].CreatedAt, w.Now())
	}
}

func TestReplayThroughRealService(t *testing.T) {
	w := NewWorld("abcdefghijklmnopqrstuv", time.Now)
	in := service.SubmitInput{
		Category: service.CategoryFlood, Severity: service.SeverityLow, Description: "A retried post of the same thing",
		Latitude: baseLat, Longitude: baseLon, RequestID: "0123456789abcdef0123",
	}
	ctx := context.Background()
	first, err := w.Reports.SubmitChecked(ctx, w.SessionFor(VisitorAccountID), VisitorAccountID, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := w.Reports.SubmitChecked(ctx, w.SessionFor(VisitorAccountID), VisitorAccountID, in)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Report.ID != first.Report.ID {
		t.Fatalf("second send: replayed=%v id=%d, first id=%d", second.Replayed, second.Report.ID, first.Report.ID)
	}
	if got := w.q.reportCount(); got != 1 {
		t.Fatalf("a replay stored a second report: %d", got)
	}
}

func TestWorldsAreIndependent(t *testing.T) {
	a := NewWorld("aaaaaaaaaaaaaaaaaaaaaa", time.Now)
	b := NewWorld("bbbbbbbbbbbbbbbbbbbbbb", time.Now)
	submitFlood(t, a, VisitorAccountID, baseLat, baseLon)
	views, err := b.Reports.Nearby(context.Background(), service.NearbyQuery{Latitude: baseLat, Longitude: baseLon, RadiusKm: 5, ViewerAccountID: VisitorAccountID, All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 0 {
		t.Fatalf("a report posted in one world appeared in another: %+v", views)
	}
}

func TestSyntheticAccountIdsAreFarFromRealOnes(t *testing.T) {
	for name, id := range map[string]int64{"visitor": VisitorAccountID, "neighbour": NeighbourAccountBase, "seed reporter": SeedReporterAccountBase, "seed voter": SeedVoterAccountBase} {
		if id < 1_000_000_000 {
			t.Fatalf("%s account id %d is within reach of a real account id", name, id)
		}
	}
	if VisitorSessionPrefix != "demo-" {
		t.Fatalf("session prefix = %q", VisitorSessionPrefix)
	}
}

func TestPracticeVotesUseVisitorAccount(t *testing.T) {
	ctx := context.Background()
	w := NewWorld("abcdefghijklmnopqrstuv", time.Now)

	// The visitor's practice report. A confirm on it is refused as a self vote by
	// the real service, whatever account the caller names.
	res, err := w.PostPractice(ctx, practiceInput())
	if err != nil {
		t.Fatalf("PostPractice: %v", err)
	}
	own := res.Report.ID
	_, err = w.Vote(ctx, service.CastVoteInput{
		ReportID: own, AccountID: NeighbourAccountBase + 1, Kind: service.VoteKindContent, Value: service.VoteConfirm,
		Latitude: PretendLat, Longitude: PretendLon,
	})
	if !errors.Is(err, service.ErrCannotVoteOwnReport) {
		t.Fatalf("confirming the visitor's own practice report: err=%v, want ErrCannotVoteOwnReport", err)
	}
	if got := w.Activity().Posted; len(got) != 1 || got[0] != own {
		t.Fatalf("Activity().Posted = %v, want [%d]", got, own)
	}

	// A report posted by someone else: the vote is recorded under the visitor,
	// never under the account the caller named.
	other := submitFlood(t, w, SeedReporterAccountBase, PretendLat, PretendLon)
	if _, err := w.Vote(ctx, service.CastVoteInput{
		ReportID: other, AccountID: 424242, Kind: service.VoteKindContent, Value: service.VoteConfirm,
		Latitude: PretendLat, Longitude: PretendLon,
	}); err != nil {
		t.Fatalf("Vote: %v", err)
	}
	rows, err := w.q.CurrentVotesForReports(ctx, []int64{other})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if rows[0].AccountID != VisitorAccountID {
		t.Fatalf("the vote was recorded for account %d, want the visitor %d", rows[0].AccountID, VisitorAccountID)
	}
	if got := w.Activity().Confirmed; len(got) != 1 || got[0] != other {
		t.Fatalf("Activity().Confirmed = %v, want [%d]", got, other)
	}

	// Changing the mind moves the report from Confirmed to Disputed (one row per
	// account, the newer wins, as in SQL).
	if _, err := w.Vote(ctx, service.CastVoteInput{
		ReportID: other, Kind: service.VoteKindContent, Value: service.VoteDispute,
		Latitude: PretendLat, Longitude: PretendLon,
	}); err != nil {
		t.Fatalf("Vote dispute: %v", err)
	}
	act := w.Activity()
	if len(act.Confirmed) != 0 || len(act.Disputed) != 1 || act.Disputed[0] != other {
		t.Fatalf("Activity = %+v, want the report only under Disputed", act)
	}

	// The returned slices are copies.
	act.Disputed[0] = -1
	if w.Activity().Disputed[0] != other {
		t.Fatal("Activity exposes the world's own slice")
	}
}

func TestFarVoteSaysTooFar(t *testing.T) {
	ctx := context.Background()
	w := seededWorld(t)
	var far int64
	for _, r := range w.q.reports {
		if trueKm(PretendLat, PretendLon, r.p.Latitude, r.p.Longitude) >= 1.2 {
			far = r.id
			break
		}
	}
	if far == 0 {
		t.Fatal("the seed holds no report beyond 1 km")
	}
	_, err := w.Vote(ctx, service.CastVoteInput{
		ReportID: far, Kind: service.VoteKindContent, Value: service.VoteConfirm,
		Latitude: PretendLat, Longitude: PretendLon,
	})
	if !errors.Is(err, service.ErrVoterTooFar) {
		t.Fatalf("vote from the pretend position on a far report: err=%v, want ErrVoterTooFar", err)
	}
	const words = "Too far to vote. You need to be within 1 km of this report."
	if got := service.TooFarMessage(); got != words {
		t.Fatalf("TooFarMessage = %q, want %q", got, words)
	}
	if act := w.Activity(); len(act.Confirmed) != 0 || len(act.Disputed) != 0 {
		t.Fatalf("a refused vote was recorded: %+v", act)
	}
}

func TestNearSeedReportCanBeConfirmedByTheVisitor(t *testing.T) {
	ctx := context.Background()
	w := seededWorld(t)
	var near int64
	for _, r := range w.q.reports {
		if trueKm(PretendLat, PretendLon, r.p.Latitude, r.p.Longitude) <= 0.85 && r.p.ExpiresAt.After(w.Now()) {
			near = r.id
			break
		}
	}
	res, err := w.Vote(ctx, service.CastVoteInput{
		ReportID: near, Kind: service.VoteKindContent, Value: service.VoteConfirm,
		Latitude: PretendLat, Longitude: PretendLon,
	})
	if err != nil {
		t.Fatalf("visitor confirm on a near seed report: %v", err)
	}
	if res.Trust.ConfirmedPlaces < 1 {
		t.Fatalf("ConfirmedPlaces = %d after the visitor confirmed", res.Trust.ConfirmedPlaces)
	}
}
