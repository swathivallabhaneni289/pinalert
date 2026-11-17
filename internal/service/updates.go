// updates.go is the pure decision core of in app updates (plan 03.2-02): the
// closed list of what an update can say, DecideUpdate (when an update is
// written and when the cursor only moves silently) and MayNotifyAboutReport
// (the one rule for whether a report may be announced to anyone). Nothing here
// does I/O, reads a clock or logs, like Resolve and ClassifyOutcome, so every
// rule is a table test with injected times.

package service

import "time"

// UpdateKind is the closed set of things an update can say. A later phase that
// adds a kind extends this list and its test. Phase 8 added the two comment
// kinds, and Phase 9 added the follow note.
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
	// UpdateCommentOnYours says someone else commented on your report (Phase 8,
	// D-16). The row holds the kind and the report id only.
	UpdateCommentOnYours UpdateKind = "comment_on_yours"
	// UpdateReplyToYours says someone replied to your comment (Phase 8, D-16).
	// The row holds the kind and the report id only.
	UpdateReplyToYours UpdateKind = "reply_to_yours"
	// UpdateFollowedPosted says a person you follow, with Notify me on, posted a
	// report (Phase 9, D-09). The row holds the kind and the report id only; the
	// sentence is written at read time. It shares the Reliable reporter alert's
	// allowance in MayNotifyAboutReport and its once per account and report
	// index (uq_updates_first_alert).
	UpdateFollowedPosted UpdateKind = "followed_posted"
	// UpdateMore marks the rolled up row of D-16 ("And N more changes"). It is
	// never an event: it is never stored and never decided, only built when a
	// list is read.
	UpdateMore UpdateKind = "more"
)

// The four rows an author gets about their own report (quick 261117-vvb,
// owner 2026-11-17). The moderation paths write them straight to the author's
// updates, whatever their follows or mutes, at most once per report and kind
// (uq_updates_own_report). They are bell only: no phone note and no email.
// They are not in UpdateEventKinds, because they have no trust line and never
// go through DecideUpdate or MayNotifyAboutReport, but they are in UpdateKinds.
const (
	// UpdateOwnShown says a held report is now showing: the late text check
	// passed it or a moderator kept it.
	UpdateOwnShown UpdateKind = "own_shown"
	// UpdateOwnReview says the report is hidden while a moderator reviews it,
	// after flags from FlagHideThreshold different places.
	UpdateOwnReview UpdateKind = "own_review"
	// UpdateOwnKept says a moderator kept the report after a review hide.
	UpdateOwnKept UpdateKind = "own_kept"
	// UpdateOwnRemoved says the report was removed, by a moderator or by the
	// late text check.
	UpdateOwnRemoved UpdateKind = "own_removed"
)

// UpdateOwnKinds is the four rows about the reader's own report, in the order
// they can happen.
var UpdateOwnKinds = []UpdateKind{UpdateOwnShown, UpdateOwnReview, UpdateOwnKept, UpdateOwnRemoved}

// IsOwnReport reports whether k is one of the four rows about the reader's own
// report.
func (k UpdateKind) IsOwnReport() bool {
	for _, x := range UpdateOwnKinds {
		if x == k {
			return true
		}
	}
	return false
}

// UpdateCriticalNearby is a push only kind for the Critical report near the
// home area (Phase 3.3 kind 1). It needs Live like every kind except
// UpdateReliableNearby, which a Critical report is through the bypass, so it
// takes the existing Live branch of MayNotifyAboutReport and no second
// critical rule exists. It is never written to updates (00009's
// updates_kind_valid CHECK refuses it), so it is not in UpdateKinds.
const UpdateCriticalNearby UpdateKind = "critical_nearby"

// PushOnlyUpdateKinds is the kinds that reach MayNotifyAboutReport but are
// never stored: the phone channel's own kinds. Phase 3.2's UpdateKinds stays
// at its six stored kinds.
var PushOnlyUpdateKinds = []UpdateKind{UpdateCriticalNearby}

// NotifyFollowsCap is the most Notify me bells one account can have on at once
// (Phase 9, D-08). The follow note producer reads that many at most and the
// Settings switch counts against it, so one account cannot fan the producer out
// without limit.
const NotifyFollowsCap = 50

// UpdateEventKinds is the eight kinds that are real events, in display order.
// The two comment kinds (Phase 8, D-16) come after the five 3.2 events, and the
// follow note (Phase 9, D-09) after them.
var UpdateEventKinds = []UpdateKind{UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading, UpdateReliableNearby, UpdateCommentOnYours, UpdateReplyToYours, UpdateFollowedPosted}

// UpdateKinds is the thirteen kinds a list row can carry: the eight events,
// the rolled up marker and the four rows about the reader's own report (quick
// 261117-vvb). Comment rows never roll up into the marker and do not
// count toward UpdatesHourlyCeiling: the unread unique index of migration
// 00019 is their only cap (Phase 8 plan 11). The follow note is an ordinary
// event for the ceiling. The Reliable reporter alert is not: it has its own cap,
// ReliableHourlyCeiling (review WR-01, quick 261103-jal).
var UpdateKinds = []UpdateKind{UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading, UpdateReliableNearby, UpdateCommentOnYours, UpdateReplyToYours, UpdateFollowedPosted, UpdateMore, UpdateOwnShown, UpdateOwnReview, UpdateOwnKept, UpdateOwnRemoved}

// Valid reports whether k is one of the thirteen kinds. Comparison is exact.
func (k UpdateKind) Valid() bool {
	for _, x := range UpdateKinds {
		if x == k {
			return true
		}
	}
	return false
}

// IsEvent reports whether k is one of the eight event kinds (never more).
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
	// ReliableHourlyCeiling is how many Reliable reporter rows one account gets
	// per hour. It is its own cap: those rows are left out of UpdatesHourlyCeiling
	// so a busy hour of them never stops a follow note or rolls a change row up
	// (review WR-01, quick 261103-jal). It starts at the same value as that one.
	ReliableHourlyCeiling = 10
	// FollowHourlyCeiling is how many follow rows one account gets per hour. It
	// counts follow rows only, so a full hour of change or Reliable rows never
	// holds a follow row, whose phone note is queued only from a written row
	// (review follow up F2, quick 261103-jal). Change rows still count follow
	// rows toward UpdatesHourlyCeiling. It starts at the same value as that one.
	FollowHourlyCeiling = 10
	// ReliableBellLookBack is how far back the Reliable reporter bell row looks,
	// for every account with a home area. A stored enabled_at can only move the
	// start later, so the bell never fills with old reports now that the switch
	// is gone (owner 2026-11-03, quick 261103-jal, D4).
	ReliableBellLookBack = time.Hour
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
// Live). The follow note (UpdateFollowedPosted, Phase 9 D-09) takes the same
// allowance in the same condition, never a second copy of the rule: it also
// comes at posting, marked Not confirmed yet until people nearby confirm it.
// Every other kind needs Live, so the Provisional gate holds for anything
// Phase 3.3 adds.
//
// It is a free function on purpose: a bool method on Visibility, or a
// signature that names FeedView, would have to be exercised in
// TestWithheldInNoView by its source guard.
func MayNotifyAboutReport(kind UpdateKind, vis Visibility, expiresAt, now time.Time) bool {
	if ViewOf(vis, expiresAt, now) != FeedViewActive {
		return false
	}
	if kind == UpdateReliableNearby || kind == UpdateFollowedPosted {
		return true
	}
	return vis == VisibilityLive
}

// HourBucket truncates t to its UTC clock hour. The Reliable alert uses it to
// make "one per reporter per recipient per hour" a unique key.
func HourBucket(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}
