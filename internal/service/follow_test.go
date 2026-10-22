package service_test

import (
	"testing"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// TestViewerFollows pins the follow rule the feed carries (D-09, D-10): the
// reader follows a report when they posted it or hold a vote row of any kind
// and value on it, and for nobody else.
func TestViewerFollows(t *testing.T) {
	const report, other, viewer = int64(1), int64(2), int64(7)

	cases := []struct {
		name   string
		rows   []sqlcgen.CurrentVotesForReportsRow
		viewer int64
		isOwn  bool
		want   bool
	}{
		{name: "no rows and not own is false", viewer: viewer, want: false},
		{name: "viewer 0 is false even with a matching row", viewer: 0, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(report, 0, service.VoteKindContent, service.VoteConfirm, "c1")}, want: false},
		{name: "viewer 0 is false even when marked own", viewer: 0, isOwn: true, want: false},
		{name: "own report is true with no rows", viewer: viewer, isOwn: true, want: true},
		{name: "a confirm is true", viewer: viewer, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(report, viewer, service.VoteKindContent, service.VoteConfirm, "c1")}, want: true},
		{name: "a dispute is true", viewer: viewer, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(report, viewer, service.VoteKindContent, service.VoteDispute, "c1")}, want: true},
		{name: "a resolve is true", viewer: viewer, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(report, viewer, service.VoteKindResolution, service.VoteResolve, "c1")}, want: true},
		{name: "a reopen is true", viewer: viewer, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(report, viewer, service.VoteKindResolution, service.VoteReopen, "c1")}, want: true},
		{name: "an unresolve is true", viewer: viewer, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(report, viewer, service.VoteKindResolution, service.VoteUnresolve, "c1")}, want: true},
		{name: "someone else's vote is false", viewer: viewer, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(report, 99, service.VoteKindContent, service.VoteConfirm, "c1")}, want: false},
		{name: "the viewer's vote on another report is false", viewer: viewer, rows: []sqlcgen.CurrentVotesForReportsRow{voteRow(other, viewer, service.VoteKindContent, service.VoteConfirm, "c1")}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.ViewerFollows(tc.rows, report, tc.viewer, tc.isOwn); got != tc.want {
				t.Errorf("ViewerFollows = %v, want %v", got, tc.want)
			}
		})
	}
}
