// updates.go is the pure decision core of in app updates (plan 03.2-02): the
// closed list of what an update can say, DecideUpdate (when an update is
// written and when the cursor only moves silently) and MayNotifyAboutReport
// (the one rule for whether a report may be announced to anyone). Nothing here
// does I/O, reads a clock or logs, like Resolve and ClassifyOutcome, so every
// rule is a table test with injected times.

package service

import "time"

// UpdateKind is the closed set of things an update can say. A later phase that
// adds a kind (comment, Phase 8) extends this list and its test.
type UpdateKind string

const (
	// UpdateReachedLive says a report is now confirmed by people nearby.
	UpdateReachedLive UpdateKind = "reached_live"
	// UpdateHidden says a report is now disputed (the Disputed tab word).
	UpdateHidden UpdateKind = "hidden"
	// UpdateResolved says a report was marked resolved.
	UpdateResolved UpdateKind = "resolved"
	// UpdateFading says a report is within an hour of expiring.
	UpdateFading UpdateKind = "fading"
	// UpdateReliableNearby says a Reliable reporter posted nearby.
	UpdateReliableNearby UpdateKind = "reliable_nearby"
	// UpdateMore marks the rolled up row of D-16 ("And N more changes"). It is
	// never an event: it is never stored and never decided, only built when a
	// list is read.
	UpdateMore UpdateKind = "more"
)

// UpdateEventKinds is the five kinds that are real events, in display order.
var UpdateEventKinds = []UpdateKind{UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading, UpdateReliableNearby}

// UpdateKinds is the six kinds a list row can carry: the five events and the
// rolled up marker.
var UpdateKinds = []UpdateKind{UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading, UpdateReliableNearby, UpdateMore}

// Valid reports whether k is one of the six kinds. Comparison is exact.
func (k UpdateKind) Valid() bool {
	for _, x := range UpdateKinds {
		if x == k {
			return true
		}
	}
	return false
}

// IsEvent reports whether k is one of the five event kinds (never more).
func (k UpdateKind) IsEvent() bool {
	for _, x := range UpdateEventKinds {
		if x == k {
			return true
		}
	}
	return false
}

const (
	// UpdateReportGap is the least time between two updates about one report
	// for one account (D-12).
	UpdateReportGap = time.Hour
	// FadingLead is how long before expiry the fading event may fire (D-14).
	FadingLead = time.Hour
	// UpdatesHourlyCeiling is how many updates one account sees per hour
	// before the rest roll up into "And N more changes" (D-16).
	UpdatesHourlyCeiling = 10
	// UpdatesRetention is how long an update is kept (D-16, 7 days).
	UpdatesRetention = HistoryWindow
	// FollowedReportsCap bounds the followed set of one refresh (research
	// Pattern 3). A heavy account loses the oldest reports first.
	FollowedReportsCap = 200
	// BellListSize is how many rows the bell list shows (D-02).
	BellListSize = 10
	// ActivityUpdatesListSize is how many rows My activity shows (UI-SPEC B1).
	ActivityUpdatesListSize = 50
	// UnreadCountCap is the largest number the unread count shows as a plain
	// numeral (UI-SPEC A1).
	UnreadCountCap = 99
	// ReliableAlertsPerReporterPerHour is the NOTIF-05 cap per reporter per
	// recipient per hour. One constant, so the number is easy to change.
	ReliableAlertsPerReporterPerHour = 1
	// MutedListCap bounds how many reports one account may mute.
	MutedListCap = 100
	// ReportLineMaxRunes is the longest first line of a report an update shows.
	ReportLineMaxRunes = 160
)

// KindForState maps the three visibilities that announce a change to their
// event kind: Live to reached_live, Hidden to hidden, Retracted to resolved.
// Provisional, Withheld and anything unknown announce nothing.
func KindForState(v Visibility) (UpdateKind, bool) {
	switch v {
	case VisibilityLive:
		return UpdateReachedLive, true
	case VisibilityHidden:
		return UpdateHidden, true
	case VisibilityRetracted:
		return UpdateResolved, true
	default:
		return "", false
	}
}

// UpdateCursor is one account's position on one followed report: the last
// state it was told about, when it was last told anything, when the cursor
// began following, and the expiry the fading event last fired for.
type UpdateCursor struct {
	LastState    Visibility
	LastUpdateAt *time.Time
	AdoptedAt    time.Time
	FadingFor    *time.Time
}

// DecideInput is everything DecideUpdate reads. View is the caller's ViewOf
// answer and Visibility the caller's Resolve answer (with the report's
// moderation state set); neither is derived again here.
type DecideInput struct {
	// Cursor is nil when the account has no cursor row for the report yet.
	Cursor     *UpdateCursor
	Visibility Visibility
	View       FeedView
	ExpiresAt  time.Time
	// OwnLatestVoteAt is the account's own latest vote on the report, if any.
	OwnLatestVoteAt *time.Time
	Muted           bool
	Now             time.Time
}

// DecideAction says what the caller must write.
type DecideAction int

const (
	// DecideNothing writes nothing and leaves the cursor alone.
	DecideNothing DecideAction = iota
	// DecideBaseline creates the cursor at NewState with no update.
	DecideBaseline
	// DecideAdopt moves the cursor to NewState silently, with no update.
	DecideAdopt
	// DecideEvent writes one update of Kind and moves the cursor to NewState.
	DecideEvent
	// DecideFading writes one fading update, records the expiry it fired for
	// and keeps the announced state at NewState.
	DecideFading
)

// Decision is the answer of DecideUpdate. Kind is set only for DecideEvent and
// DecideFading; NewState is set for every action but DecideNothing.
type Decision struct {
	Action   DecideAction
	Kind     UpdateKind
	NewState Visibility
}

// DecideUpdate decides what one account's refresh does about one followed
// report. The rules run in this order and the first that applies wins:
//
//  1. Muted (D-11): nothing is written.
//  2. Withheld (held or removed after review): nothing is written and the
//     cursor is not moved, even with no cursor yet. A row shown later reads as
//     no longer available.
//  3. No cursor yet: the current visibility becomes the baseline silently, so
//     a person who already has 40 votes on open reports is not flooded.
//  4. Self caused (D-13): when the account's own latest vote is newer than the
//     cursor's AdoptedAt, the current visibility is adopted silently, so the
//     person's own confirm or Mark resolved never tells them about itself.
//  5. State change (D-12): when the visibility is Live, Hidden or Retracted
//     and differs from the cursor's last state, one event is written if the
//     last update was at least UpdateReportGap ago; inside the hour nothing is
//     written and the cursor does not move, so the next poll checks again (a
//     change that reverts inside the hour is never reported, on purpose).
//     Provisional never moves the announced state, so Live, Provisional, Live
//     collapses to nothing.
//  6. Fading (D-14): when the view is active, now is before the expiry, the
//     expiry is within FadingLead, it is not the expiry already announced and
//     the hour gap allows, one fading event is written. It never fires after
//     expiry, and a state event in the same call wins over it.
//
// Anything else writes nothing.
func DecideUpdate(in DecideInput) Decision {
	if in.Muted {
		return Decision{Action: DecideNothing}
	}
	if in.Visibility == VisibilityWithheld {
		return Decision{Action: DecideNothing}
	}
	if in.Cursor == nil {
		return Decision{Action: DecideBaseline, NewState: in.Visibility}
	}
	if in.OwnLatestVoteAt != nil && in.OwnLatestVoteAt.After(in.Cursor.AdoptedAt) {
		return Decision{Action: DecideAdopt, NewState: in.Visibility}
	}

	gapAllows := in.Cursor.LastUpdateAt == nil || !in.Now.Before(in.Cursor.LastUpdateAt.Add(UpdateReportGap))

	if kind, announces := KindForState(in.Visibility); announces && in.Visibility != in.Cursor.LastState {
		if !gapAllows {
			return Decision{Action: DecideNothing}
		}
		return Decision{Action: DecideEvent, Kind: kind, NewState: in.Visibility}
	}

	if in.View == FeedViewActive &&
		in.Now.Before(in.ExpiresAt) &&
		!in.ExpiresAt.After(in.Now.Add(FadingLead)) &&
		(in.Cursor.FadingFor == nil || !in.Cursor.FadingFor.Equal(in.ExpiresAt)) &&
		gapAllows {
		return Decision{Action: DecideFading, Kind: UpdateFading, NewState: in.Cursor.LastState}
	}
	return Decision{Action: DecideNothing}
}

// MayNotifyAboutReport is the one eligibility rule (D-29) for announcing a
// report to anyone: the alert step of the refresh and Phase 3.3's phone
// channel both call it, and a second copy of it anywhere is a bug. It asks the
// resolver's output, never the report's own severity, so the Provisional gate
// cannot be bypassed by a second copy of the critical rule.
//
// A report may be announced only while it is active: ViewOf says active, which
// already excludes Hidden, Retracted, Withheld and expired reports. On top of
// that, the Reliable reporter note (UpdateReliableNearby) is allowed for Live
// and Provisional alike, because the note comes at posting for critical and
// non critical reports (owner amendment 2026-10-18, replacing D-26's only once
// Live). Every other kind needs Live, so the Provisional gate holds for
// anything Phase 3.3 adds.
//
// It is a free function on purpose: a bool method on Visibility, or a
// signature that names FeedView, would have to be exercised in
// TestWithheldInNoView by its source guard.
func MayNotifyAboutReport(kind UpdateKind, vis Visibility, expiresAt, now time.Time) bool {
	if ViewOf(vis, expiresAt, now) != FeedViewActive {
		return false
	}
	if kind == UpdateReliableNearby {
		return true
	}
	return vis == VisibilityLive
}

// HourBucket truncates t to its UTC clock hour. The Reliable alert uses it to
// make "one per reporter per recipient per hour" a unique key.
func HourBucket(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}
