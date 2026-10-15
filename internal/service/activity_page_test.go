package service_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The tests in this file belong to quick task 261013-wm5. They prove that the
// Activity page reads the account's posts and its voted reports in one pass,
// with one history read and one vote read, and that a voted report carries its
// own reporter's signal and the same trust the feed builds for it.

// wm5VotedRow is a VotedReportsByAccount row for a low severity flood report
// created an hour before p12Now, voted on at votedAt.
func wm5VotedRow(id int64, value service.VoteValue, votedAt time.Time) sqlcgen.VotedReportsByAccountRow {
	return sqlcgen.VotedReportsByAccountRow{
		ID:          id,
		Category:    string(service.CategoryFlood),
		Severity:    string(service.SeverityLow),
		Description: "a wm5 voted report",
		Latitude:    12.9716,
		Longitude:   77.5946,
		Geohash:     "tdr1qgzp",
		CreatedAt:   p12Now.Add(-time.Hour),
		ExpiresAt:   p12Now.Add(7 * time.Hour),
		VoteValue:   string(value),
		VotedAt:     votedAt,
	}
}

func wm5Service(q *fakeAuthQuerier, m *fakeMailer) *service.AuthService {
	return service.NewAuthService(q, m, "https://pinalert.example", service.WithClock(p12Clock))
}

func TestActivityPageReadsPostsAndVotesInOnePass(t *testing.T) {
	ctx := context.Background()
	const viewer int64 = 42
	const reporterA, reporterB int64 = 7, 8

	q, m, calls := newFakes()
	q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1)}
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{
		wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-10*time.Minute)),
		wm5VotedRow(11, service.VoteDispute, p12Now.Add(-20*time.Minute)),
	}
	q.reporterRows = []sqlcgen.ReporterAccountsForReportsRow{
		reporterRow(10, ptrID(reporterA)),
		reporterRow(11, ptrID(reporterB)),
	}
	historyA, votesA := p12Corroborated(reporterA, 100, 5)
	q.historyRows = historyA
	q.votesRows = votesA

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, viewer)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}

	wantCalls := []string{
		"ReportsByAccount", "VotedReportsByAccount", "ReporterAccountsForReports",
		"ReporterHistoryReports", "CurrentVotesForReports",
	}
	if !slices.Equal(*calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", *calls, wantCalls)
	}
	if len(q.votedArgs) != 1 || q.votedArgs[0].AccountID != viewer || q.votedArgs[0].Kind != "content" {
		t.Errorf("votedArgs = %+v, want one call for account %d and kind content", q.votedArgs, viewer)
	}
	if len(q.reporterArgs) != 1 || !slices.Equal(q.reporterArgs[0], []int64{10, 11}) {
		t.Errorf("reporterArgs = %v, want [[10 11]]", q.reporterArgs)
	}
	if len(q.historyArgs) != 1 || !slices.Equal(q.historyArgs[0].AccountIds, []int64{viewer, reporterA, reporterB}) {
		t.Errorf("historyArgs = %+v, want one call over viewer then reporters", q.historyArgs)
	}
	if q.votesCallCount != 1 {
		t.Errorf("votesCallCount = %d, want 1", q.votesCallCount)
	}
	if want := []int64{1, 10, 11, 100, 101, 102, 103, 104}; !slices.Equal(q.votesArg, want) {
		t.Errorf("votesArg = %v, want post ids, voted ids, then history ids %v", q.votesArg, want)
	}
	if len(page.Posts) != 1 || len(page.Votes) != 2 {
		t.Fatalf("page has %d posts and %d votes, want 1 and 2", len(page.Posts), len(page.Votes))
	}
}

func TestActivityPageWithoutPostsReadsOnlyTheReportersHistory(t *testing.T) {
	ctx := context.Background()
	q, m, _ := newFakes()
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-time.Minute))}
	q.reporterRows = []sqlcgen.ReporterAccountsForReportsRow{reporterRow(10, ptrID(7))}

	if _, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42); err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	if len(q.historyArgs) != 1 || !slices.Equal(q.historyArgs[0].AccountIds, []int64{7}) {
		t.Errorf("historyArgs = %+v, want one call over the reporter only", q.historyArgs)
	}
}

func TestActivityPageVotedReportCarriesItsReportersSignalNotTheViewers(t *testing.T) {
	ctx := context.Background()
	const viewer int64 = 42
	const reporter int64 = 7

	q, m, _ := newFakes()
	q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1)}
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-time.Minute))}
	q.reporterRows = []sqlcgen.ReporterAccountsForReportsRow{reporterRow(10, ptrID(reporter))}
	// The reporter has a reliable record. The viewer has an unconfirmed one.
	history, votes := p12Corroborated(reporter, 100, 5)
	q.historyRows = append(history, p12Unconfirmed(viewer, 200, 5)...)
	q.votesRows = votes

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, viewer)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	signals := service.ReporterSignals([]int64{viewer, reporter}, q.historyRows, votes, p12Now)
	wantReporter := signals[reporter]
	wantViewer := signals[viewer]
	if wantReporter.Tag != service.ReporterReliable {
		t.Fatalf("fixture: reporter tag = %q, want reliable", wantReporter.Tag)
	}
	if reflect.DeepEqual(wantReporter, wantViewer) {
		t.Fatal("fixture: the two signals must differ for this test to mean anything")
	}
	if got := page.Votes[0].Trust.Reporter; !reflect.DeepEqual(got, wantReporter) {
		t.Errorf("voted report reporter signal = %+v, want the reporter's %+v", got, wantReporter)
	}
	if got := page.Posts[0].Trust.Reporter; !reflect.DeepEqual(got, wantViewer) {
		t.Errorf("post reporter signal = %+v, want the viewer's %+v", got, wantViewer)
	}
}

func TestActivityPageReportersOwnResolveRetractsTheVotedReport(t *testing.T) {
	ctx := context.Background()
	const reporter int64 = 7

	q, m, _ := newFakes()
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-time.Minute))}
	q.reporterRows = []sqlcgen.ReporterAccountsForReportsRow{reporterRow(10, ptrID(reporter))}
	q.votesRows = []sqlcgen.CurrentVotesForReportsRow{
		p12Vote(10, reporter, service.VoteKindResolution, service.VoteResolve, "wm5cell"),
	}

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	if got := page.Votes[0].Visibility; got != service.VisibilityRetracted {
		t.Errorf("voted report visibility = %q, want retracted after the reporter's own resolve", got)
	}
}

// TestActivityPageVotedTrustEqualsTheFeedTrust is D-21 for the new list: the
// trust a voted card carries is DeepEqual to the trust Nearby builds for the
// same report from the same rows and clock.
func TestActivityPageVotedTrustEqualsTheFeedTrust(t *testing.T) {
	ctx := context.Background()
	const reporter int64 = 7

	history, histVotes := p12Corroborated(reporter, 100, 5)
	reportVotes := append(p12Places(10, service.VoteConfirm, 2, 21), p12Vote(10, 42, service.VoteKindContent, service.VoteConfirm, "viewercell"))
	allVotes := append(histVotes, reportVotes...)

	q, m, _ := newFakes()
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-time.Minute))}
	q.reporterRows = []sqlcgen.ReporterAccountsForReportsRow{reporterRow(10, ptrID(reporter))}
	q.historyRows = history
	q.votesRows = allVotes

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}

	fake := &fakeFeedQuerier{
		nearbyRows:   []sqlcgen.NearbyReportsRow{p12Row(10, service.SeverityLow, service.CategoryFlood, 1)},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{reporterRow(10, ptrID(reporter))},
		historyRows:  history,
		voteRows:     allVotes,
	}
	views, err := service.NewReportService(fake, service.WithReportClock(p12Clock)).Nearby(ctx, service.NearbyQuery{
		Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5, ViewerAccountID: 42,
	})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	v, ok := findView(views, 10)
	if !ok {
		t.Fatalf("report 10 not listed by Nearby: %v", viewIDs(views))
	}
	if !reflect.DeepEqual(page.Votes[0].Trust, v.Trust) {
		t.Errorf("voted trust differs from the feed trust\nactivity: %+v\nfeed:     %+v", page.Votes[0].Trust, v.Trust)
	}
}

func TestActivityPageDropsOwnPostsAndInvalidVoteValuesFromTheVotes(t *testing.T) {
	ctx := context.Background()
	q, m, _ := newFakes()
	q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1)}
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{
		wm5VotedRow(1, service.VoteConfirm, p12Now.Add(-time.Minute)),  // the account's own post
		wm5VotedRow(10, service.VoteResolve, p12Now.Add(-time.Minute)), // not a content value
		wm5VotedRow(11, service.VoteConfirm, p12Now.Add(-time.Minute)),
	}

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	if len(page.Votes) != 1 || page.Votes[0].Row.ID != 11 {
		t.Fatalf("votes = %+v, want only report 11", page.Votes)
	}
	if len(page.Posts) != 1 || page.Posts[0].Row.ID != 1 {
		t.Fatalf("posts = %+v, want only report 1", page.Posts)
	}
	if want := []string{"11"}; len(q.reporterArgs) != 1 || len(q.reporterArgs[0]) != 1 || q.reporterArgs[0][0] != 11 {
		t.Errorf("reporterArgs = %v, want only the remaining voted id %v", q.reporterArgs, want)
	}
}

func TestActivityPageValidatesCategoryAndSeverityOnVotedReports(t *testing.T) {
	ctx := context.Background()
	q, m, _ := newFakes()
	row := wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-time.Minute))
	row.Category = "not-a-category"
	row.Severity = "not-a-severity"
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{row}

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	got := page.Votes[0]
	if got.Category != service.CategoryOther || got.Severity != service.SeverityLow {
		t.Errorf("category %q severity %q, want other and low", got.Category, got.Severity)
	}
	if got.Row.Category != "not-a-category" {
		t.Errorf("raw row category = %q, want the raw value kept", got.Row.Category)
	}
}

func TestActivityPageEmptyPageMakesOnlyTheTwoFirstReads(t *testing.T) {
	ctx := context.Background()
	q, m, calls := newFakes()

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	if want := []string{"ReportsByAccount", "VotedReportsByAccount"}; !slices.Equal(*calls, want) {
		t.Errorf("calls = %v, want %v", *calls, want)
	}
	if page.Posts == nil || page.Votes == nil || len(page.Posts) != 0 || len(page.Votes) != 0 {
		t.Errorf("page = %+v, want two empty non nil lists", page)
	}
}

func TestActivityPageSortsVotesNewestVoteFirstTiesByReportID(t *testing.T) {
	ctx := context.Background()
	q, m, _ := newFakes()
	tie := p12Now.Add(-30 * time.Minute)
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{
		wm5VotedRow(10, service.VoteConfirm, p12Now.Add(-2*time.Hour)),
		wm5VotedRow(11, service.VoteConfirm, tie),
		wm5VotedRow(12, service.VoteDispute, p12Now.Add(-time.Minute)),
		wm5VotedRow(13, service.VoteConfirm, tie),
	}

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	var got []int64
	for _, v := range page.Votes {
		got = append(got, v.Row.ID)
	}
	if want := []int64{12, 13, 11, 10}; !slices.Equal(got, want) {
		t.Errorf("vote order = %v, want %v", got, want)
	}
	if page.Votes[0].Vote != service.VoteDispute {
		t.Errorf("first vote value = %q, want dispute", page.Votes[0].Vote)
	}
}

func TestActivityPageWrapsFailedReads(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("read failed")

	t.Run("voted reports read", func(t *testing.T) {
		q, m, calls := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{p12ActivityRow(1)}
		q.votedErr = boom
		_, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "reading voted reports for account 42") {
			t.Fatalf("err = %v, want a wrapped voted reports error", err)
		}
		if q.votesCallCount != 0 {
			t.Errorf("a vote read happened after the failure: %v", *calls)
		}
	})

	t.Run("reporter accounts read", func(t *testing.T) {
		q, m, calls := newFakes()
		q.votedRows = []sqlcgen.VotedReportsByAccountRow{wm5VotedRow(10, service.VoteConfirm, p12Now)}
		q.reporterErr = boom
		_, err := wm5Service(q, m).ActivityPageForAccount(ctx, 42)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want the wrapped reporter error", err)
		}
		if q.votesCallCount != 0 || len(q.historyArgs) != 0 {
			t.Errorf("history or vote read happened after the failure: %v", *calls)
		}
	})
}

// p31ActivityPost is an own post of the given category created an hour before
// p12Now, expiring at expiresAt.
func p31ActivityPost(id int64, category service.Category, expiresAt time.Time) sqlcgen.ReportsByAccountRow {
	row := p12ActivityRow(id)
	row.Category = string(category)
	row.Description = "a p31 own post"
	row.ExpiresAt = expiresAt
	return row
}

// p31VotedRow is a voted card of the given category with the given expiry.
func p31VotedRow(id int64, category service.Category, expiresAt time.Time) sqlcgen.VotedReportsByAccountRow {
	row := wm5VotedRow(id, service.VoteConfirm, p12Now.Add(-time.Minute))
	row.Category = string(category)
	row.Description = "a p31 secret rescue description"
	row.ExpiresAt = expiresAt
	return row
}

// TestActivityUndoReadOnlyForOpenClosedPosts is D-28: the undo count is one
// batched read, made only when an own post is closed and not yet faded.
func TestActivityUndoReadOnlyForOpenClosedPosts(t *testing.T) {
	ctx := context.Background()
	const viewer int64 = 42
	live := p12Now.Add(7 * time.Hour)
	faded := p12Now.Add(-time.Minute)

	t.Run("no closed unexpired post makes no read", func(t *testing.T) {
		q, m, calls := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{
			p31ActivityPost(1, service.CategoryFlood, live),
			p31ActivityPost(2, service.CategoryFlood, faded),
		}
		// Post 2 is closed by its poster but already faded.
		q.votesRows = []sqlcgen.CurrentVotesForReportsRow{
			p12Vote(2, viewer, service.VoteKindResolution, service.VoteResolve, "p31cell"),
		}
		page, err := wm5Service(q, m).ActivityPageForAccount(ctx, viewer)
		if err != nil {
			t.Fatalf("ActivityPageForAccount: %v", err)
		}
		if len(q.undoArgs) != 0 || slices.Contains(*calls, "ResolutionUndoCounts") {
			t.Fatalf("undo reads = %d, calls %v, want none", len(q.undoArgs), *calls)
		}
		for _, p := range page.Posts {
			if p.UndosLeft != service.UndoLimit {
				t.Errorf("post %d UndosLeft = %d, want %d", p.Row.ID, p.UndosLeft, service.UndoLimit)
			}
		}
	})

	t.Run("two closed posts make exactly one read", func(t *testing.T) {
		q, m, _ := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{
			p31ActivityPost(1, service.CategoryFlood, live),
			p31ActivityPost(2, service.CategoryFlood, live),
			p31ActivityPost(3, service.CategoryFlood, live),
		}
		q.votesRows = []sqlcgen.CurrentVotesForReportsRow{
			p12Vote(1, viewer, service.VoteKindResolution, service.VoteResolve, "p31cell"),
			p12Vote(2, viewer, service.VoteKindResolution, service.VoteResolve, "p31cell"),
		}
		q.undoRows = []sqlcgen.ResolutionUndoCountsRow{
			{ReportID: 1, Unresolves: 1, Reopens: 1},
			{ReportID: 2, Unresolves: 1, Reopens: 0},
		}
		page, err := wm5Service(q, m).ActivityPageForAccount(ctx, viewer)
		if err != nil {
			t.Fatalf("ActivityPageForAccount: %v", err)
		}
		if len(q.undoArgs) != 1 {
			t.Fatalf("undo reads = %d, want exactly 1", len(q.undoArgs))
		}
		arg := q.undoArgs[0]
		if arg.AccountID != viewer || !slices.Equal(arg.ReportIds, []int64{1, 2}) {
			t.Errorf("undo arg = %+v, want account %d and ids [1 2]", arg, viewer)
		}
		got := map[int64]int{}
		for _, p := range page.Posts {
			got[p.Row.ID] = p.UndosLeft
		}
		want := map[int64]int{1: 0, 2: 1, 3: service.UndoLimit}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("UndosLeft = %v, want %v (unresolve plus reopen rows count for the poster)", got, want)
		}
	})

	t.Run("a failed read is wrapped", func(t *testing.T) {
		boom := errors.New("undo read failed")
		q, m, _ := newFakes()
		q.reportsRows = []sqlcgen.ReportsByAccountRow{p31ActivityPost(1, service.CategoryFlood, live)}
		q.votesRows = []sqlcgen.CurrentVotesForReportsRow{
			p12Vote(1, viewer, service.VoteKindResolution, service.VoteResolve, "p31cell"),
		}
		q.undoErr = boom
		_, err := wm5Service(q, m).ActivityPageForAccount(ctx, viewer)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want the wrapped undo read error", err)
		}
	})
}

// TestActivityVotedRescueCoarsened is D-27: a voted card of another person's
// finished Rescue report is blurred and loses its text, while a live Rescue
// card, a finished flood card and the account's own posts stay exact.
func TestActivityVotedRescueCoarsened(t *testing.T) {
	ctx := context.Background()
	const viewer, reporter int64 = 42, 7
	live := p12Now.Add(7 * time.Hour)
	faded := p12Now.Add(-time.Minute)

	q, m, _ := newFakes()
	q.reportsRows = []sqlcgen.ReportsByAccountRow{p31ActivityPost(1, service.CategoryRescueNeeded, live)}
	q.votedRows = []sqlcgen.VotedReportsByAccountRow{
		p31VotedRow(10, service.CategoryRescueNeeded, live),  // closed by its reporter
		p31VotedRow(11, service.CategoryRescueNeeded, faded), // expired
		p31VotedRow(12, service.CategoryRescueNeeded, live),  // still live
		p31VotedRow(13, service.CategoryFlood, live),         // closed flood
	}
	q.reporterRows = []sqlcgen.ReporterAccountsForReportsRow{
		reporterRow(10, ptrID(reporter)), reporterRow(11, ptrID(reporter)),
		reporterRow(12, ptrID(reporter)), reporterRow(13, ptrID(reporter)),
	}
	q.votesRows = []sqlcgen.CurrentVotesForReportsRow{
		p12Vote(1, viewer, service.VoteKindResolution, service.VoteResolve, "p31cell"),
		p12Vote(10, reporter, service.VoteKindResolution, service.VoteResolve, "p31cell"),
		p12Vote(13, reporter, service.VoteKindResolution, service.VoteResolve, "p31cell"),
	}

	page, err := wm5Service(q, m).ActivityPageForAccount(ctx, viewer)
	if err != nil {
		t.Fatalf("ActivityPageForAccount: %v", err)
	}
	byID := map[int64]service.VotedReport{}
	for _, v := range page.Votes {
		byID[v.Row.ID] = v
	}
	wantLat, wantLon, wantCell := service.CoarsePoint(12.9716, 77.5946)
	for _, id := range []int64{10, 11} {
		v := byID[id]
		if !v.Approximate {
			t.Errorf("card %d Approximate = false, want true", id)
		}
		if v.Row.Description != "" {
			t.Errorf("card %d kept its description %q", id, v.Row.Description)
		}
		if v.Row.Latitude != wantLat || v.Row.Longitude != wantLon || v.Row.Geohash != wantCell {
			t.Errorf("card %d point = %v,%v,%q want %v,%v,%q", id, v.Row.Latitude, v.Row.Longitude, v.Row.Geohash, wantLat, wantLon, wantCell)
		}
	}
	for _, id := range []int64{12, 13} {
		v := byID[id]
		if v.Approximate || v.Row.Description == "" || v.Row.Latitude != 12.9716 || v.Row.Geohash != "tdr1qgzp" {
			t.Errorf("card %d was coarsened but must stay exact: %+v", id, v)
		}
	}
	own := page.Posts[0]
	if own.Approximate || own.Row.Description != "a p31 own post" || own.Row.Latitude != 12.9716 {
		t.Errorf("own post was coarsened: %+v", own)
	}
}
