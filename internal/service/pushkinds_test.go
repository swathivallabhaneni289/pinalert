// pushkinds_test.go pins the closed list of phone notification kinds and the
// phase's named caps (plan 03.3-03). A change to either is a deliberate edit of
// this file.

package service

import (
	"testing"
	"time"
)

func TestPushKindsClosed(t *testing.T) {
	built := []PushKind{PushKindCritical, PushKindReliable, PushKindChanged, PushKindConfirmedUpdate}
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

	if len(ReservedPushKinds) != 2 {
		t.Fatalf("ReservedPushKinds has %d entries, want 2", len(ReservedPushKinds))
	}
	if ReservedPushKinds[0] != PushKindOfficial || ReservedPushKinds[1] != PushKindFollow {
		t.Errorf("ReservedPushKinds = %v, want official then follow", ReservedPushKinds)
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

func TestPushCapsAreNamed(t *testing.T) {
	ints := []struct {
		name      string
		got, want int
	}{
		{"PushPerAccountPerHour", PushPerAccountPerHour, 4},
		{"PushCriticalReportsPerReporterPerHour", PushCriticalReportsPerReporterPerHour, 3},
		{"PushReliableReportsPerReporterPerHour", PushReliableReportsPerReporterPerHour, 2},
		{"PushRecipientsPerReport", PushRecipientsPerReport, 2000},
		{"PushDevicesPerAccount", PushDevicesPerAccount, 8},
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
}
