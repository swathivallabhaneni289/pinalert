// pushkinds_test.go pins the closed list of phone notification kinds and the
// phase's named caps (plan 03.3-03). A change to either is a deliberate edit of
// this file.

package service

import (
	"testing"
	"time"
)

// Amended 2026-10-29 by plan 05-11 (D-11): official is a built kind, follow
// stays reserved. The four 3.3 kinds, their order, their wire values and the
// refusal of every unknown value are unchanged; only the count (four to five),
// the new last entry and the reserved list (official and follow to follow alone)
// moved.
//
// Amended 2026-10-30 by Phase 9 plan 25 (D-09): follow is a built kind too, so
// PushKinds holds six (follow is the new last entry) and ReservedPushKinds is
// empty. The wire values, the order of the first five and the refusal of every
// unknown value are unchanged.
func TestPushKindsClosed(t *testing.T) {
	built := []PushKind{PushKindCritical, PushKindReliable, PushKindChanged, PushKindConfirmedUpdate, PushKindOfficial, PushKindFollow}
	if len(PushKinds) != len(built) {
		t.Fatalf("PushKinds has %d entries, want %d", len(PushKinds), len(built))
	}
	for i, k := range built {
		if PushKinds[i] != k {
			t.Errorf("PushKinds[%d] = %q, want %q", i, PushKinds[i], k)
		}
		if !k.Valid() {
			t.Errorf("%q is not valid", k)
		}
	}

	wire := map[PushKind]string{
		PushKindCritical:        "critical",
		PushKindReliable:        "reliable",
		PushKindChanged:         "changed",
		PushKindConfirmedUpdate: "confirmed_update",
		PushKindOfficial:        "official",
		PushKindFollow:          "follow",
	}
	for k, want := range wire {
		if string(k) != want {
			t.Errorf("kind value %q, want %q", k, want)
		}
	}

	if len(ReservedPushKinds) != 0 {
		t.Fatalf("ReservedPushKinds = %v, want empty (follow was built by plan 09-25)", ReservedPushKinds)
	}
	if !PushKindOfficial.Valid() {
		t.Error("official must be valid: it is built by plan 05-11")
	}
	if !PushKindFollow.Valid() {
		t.Error("follow must be valid: it is built by plan 09-25")
	}
	seen := 0
	for _, k := range PushKinds {
		if k == PushKindFollow {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("PushKinds holds follow %d times, want once", seen)
	}
	for _, k := range ReservedPushKinds {
		if k.Valid() {
			t.Errorf("reserved kind %q must not be valid (no switch, no storage)", k)
		}
	}

	// There is no rolled up kind and no unknown value is valid.
	for _, k := range []PushKind{"more", "", "Critical", "critical ", "comment"} {
		if k.Valid() {
			t.Errorf("PushKind(%q).Valid() = true, want false", string(k))
		}
	}
}

// Amended 2026-10-30 by Phase 9 plan 25 (D-09, D-25): every value is unchanged;
// FollowFanoutCap (500) joins the table, and the new assertions at the end say a
// follow push counts toward the hourly cap and is held back by it like
// Reliable, so follow notes stay few.
func TestPushCapsAreNamed(t *testing.T) {
	ints := []struct {
		name      string
		got, want int
	}{
		{"PushPerAccountPerHour", PushPerAccountPerHour, 4},
		{"PushCriticalReportsPerReporterPerHour", PushCriticalReportsPerReporterPerHour, 3},
		{"PushRecipientsPerReport", PushRecipientsPerReport, 2000},
		{"PushDevicesPerAccount", PushDevicesPerAccount, 8},
		{"FollowFanoutCap", FollowFanoutCap, 500},
	}
	for _, c := range ints {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}

	durations := []struct {
		name      string
		got, want time.Duration
	}{
		{"PushInitialWindow", PushInitialWindow, time.Hour},
		{"PushChangedWindow", PushChangedWindow, 2 * time.Hour},
		{"PushCriticalTTLMax", PushCriticalTTLMax, time.Hour},
		{"PushOtherTTL", PushOtherTTL, 30 * time.Minute},
		{"PushOutboxRetention", PushOutboxRetention, 7 * 24 * time.Hour},
	}
	for _, c := range durations {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	if !DefaultCriticalPushOn {
		t.Error("DefaultCriticalPushOn must be true (decision 12)")
	}
	if DefaultChangesPushOn {
		t.Error("DefaultChangesPushOn must be false")
	}
	if DefaultAlertRadiusForPushKm != 5 {
		t.Errorf("DefaultAlertRadiusForPushKm = %v, want 5", DefaultAlertRadiusForPushKm)
	}

	// A follow push is held by the hourly cap once the person already got
	// PushPerAccountPerHour, and not before (D-25: few notes, never piled up).
	if heldByHourlyCap(PushKindFollow, PushPerAccountPerHour-1) {
		t.Error("a follow push must not be held below the hourly cap")
	}
	if !heldByHourlyCap(PushKindFollow, PushPerAccountPerHour) {
		t.Error("a follow push must be held at the hourly cap")
	}
	if heldByHourlyCap(PushKindCritical, PushPerAccountPerHour+5) {
		t.Error("a Critical push is never held by the hourly cap")
	}
}
