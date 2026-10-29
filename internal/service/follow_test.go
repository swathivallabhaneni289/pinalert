package service_test

import (
	"testing"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// Amended 2026-10-29 by Phase 8 plan 10 (PAGE-02): ViewerFollows takes a
// commented argument, so the existing cases pass false through the zero value
// and two commented cases are added; every other assertion is unchanged.
//
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

		commented bool
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
		{name: "a comment alone is true", viewer: viewer, commented: true, want: true},
		{name: "viewer 0 is false even when commented", viewer: 0, commented: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.ViewerFollows(tc.rows, report, tc.viewer, tc.isOwn, tc.commented); got != tc.want {
				t.Errorf("ViewerFollows = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCommenterFollowsReportInFeed is the feed arm of PAGE-02: a viewer whose
// only link to a report is a clear comment follows it, and the same viewer with
// no comment, no vote and no post of their own does not.
func TestCommenterFollowsReportInFeed(t *testing.T) {
	const report, viewer = int64(1), int64(7)
	if !service.ViewerFollows(nil, report, viewer, false, true) {
		t.Error("a commenter with no vote and no post must follow the report")
	}
	if service.ViewerFollows(nil, report, viewer, false, false) {
		t.Error("a viewer with no comment, vote or post must not follow the report")
	}
	if service.ViewerFollows(nil, report, 0, false, true) {
		t.Error("no identified viewer follows nothing, a comment flag or not")
	}
}
