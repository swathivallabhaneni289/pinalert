// pushkinds.go is the vocabulary of phone notifications (plan 03.3-03): the
// closed list of kinds a push can be, the two kind names reserved for later
// phases, and the named caps of the phase. Nothing here does I/O, so every
// rule is a table test.

package service

import "time"

// PushKind is the closed set of things a phone notification can be.
//
// Four kinds are built in this phase. Two more names are reserved so a later
// phase has a place to land without renaming anything: official arrives with
// Phase 5 (05-CONTEXT D-11) and follow with Phase 9's per person Notify me
// bell (09-CONTEXT D-08 to D-11), each with its own migration. Neither gets a
// Settings row from Phase 3.3, and Valid refuses both until that phase widens
// the list and its test. There is no rolled up kind: a push dropped by a cap
// shows nothing extra.
type PushKind string

const (
	// PushKindCritical is a Critical report near the home area (kind 1).
	PushKindCritical PushKind = "critical"
	// PushKindReliable is a report from a Reliable reporter near the home area
	// (kind 4).
	PushKindReliable PushKind = "reliable"
	// PushKindChanged is a change to a report the person posted or voted on
	// (kind 3).
	PushKindChanged PushKind = "changed"
	// PushKindConfirmedUpdate is the in place replacement of a critical or
	// reliable notification once people nearby confirm the report (kind 5). Its
	// words are built from the original kind with Confirmed set.
	PushKindConfirmedUpdate PushKind = "confirmed_update"

	// PushKindOfficial is reserved for Phase 5. It has no switch and no storage.
	PushKindOfficial PushKind = "official"
	// PushKindFollow is reserved for Phase 9. It has no switch and no storage.
	PushKindFollow PushKind = "follow"
)

// PushKinds is the four built kinds, in the order the sender considers them
// for display and storage.
var PushKinds = []PushKind{PushKindCritical, PushKindReliable, PushKindChanged, PushKindConfirmedUpdate}

// ReservedPushKinds is the two names held for later phases.
var ReservedPushKinds = []PushKind{PushKindOfficial, PushKindFollow}

// Valid reports whether k is one of the four built kinds. The reserved names,
// the empty string and anything else are refused. Comparison is exact.
func (k PushKind) Valid() bool {
	for _, x := range PushKinds {
		if x == k {
			return true
		}
	}
	return false
}

// The caps of the phone channel. A push dropped by a cap shows nothing extra
// (decision 6): there is no "and N more" notification, and the bell holds the
// full list of the stored kinds. A Critical alert is push only and is never held
// by the hourly cap (heldByHourlyCap), so it is never lost to it. The values are
// pinned by TestPushCapsAreNamed so a change is a deliberate edit.
const (
	// PushPerAccountPerHour is how many pushes one person gets in a rolling
	// hour. It counts critical, reliable and changed pushes, and not confirmed
	// updates, which replace a notification already on the screen. A Critical
	// push is counted but never held back by this limit (heldByHourlyCap), so
	// the other kinds are held back first.
	PushPerAccountPerHour = 4
	// PushCriticalReportsPerReporterPerHour is how many Critical reports from
	// one reporter may be pushed per hour across all recipients. The per
	// reporter fan out caps stand in for TRUSTX-03 burst detection, which is
	// not built; the lower 3.5 star Reliable bar of Phase 9 is a further reason
	// to keep them. Recorded in docs/TRUST-MODEL.md.
	PushCriticalReportsPerReporterPerHour = 3
	// PushReliableReportsPerReporterPerHour is the same cap for Reliable
	// reporter pushes.
	PushReliableReportsPerReporterPerHour = 2
	// PushRecipientsPerReport is the most people one report is pushed to.
	PushRecipientsPerReport = 2000
	// PushDevicesPerAccount is the most devices one account may subscribe.
	PushDevicesPerAccount = 8
)

// Windows and lifetimes of a push.
const (
	// PushInitialWindow is how long after posting a report may still be pushed
	// as a new report.
	PushInitialWindow = time.Hour
	// PushChangedWindow is how long a change may still be pushed after it
	// happened.
	PushChangedWindow = 2 * time.Hour
	// PushCriticalTTLMax is the longest a Critical push waits for a device that
	// is offline.
	PushCriticalTTLMax = time.Hour
	// PushOtherTTL is the same wait for every other kind.
	PushOtherTTL = 30 * time.Minute
	// PushOutboxRetention is how long a sent or dropped push row is kept.
	PushOutboxRetention = 7 * 24 * time.Hour
)

// Defaults of the two switches. The SQL COALESCE in push.sql uses the same
// values, and a test in plan 12 pins both.
const (
	// DefaultCriticalPushOn is on: Critical reports near the home area are the
	// point of the feature (decision 12).
	DefaultCriticalPushOn = true
	// DefaultChangesPushOn is off: changes to reports a person posted or voted
	// on are opt in.
	DefaultChangesPushOn = false
	// DefaultAlertRadiusForPushKm is the radius used for a person with no
	// radius of their own.
	DefaultAlertRadiusForPushKm = 5
)
