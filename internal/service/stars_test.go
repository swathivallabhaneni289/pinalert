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

// TestCombinePoints pins the integer arithmetic of the combined points (D-02,
// research 1.4, review C1): below five raters the outcome is returned as it is,
// from five on the points are half the outcome and half the tapped average,
// rounded half up.
func TestCombinePoints(t *testing.T) {
	cases := []struct {
		name                  string
		outcome, raters, taps int
		want                  int
	}{
		{"four raters keep the outcome", 86, 4, 4, 86},
		{"no raters keep the outcome", 10, 0, 0, 10},
		{"exactly 40 with five fives is 70", 40, 5, 25, 70},
		{"39 with five fives rounds 69.5 up to 70", 39, 5, 25, 70},
		{"38 with five fives stays below the line", 38, 5, 25, 69},
		{"86 with five ones is 53", 86, 5, 5, 53},
		{"100 with five ones is 60", 100, 5, 5, 60},
		{"70 with six two star ratings is 55", 70, 6, 12, 55},
		{"100 with five fives is 100", 100, 5, 25, 100},
		{"0 with five ones is 10", 0, 5, 5, 10},
		{"a score of 0 with five fives is 50", 0, 5, 25, 50},
	}
	for _, c := range cases {
		if got := CombinePoints(c.outcome, c.raters, c.taps); got != c.want {
			t.Errorf("%s: CombinePoints(%d, %d, %d) = %d, want %d", c.name, c.outcome, c.raters, c.taps, got, c.want)
		}
	}

	// The result stays on the 0 to 100 scale for every outcome, every rater count
	// from the minimum to 40 and every possible tap sum.
	for outcome := 0; outcome <= 100; outcome += 7 {
		for raters := MinRatersForStars; raters <= 40; raters++ {
			for taps := raters; taps <= 5*raters; taps += 3 {
				if got := CombinePoints(outcome, raters, taps); got < 0 || got > 100 {
					t.Fatalf("CombinePoints(%d, %d, %d) = %d, outside 0 to 100", outcome, raters, taps, got)
				}
			}
		}
	}
}

// TestTappedHalfStars pins the half star rounding of the tapped part (review W5)
// and its text.
func TestTappedHalfStars(t *testing.T) {
	cases := []struct {
		raters, taps int
		halves       int
		text         string
	}{
		{12, 48, 8, "4.0"},
		{5, 22, 9, "4.5"},
		{5, 21, 8, "4.0"},
		{5, 5, 2, "1.0"},
		{5, 25, 10, "5.0"},
		{0, 0, 0, "0.0"},
	}
	for _, c := range cases {
		got := TappedHalfStars(c.raters, c.taps)
		if got != c.halves {
			t.Errorf("TappedHalfStars(%d, %d) = %d, want %d", c.raters, c.taps, got, c.halves)
		}
		if text := HalfStarsText(got); text != c.text {
			t.Errorf("HalfStarsText(%d) = %q, want %q", got, text, c.text)
		}
	}
	if got := HalfStarsText(-3); got != "0.0" {
		t.Errorf("HalfStarsText(-3) = %q, want 0.0", got)
	}
}
