// timeline.go builds the history of one report, "What happened so far" (plan
// 08-01, PAGE-01, SC2, D-01), from the append only vote log. BuildTimeline is
// pure: it takes values and returns values, with no context, no querier and no
// I/O, in the style of history.go.
//
// The log is replayed one row at a time. After each row the standing row per
// (account, kind) is rebuilt, latest wins, which is the rule of the DISTINCT ON
// in CurrentVotesForReports. The standing rows go through BuildVoteTally and
// Resolve, the same two functions the trust block reads, and the step is
// compared with the one before it. No number is counted by hand here: every
// count in a sentence is the distinct spot count the tally gives at that step,
// and the reporter's own votes never count because the tally leaves them out.
//
// The timeline shows only upward steps and state changes, by decision (08
// RESEARCH section 3): the first time the confirming or disputing spot count
// reaches 1, 2, 3, 4, 5, 10, 25 and 50, the moments the report is shown to
// everyone, enters or leaves the Disputed view, is resolved or reopened, and
// fades. A take back never writes a downward line, and a milestone already
// reached is never written twice. A reader who sees "Confirmed from 4 spots" in
// the history and a lower number in the trust block understands that votes were
// changed; the trust block is the current truth.
//
// Nothing here names a voter, an account or a cell. The viewer's own rows give
// the lines marked Yours, and only for the viewer the caller names.

package service

import (
	"math"
	"sort"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// TimelineKind is the closed set of history line kinds.
type TimelineKind string

const (
	TimelinePosted    TimelineKind = "posted"
	TimelineConfirmed TimelineKind = "confirmed"
	TimelineDisputed  TimelineKind = "disputed"
	TimelineResolved  TimelineKind = "resolved"
	TimelineReopened  TimelineKind = "reopened"
	TimelineFaded     TimelineKind = "faded"
	TimelineYours     TimelineKind = "yours"
)

// The tones of a history line, the same words FactResponse uses.
const (
	TimelineToneGood    = "good"
	TimelineToneNeutral = "neutral"
	TimelineToneBad     = "bad"
)

// TimelineMilestones are the spot counts a confirming or disputing line is
// written for. A busy report therefore has about sixteen count lines at most.
var TimelineMilestones = []int{1, 2, 3, 4, 5, 10, 25, 50}

// TimelineLogCap bounds one vote log read (T-08-04).
const TimelineLogCap = 2000

// TimelineEvent is one line of the history. Text comes from timelinecopy.go
// only. At is the time of the vote row that caused the line, in UTC when the
// handler serialises it. Yours is true for a line about the viewer's own vote.
type TimelineEvent struct {
	Kind  TimelineKind
	Tone  string
	Text  string
	At    time.Time
	Yours bool
}

// TimelineInput is everything BuildTimeline needs. Log is the vote log of the
// report in id order; the function sorts a copy by id so equal inputs always
// give equal output. ViewerAccountID 0 means no viewer, so no Yours line.
type TimelineInput struct {
	ReportID          int64
	ReporterAccountID *int64
	Meta              ReportMeta
	CreatedAt         time.Time
	ExpiresAt         time.Time
	Now               time.Time
	Log               []sqlcgen.TimelineVoteLogRow
	ViewerAccountID   int64
}

// The fixed order of lines made by one vote row, so equal inputs give byte
// equal output.
const (
	timelineSubClosure = iota
	timelineSubConfirm
	timelineSubDispute
	timelineSubLeave
	timelineSubYours
)

type timelineLine struct {
	ev  TimelineEvent
	seq int64
	sub int
}

type timelineKey struct {
	account int64
	kind    string
}

// topMilestone returns the highest milestone that is above reached and at most
// count, or 0. One vote row moves a distinct spot count by one at most, so it
// crosses at most one milestone and the milestone equals the count.
func topMilestone(count, reached int) int {
	top := 0
	for _, m := range TimelineMilestones {
		if m > reached && m <= count {
			top = m
		}
	}
	return top
}

// BuildTimeline replays the vote log of one report into history lines, ordered
// by time, then by log id, then by a fixed order within one row.
func BuildTimeline(in TimelineInput) []TimelineEvent {
	log := make([]sqlcgen.TimelineVoteLogRow, len(in.Log))
	copy(log, in.Log)
	sort.SliceStable(log, func(i, j int) bool { return log[i].ID < log[j].ID })

	var lines []timelineLine
	add := func(seq int64, sub int, kind TimelineKind, tone, text string, at time.Time, yours bool) {
		lines = append(lines, timelineLine{
			ev:  TimelineEvent{Kind: kind, Tone: tone, Text: text, At: at, Yours: yours},
			seq: seq,
			sub: sub,
		})
	}

	add(math.MinInt64, 0, TimelinePosted, TimelineToneNeutral, timelinePostedText(in.Meta.Severity), in.CreatedAt, false)

	viewerIsReporter := in.ReporterAccountID != nil && in.ViewerAccountID != 0 && *in.ReporterAccountID == in.ViewerAccountID

	// The standing rows keep first seen order, so a replay is deterministic.
	// BuildVoteTally does not depend on the order.
	index := make(map[timelineKey]int)
	var standing []sqlcgen.CurrentVotesForReportsRow

	prevVis, _ := Resolve(in.Meta, VoteTally{}, in.CreatedAt)
	// A report that is Live from the start (critical) is never "shown to
	// everyone" at a later step.
	shown := prevVis == VisibilityLive
	reachedConfirm, reachedDispute := 0, 0

	for _, row := range log {
		key := timelineKey{account: row.AccountID, kind: row.Kind}
		cur := sqlcgen.CurrentVotesForReportsRow{
			ReportID:    in.ReportID,
			AccountID:   row.AccountID,
			Kind:        row.Kind,
			Value:       row.Value,
			GeohashCell: row.GeohashCell,
			CreatedAt:   row.CreatedAt,
		}
		if i, ok := index[key]; ok {
			// Latest wins by (created_at, id), the order of the DISTINCT ON.
			if !row.CreatedAt.Before(standing[i].CreatedAt) {
				standing[i] = cur
			}
		} else {
			index[key] = len(standing)
			standing = append(standing, cur)
		}

		tally := BuildVoteTally(standing, in.ReportID, in.ReporterAccountID)
		vis, _ := Resolve(in.Meta, tally, row.CreatedAt)
		at := row.CreatedAt

		closureSaysYou := false

		// Resolved and reopened: the visibility flips to or from Retracted.
		if vis == VisibilityRetracted && prevVis != VisibilityRetracted {
			by := ClosedByFor(Closure{
				ByReporter: tally.ReporterResolved,
				ByPeople:   tally.ResolveCells >= IndependentAgreementThreshold,
				At:         at,
			}, viewerIsReporter)
			closureSaysYou = by == ClosedByYou || by == ClosedByYouAndPeople
			add(row.ID, timelineSubClosure, TimelineResolved, TimelineToneGood, timelineResolvedText(by), at, false)
		} else if prevVis == VisibilityRetracted && vis != VisibilityRetracted && VoteValue(row.Value) == VoteReopen {
			// Only a row that is a reopen writes the reopen line. A take back
			// (unresolve) that lifts a retraction is not a reopen (D-28).
			switch {
			case tally.ReporterReopened:
				add(row.ID, timelineSubClosure, TimelineReopened, TimelineToneNeutral, timelineReopenedText(true), at, false)
			case tally.ReopenCells >= IndependentAgreementThreshold:
				add(row.ID, timelineSubClosure, TimelineReopened, TimelineToneNeutral, timelineReopenedText(false), at, false)
			}
		}

		// Confirmed, and shown to everyone at the first Live step. One line per
		// step: the Live form replaces the milestone sentence.
		confirmMilestone := topMilestone(tally.ConfirmCells, reachedConfirm)
		if confirmMilestone > 0 {
			reachedConfirm = confirmMilestone
		}
		switch {
		case vis == VisibilityLive && !shown:
			shown = true
			if tally.ConfirmCells >= 1 {
				add(row.ID, timelineSubConfirm, TimelineConfirmed, TimelineToneGood, timelineConfirmedLiveText(tally.ConfirmCells), at, false)
			}
		case confirmMilestone > 0:
			add(row.ID, timelineSubConfirm, TimelineConfirmed, TimelineToneGood, timelineConfirmedText(confirmMilestone), at, false)
		}

		// Disputed, and marked Disputed when the step enters the Disputed view.
		disputeMilestone := topMilestone(tally.DisputeCells, reachedDispute)
		if disputeMilestone > 0 {
			reachedDispute = disputeMilestone
		}
		entersDisputed := vis == VisibilityHidden && prevVis != VisibilityHidden
		switch {
		case entersDisputed && disputeMilestone > 0:
			add(row.ID, timelineSubDispute, TimelineDisputed, TimelineToneBad, timelineDisputedMarkedText(disputeMilestone), at, false)
		case entersDisputed:
			add(row.ID, timelineSubDispute, TimelineDisputed, TimelineToneBad, timelineDisputedMarkedCountText(tally.DisputeCells), at, false)
		case disputeMilestone > 0:
			add(row.ID, timelineSubDispute, TimelineDisputed, TimelineToneBad, timelineDisputedText(disputeMilestone), at, false)
		}
		if prevVis == VisibilityHidden && vis != VisibilityHidden && vis != VisibilityRetracted {
			add(row.ID, timelineSubLeave, TimelineDisputed, TimelineToneNeutral, timelineNoLongerDisputed, at, false)
		}

		// The viewer's own lines, from the viewer's own rows with their real
		// times. The reporter's own content votes never count, so they get no
		// line either.
		if in.ViewerAccountID != 0 && row.AccountID == in.ViewerAccountID {
			switch VoteKind(row.Kind) {
			case VoteKindContent:
				if viewerIsReporter {
					break
				}
				switch VoteValue(row.Value) {
				case VoteConfirm:
					add(row.ID, timelineSubYours, TimelineYours, TimelineToneNeutral, timelineYouConfirmed, at, true)
				case VoteDispute:
					add(row.ID, timelineSubYours, TimelineYours, TimelineToneNeutral, timelineYouDisputed, at, true)
				}
			case VoteKindResolution:
				switch VoteValue(row.Value) {
				case VoteResolve:
					if !closureSaysYou {
						add(row.ID, timelineSubYours, TimelineYours, TimelineToneNeutral, timelineYouResolved, at, true)
					}
				case VoteReopen:
					add(row.ID, timelineSubYours, TimelineYours, TimelineToneNeutral, timelineYouReopened, at, true)
				}
			}
		}

		prevVis = vis
	}

	// A report that ran out its time without being resolved fades at its expiry.
	if ViewOf(prevVis, in.ExpiresAt, in.Now) == FeedViewPast {
		add(math.MaxInt64, 0, TimelineFaded, TimelineToneNeutral, timelineFaded, in.ExpiresAt, false)
	}

	sort.SliceStable(lines, func(i, j int) bool {
		a, b := lines[i], lines[j]
		if !a.ev.At.Equal(b.ev.At) {
			return a.ev.At.Before(b.ev.At)
		}
		if a.seq != b.seq {
			return a.seq < b.seq
		}
		return a.sub < b.sub
	})
	out := make([]TimelineEvent, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.ev)
	}
	return out
}
