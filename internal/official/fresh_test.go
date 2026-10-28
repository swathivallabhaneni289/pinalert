package official

import (
	"testing"
	"time"
)

func TestState(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time {
		x := now.Add(-d)
		return &x
	}
	cases := []struct {
		name string
		last *time.Time
		want FreshState
		age  time.Duration
	}{
		{"just now", ago(0), StateFresh, 0},
		{"inside the window", ago(12 * time.Minute), StateFresh, 12 * time.Minute},
		{"exactly 30 minutes", ago(30 * time.Minute), StateFresh, 30 * time.Minute},
		{"just after 30 minutes", ago(30*time.Minute + time.Second), StateStale, 30*time.Minute + time.Second},
		{"two hours", ago(2 * time.Hour), StateStale, 2 * time.Hour},
		{"exactly 6 hours", ago(6 * time.Hour), StateStale, 6 * time.Hour},
		{"just after 6 hours", ago(6*time.Hour + time.Second), StateUnavailable, 6*time.Hour + time.Second},
		{"a day", ago(24 * time.Hour), StateUnavailable, 24 * time.Hour},
		{"never", nil, StateUnavailable, 0},
		{"a clock that ran ahead", ago(-time.Minute), StateFresh, 0},
	}
	for _, c := range cases {
		got, age := State(c.last, now)
		if got != c.want || age != c.age {
			t.Errorf("%s: State = %q, %v; want %q, %v", c.name, got, age, c.want, c.age)
		}
	}
}

func TestFreshConstants(t *testing.T) {
	if FreshWindow != 30*time.Minute || StaleLimit != 6*time.Hour || LazyKickAfter != 10*time.Minute {
		t.Fatalf("constants moved: %v %v %v", FreshWindow, StaleLimit, LazyKickAfter)
	}
}
