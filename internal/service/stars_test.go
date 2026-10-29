package service

import "testing"

func TestStarsText(t *testing.T) {
	cases := []struct {
		points int
		want   string
	}{
		{-5, "0.0"},
		{0, "0.0"},
		{1, "0.0"},
		{34, "1.7"},
		{35, "1.7"},
		{36, "1.8"},
		{69, "3.4"},
		{70, "3.5"},
		{71, "3.5"},
		{99, "4.9"},
		{100, "5.0"},
		{101, "5.0"},
		{400, "5.0"},
	}
	for _, c := range cases {
		if got := StarsText(c.points); got != c.want {
			t.Errorf("StarsText(%d) = %q, want %q", c.points, got, c.want)
		}
	}
}

// TestStarsTextWalkMatchesTag walks every score and proves the shown stars
// and the tag agree: 3.5 stars or more exactly when the tag is Reliable
// reporter (research Pitfall 1, T-09-03).
func TestStarsTextWalkMatchesTag(t *testing.T) {
	for p := 0; p <= 100; p++ {
		reliableStars := StarsTenths(p) >= 35
		reliableTag := reporterTagForScore(p) == ReporterReliable
		if reliableStars != reliableTag {
			t.Errorf("score %d: stars %s (at least 3.5 is %v) but tag %s", p, StarsText(p), reliableStars, reporterTagForScore(p))
		}
	}
}
