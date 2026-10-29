package service

import (
	"testing"
	"time"
)

// TestExtendedExpiryRule pins the one rule behind backlog 999.12 with no
// database: the new fade time is the confirmation's time plus the lifetime,
// never earlier than the current fade time, never later than the cap counted
// from posting, and a report that has already faded is not revived.
func TestExtendedExpiryRule(t *testing.T) {
	c := time.Date(2026, 10, 30, 6, 0, 0, 0, time.UTC)
	h := func(n int) time.Time { return c.Add(time.Duration(n) * time.Hour) }

	cases := []struct {
		name      string
		current   time.Time
		confirmed time.Time
		life, cap time.Duration
		want      time.Time
	}{
		{"a new spot adds the lifetime from the vote", h(8), h(3), 8 * time.Hour, 24 * time.Hour, h(11)},
		{"never earlier than the current fade time", h(20), h(1), 8 * time.Hour, 24 * time.Hour, h(20)},
		{"capped at 24 hours after posting", h(23), h(20), 8 * time.Hour, 24 * time.Hour, h(24)},
		{"a Critical report is capped at 48 hours", h(47), h(40), 24 * time.Hour, 48 * time.Hour, h(48)},
		{"a Critical report under its cap", h(24), h(10), 24 * time.Hour, 48 * time.Hour, h(34)},
		{"a report fading right now is not revived", h(8), h(8), 8 * time.Hour, 24 * time.Hour, h(8)},
		{"a faded report is not revived", h(8), h(9), 8 * time.Hour, 24 * time.Hour, h(8)},
		{"at the cap stays", h(24), h(23), 8 * time.Hour, 24 * time.Hour, h(24)},
		{"a seeded time past the cap is never shortened", h(30), h(1), 8 * time.Hour, 24 * time.Hour, h(30)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtendedExpiry(c, tc.current, tc.confirmed, tc.life, tc.cap)
			if !got.Equal(tc.want) {
				t.Fatalf("ExtendedExpiry(created C, current %v, confirmed %v, life %v, cap %v) = C+%v, want C+%v",
					tc.current.Sub(c), tc.confirmed.Sub(c), tc.life, tc.cap, got.Sub(c), tc.want.Sub(c))
			}
		})
	}

	t.Run("the caps and lifetimes", func(t *testing.T) {
		for _, s := range []Severity{SeverityLow, SeverityMedium} {
			if got := ExpiryCap(s); got != 24*time.Hour {
				t.Fatalf("ExpiryCap(%s) = %v, want 24h", s, got)
			}
			if got := ExpiryDuration(s); got != 8*time.Hour {
				t.Fatalf("ExpiryDuration(%s) = %v, want 8h", s, got)
			}
		}
		if got := ExpiryCap(SeverityCritical); got != 48*time.Hour {
			t.Fatalf("ExpiryCap(critical) = %v, want 48h", got)
		}
		if got := ExpiryDuration(SeverityCritical); got != 24*time.Hour {
			t.Fatalf("ExpiryDuration(critical) = %v, want 24h", got)
		}
	})
}

// TestMayExtendOnlyWhileLive pins the Go side of the amendment A1 decision: a
// resolved or faded report is never extended, a live, provisional or disputed
// one may be.
func TestMayExtendOnlyWhileLive(t *testing.T) {
	now := time.Date(2026, 10, 30, 6, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	cases := []struct {
		name string
		vis  Visibility
		exp  time.Time
		want bool
	}{
		{"live", VisibilityLive, future, true},
		{"provisional", VisibilityProvisional, future, true},
		{"hidden", VisibilityHidden, future, true},
		{"retracted", VisibilityRetracted, future, false},
		{"withheld", VisibilityWithheld, future, false},
		{"faded", VisibilityLive, now, false},
		{"faded a second ago", VisibilityLive, now.Add(-time.Second), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mayExtend(tc.vis, tc.exp, now); got != tc.want {
				t.Fatalf("mayExtend(%s, expires %v, now %v) = %v, want %v", tc.vis, tc.exp, now, got, tc.want)
			}
		})
	}
}
