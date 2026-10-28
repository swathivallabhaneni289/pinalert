package official

import "time"

// FreshState is how far the stored official alerts can be trusted to be
// current. The server decides it from last_success_at alone and the page only
// shows it (ALERT-04, D-08, research Pattern 2): the browser recomputes none of
// it.
type FreshState string

const (
	// StateFresh: the last good check is at most FreshWindow old. This is the
	// only state in which an empty list may be read as "no alerts in force".
	StateFresh FreshState = "fresh"
	// StateStale: the last good check is older than FreshWindow and at most
	// StaleLimit old. The alerts still inside their own expiry are sent as the
	// last known ones, and no banner is raised.
	StateStale FreshState = "stale"
	// StateUnavailable: no good check ever, or the last one is older than
	// StaleLimit. No alerts are sent.
	StateUnavailable FreshState = "unavailable"
)

// The three honesty windows, named in one place and pinned by TestFreshConstants.
const (
	// FreshWindow is the longest age of a good check that still counts as fresh.
	FreshWindow = 30 * time.Minute
	// StaleLimit is the longest age of a good check that still counts as stale.
	StaleLimit = 6 * time.Hour
	// LazyKickAfter is how old the last attempt must be before a read of the
	// alerts starts a refresh in the background (the third refresh trigger).
	LazyKickAfter = 10 * time.Minute
)

// State classifies the last successful check. A nil lastSuccess is a feed that
// never succeeded. The age is zero in that case and also for a stored time that
// is ahead of now (a clock that ran fast), which counts as fresh rather than as
// a negative age.
func State(lastSuccess *time.Time, now time.Time) (FreshState, time.Duration) {
	if lastSuccess == nil || lastSuccess.IsZero() {
		return StateUnavailable, 0
	}
	age := now.Sub(*lastSuccess)
	if age < 0 {
		age = 0
	}
	switch {
	case age <= FreshWindow:
		return StateFresh, age
	case age <= StaleLimit:
		return StateStale, age
	default:
		return StateUnavailable, age
	}
}
