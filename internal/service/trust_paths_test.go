// trust_paths_test.go is an external test package (service_test) because it
// reuses fakeFeedQuerier and the feed helpers from feed_test.go and
// fakeAuthQuerier and newFakes from auth_test.go, all of which live in
// service_test. No later plan edits this file. The fixed times are in 2030 on
// purpose: the wall clock would read every one of these reports as
// unexpired, so an expiry effect can only come from an injected clock.

package service_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

var (
	p05PathCreated = time.Date(2030, 1, 1, 11, 0, 0, 0, time.UTC)
	p05PathExpires = p05PathCreated.Add(8 * time.Hour)
	p05PathNow     = p05PathCreated.Add(time.Hour)
	p05PathLate    = p05PathExpires.Add(30 * time.Minute)
)

const (
	p05PathReport   int64 = 41
	p05PathReporter int64 = 7
	p05PathLat            = 12.9716
	p05PathLon            = 77.5946
)

// p05PathVote builds a vote row on the test report, cast 20 minutes before
// p05PathNow.
func p05PathVote(reportID, accountID int64, kind service.VoteKind, value service.VoteValue, cell string) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    reportID,
		AccountID:   accountID,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
		CreatedAt:   p05PathNow.Add(-20 * time.Minute),
	}
}

// p05VotingFake is a service.VotingQuerier. It embeds *fakeFeedQuerier on
// purpose, which supplies CurrentVotesForReports. When plan 03-12 adds a
// generated method to VotingQuerier and to fakeFeedQuerier, this fake gains
// it by promotion and needs no edit.
type p05VotingFake struct {
	*fakeFeedQuerier
	voteContext sqlcgen.ReportVoteContextRow
}

func (f *p05VotingFake) InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error {
	return nil
}

func (f *p05VotingFake) ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error) {
	return f.voteContext, nil
}

// p05PathResult is what one run of the three read paths returned.
type p05PathResult struct {
	nearby      service.ReportView
	activity    service.ActivityReport
	cast        service.CastVoteResult
	castErr     error
	nearbyCount int
}

// p05RunPaths runs Nearby, ActivityForAccount and CastVote over one row set,
// one clock and one viewer.
func p05RunPaths(t *testing.T, rows []sqlcgen.CurrentVotesForReportsRow, now time.Time, viewer int64) p05PathResult {
	t.Helper()
	clock := func() time.Time { return now }
	ctx := context.Background()
	var res p05PathResult

	nr := nearbyRow(p05PathReport, service.SeverityLow, service.CategoryFlood, 0.2)
	nr.CreatedAt = p05PathCreated
	nr.ExpiresAt = p05PathExpires
	feed := &fakeFeedQuerier{
		nearbyRows:   []sqlcgen.NearbyReportsRow{nr},
		voteRows:     rows,
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(p05PathReport, ptrID(p05PathReporter))},
	}
	views, err := service.NewReportService(feed, service.WithReportClock(clock)).Nearby(ctx, service.NearbyQuery{
		Latitude: p05PathLat, Longitude: p05PathLon, RadiusKm: 5, ViewerAccountID: viewer,
	})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	res.nearbyCount = len(views)
	if len(views) == 1 {
		res.nearby = views[0]
	}

	q, m, _ := newFakes()
	q.reportsRows = []sqlcgen.ReportsByAccountRow{{
		ID: p05PathReport, Category: string(service.CategoryFlood), Severity: string(service.SeverityLow),
		Description: "p05 path report", Latitude: p05PathLat, Longitude: p05PathLon,
		CreatedAt: p05PathCreated, ExpiresAt: p05PathExpires,
	}}
	q.votesRows = rows
	auth := service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(clock))
	acts, err := auth.ActivityForAccount(ctx, p05PathReporter)
	if err != nil {
		t.Fatalf("ActivityForAccount: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("ActivityForAccount returned %d rows, want 1", len(acts))
	}
	res.activity = acts[0]

	vf := &p05VotingFake{
		fakeFeedQuerier: &fakeFeedQuerier{voteRows: rows},
		voteContext: sqlcgen.ReportVoteContextRow{
			Severity: string(service.SeverityLow), Category: string(service.CategoryFlood),
			ExpiresAt: p05PathExpires, ReporterAccountID: ptrID(p05PathReporter),
			Latitude: p05PathLat, Longitude: p05PathLon, CreatedAt: p05PathCreated,
		},
	}
	res.cast, res.castErr = service.NewVotingService(vf, service.WithVotingClock(clock)).CastVote(ctx, service.CastVoteInput{
		ReportID: p05PathReport, AccountID: 12, Kind: service.VoteKindContent, Value: service.VoteConfirm,
		Latitude: p05PathLat, Longitude: p05PathLon,
	})
	return res
}

// TestTrustAttachedOnEveryReadPath proves the three read paths carry the
// same trust object for the same data and clock, for every viewer (D-21).
func TestTrustAttachedOnEveryReadPath(t *testing.T) {
	meta := service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood}
	buildWant := func(rows []sqlcgen.CurrentVotesForReportsRow, vis service.Visibility, now time.Time) service.Trust {
		// No history is configured in these fixtures, so every path carries the
		// reporter's New reporter signal (plan 03-12).
		reporter := service.ReporterSignals([]int64{p05PathReporter}, nil, rows, now)[p05PathReporter]
		return service.BuildTrust(service.TrustInput{
			Rows: rows, ReportID: p05PathReport, ReporterAccountID: ptrID(p05PathReporter), Meta: meta,
			Visibility: vis, CreatedAt: p05PathCreated, ExpiresAt: p05PathExpires, Now: now,
			Reporter: &reporter,
		})
	}
	liveRows := []sqlcgen.CurrentVotesForReportsRow{
		p05PathVote(p05PathReport, 11, service.VoteKindContent, service.VoteConfirm, "cellA"),
		p05PathVote(p05PathReport, 12, service.VoteKindContent, service.VoteConfirm, "cellB"),
		p05PathVote(p05PathReport, 13, service.VoteKindContent, service.VoteConfirm, "cellA"),
		p05PathVote(p05PathReport, 14, service.VoteKindContent, service.VoteDispute, "cellC"),
		p05PathVote(p05PathReport, p05PathReporter, service.VoteKindContent, service.VoteConfirm, "cellD"),
		p05PathVote(p05PathReport, 15, service.VoteKindResolution, service.VoteResolve, "cellE"),
		p05PathVote(99, 11, service.VoteKindContent, service.VoteConfirm, "cellF"),
	}

	t.Run("the same rows give the same trust on all three paths", func(t *testing.T) {
		res := p05RunPaths(t, liveRows, p05PathNow, 0)
		if res.castErr != nil {
			t.Fatalf("CastVote: %v", res.castErr)
		}
		if res.nearbyCount != 1 {
			t.Fatalf("Nearby returned %d views, want 1", res.nearbyCount)
		}
		want := buildWant(liveRows, service.VisibilityLive, p05PathNow)
		paths := map[string]service.Trust{
			"nearby": res.nearby.Trust, "activity": res.activity.Trust, "cast": res.cast.Trust,
		}
		for name, got := range paths {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s trust differs from BuildTrust:\n%+v\n%+v", name, got, want)
			}
			if got.ConfirmedPlaces != 2 || got.DisputedPlaces != 1 || got.ConfirmedText != "2" || got.DisputedText != "1" || got.LeadSentence != "" {
				t.Errorf("%s trust = %+v", name, got)
			}
			if w := "2 people in 2 different places confirmed this. Counts separate places, not votes."; got.ConfirmedSentence != w {
				t.Errorf("%s ConfirmedSentence = %q", name, got.ConfirmedSentence)
			}
			if w := "1 person in 1 place disputed this. Counts separate places, not votes."; got.DisputedSentence != w {
				t.Errorf("%s DisputedSentence = %q", name, got.DisputedSentence)
			}
		}
		if !reflect.DeepEqual(res.nearby.Trust, res.activity.Trust) || !reflect.DeepEqual(res.activity.Trust, res.cast.Trust) {
			t.Error("the three trust values are not pairwise equal")
		}
	})

	t.Run("provisional on all three paths", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			p05PathVote(p05PathReport, 11, service.VoteKindContent, service.VoteConfirm, "cellA"),
			p05PathVote(p05PathReport, p05PathReporter, service.VoteKindContent, service.VoteConfirm, "cellD"),
		}
		res := p05RunPaths(t, rows, p05PathNow, 0)
		if res.castErr != nil {
			t.Fatalf("CastVote: %v", res.castErr)
		}
		if res.nearbyCount != 1 {
			t.Fatalf("Nearby returned %d views, want 1", res.nearbyCount)
		}
		lead := "Needs 2 confirmations before this counts as confirmed."
		for name, got := range map[string]service.Trust{
			"nearby": res.nearby.Trust, "activity": res.activity.Trust, "cast": res.cast.Trust,
		} {
			if got.ConfirmedPlaces != 1 || got.LeadSentence != lead {
				t.Errorf("%s trust = %+v", name, got)
			}
		}
		if !reflect.DeepEqual(res.nearby.Trust, res.activity.Trust) || !reflect.DeepEqual(res.activity.Trust, res.cast.Trust) {
			t.Error("the three trust values are not pairwise equal")
		}
	})

	t.Run("each path reads its injected clock", func(t *testing.T) {
		res := p05RunPaths(t, liveRows, p05PathLate, 0)
		expired := "This report has expired. It can no longer be confirmed."
		if res.nearbyCount != 1 {
			t.Fatalf("Nearby returned %d views, want 1", res.nearbyCount)
		}
		if res.nearby.Trust.LeadSentence != expired {
			t.Errorf("nearby LeadSentence = %q", res.nearby.Trust.LeadSentence)
		}
		if res.activity.Trust.LeadSentence != expired {
			t.Errorf("activity LeadSentence = %q", res.activity.Trust.LeadSentence)
		}
		if !errors.Is(res.castErr, service.ErrReportExpired) {
			t.Errorf("CastVote err = %v, want ErrReportExpired", res.castErr)
		}
	})

	t.Run("a retracted report on the Activity page has no lead", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			p05PathVote(p05PathReport, p05PathReporter, service.VoteKindResolution, service.VoteResolve, "cellD"),
			p05PathVote(p05PathReport, 11, service.VoteKindContent, service.VoteConfirm, "cellA"),
		}
		for name, now := range map[string]time.Time{"now": p05PathNow, "late": p05PathLate} {
			res := p05RunPaths(t, rows, now, 0)
			if res.activity.Visibility != service.VisibilityRetracted {
				t.Errorf("%s: Visibility = %q, want retracted", name, res.activity.Visibility)
			}
			if res.activity.Trust.LeadSentence != "" || res.activity.Trust.ConfirmedPlaces != 1 {
				t.Errorf("%s: Trust = %+v", name, res.activity.Trust)
			}
		}
	})

	t.Run("the trust object is the same for every viewer", func(t *testing.T) {
		var first service.Trust
		for i, viewer := range []int64{0, 11, p05PathReporter} {
			res := p05RunPaths(t, liveRows, p05PathNow, viewer)
			if res.nearbyCount != 1 {
				t.Fatalf("viewer %d: Nearby returned %d views, want 1", viewer, res.nearbyCount)
			}
			if i == 0 {
				first = res.nearby.Trust
			} else if !reflect.DeepEqual(first, res.nearby.Trust) {
				t.Errorf("viewer %d: trust differs from viewer 0", viewer)
			}
			if own := viewer == p05PathReporter; res.nearby.IsOwnReport != own {
				t.Errorf("viewer %d: IsOwnReport = %v, want %v", viewer, res.nearby.IsOwnReport, own)
			}
		}
	})
}
