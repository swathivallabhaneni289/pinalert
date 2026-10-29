// pushkinds.go is the vocabulary of phone notifications (plan 03.3-03): the
// closed list of kinds a push can be, the two kind names reserved for later
// phases, and the named caps of the phase. Nothing here does I/O, so every
// rule is a table test.

package service

import "time"

// PushKind is the closed set of things a phone notification can be.
//
// Four kinds were built in Phase 3.3. The official kind arrived with Phase 5
// plan 05-11 (D-11); its rows live in official_push_outbox, never in
// push_outbox, and its words are built by BuildOfficialPushText, not by
// BuildPushText. The follow kind arrived with Phase 9 plan 25 (D-09, D-11): the
// per person Notify me bell replaced any Settings switch, so the kind has no
// account switch and no Settings row; its rows live in push_outbox with the
// first alert kinds. There is no rolled up kind: a push dropped by a cap shows
// nothing extra.
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

	// PushKindOfficial is a Severe or Extreme official alert for the home area
	// (Phase 5, D-11 and D-16). Its rows are in official_push_outbox and its
	// words come from BuildOfficialPushText.
	PushKindOfficial PushKind = "official"

	// PushKindFollow is a report by a person the reader follows with the Notify
	// me bell on (Phase 9 plan 25, D-09 and D-11). It is a first alert kind: one
	// per account and report across critical, follow and reliable
	// (uq_push_outbox_first, whose predicate names all three),
	// queued from the followed_posted bell row. It has no account switch; the
	// bell on the follow is the switch.
	PushKindFollow PushKind = "follow"
)

// PushKinds is the six built kinds, in the order the sender considers them
// for display and storage.
var PushKinds = []PushKind{PushKindCritical, PushKindReliable, PushKindChanged, PushKindConfirmedUpdate, PushKindOfficial, PushKindFollow}

// ReservedPushKinds is the names held for a later phase. It is empty: Phase 9
// plan 25 built the follow kind, the last one that was held.
var ReservedPushKinds = []PushKind{}

// Valid reports whether k is one of the six built kinds. A reserved name, the
// empty string and anything else are refused. Comparison is exact.
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
	// hour. It counts critical, reliable, follow and changed pushes (a follow
	// push counts, Phase 9 D-25), and not confirmed updates, which replace a
	// notification already on the screen. A Critical
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

// The caps and windows of the official alert push (plan 05-11, D-11 and D-16),
// pinned by TestOfficialPushCapsAreNamed so a change is a deliberate edit. The
// official rows are in their own table and never count toward
// PushPerAccountPerHour (heldByHourlyCap), and a push dropped by a cap shows
// nothing extra.
const (
	// OfficialPushPerAccountPer6Hours is how many official pushes one person
	// gets in OfficialPushCapWindow. It is checked at send time (plan 05-16).
	OfficialPushPerAccountPer6Hours = 3
	// OfficialPushCapWindow is the rolling window of that cap.
	OfficialPushCapWindow = 6 * time.Hour
	// OfficialPushRecipientsPerAlert is the most people one alert version is
	// queued for.
	OfficialPushRecipientsPerAlert = 2000
	// OfficialPushTTL is the longest an official push waits for a device that is
	// offline.
	OfficialPushTTL = 2 * time.Hour
	// OfficialPushSentWindow is how long after an alert was sent it may still be
	// queued as a push.
	OfficialPushSentWindow = time.Hour
	// OfficialPushExpiryMargin is how close to its end an alert may be and still
	// be queued: an alert ending within this margin is not pushed.
	OfficialPushExpiryMargin = 10 * time.Minute
)

// DefaultOfficialPushOn is on: Severe and Extreme official alerts for the home
// area are the point of the feature (D-11). The SQL COALESCE in
// official_push.sql uses the same value, and TestOfficialPushDefaultMatchesSQL
// pins both.
const DefaultOfficialPushOn = true

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
