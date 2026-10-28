package demo

import (
	"strings"
	"testing"
	"time"

	"pinalert/internal/areas"
)

var sampleNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

// TestSampleView (plan 05-19, D-05, D-06, ROBUST-06): the Demo's alerts go
// through the same pure view as the real page. The view is fresh with age 0, the
// reader is the pretend position's district, the Extreme and Severe samples
// stand in it with Extreme first, the Moderate sample is across the state, every
// row is a Demo row with no authority badge and no issuer channel as its
// source, and the banner is the Extreme sample with the Demo flag.
func TestSampleView(t *testing.T) {
	d, ok := areas.Default().Locate(PretendLat, PretendLon)
	if !ok {
		t.Fatalf("the pretend position (%v, %v) is in no district: the sample cannot be placed", PretendLat, PretendLon)
	}
	if got := strings.ToUpper(d.State); got != "ANDHRA PRADESH" {
		t.Fatalf("the pretend position is in state %q, but the sample text says Andhra Pradesh", d.State)
	}

	v := SampleView(NewWorld("sample", nil), sampleNow)
	if v.State != "fresh" {
		t.Errorf("state = %q, want fresh", v.State)
	}
	if v.AgeSeconds == nil || *v.AgeSeconds != 0 {
		t.Errorf("age = %v, want 0", v.AgeSeconds)
	}
	if !v.AreaKnown {
		t.Error("AreaKnown = false, want true")
	}
	if v.Count != 3 || len(v.Groups) != 2 {
		t.Fatalf("count %d, groups %d, want 3 rows in 2 groups", v.Count, len(v.Groups))
	}

	reader, state := v.Groups[0], v.Groups[1]
	if reader.Kind != "reader" || reader.Label != "Where you are: "+d.Name+" district" {
		t.Errorf("reader group = %q %q, want reader and the pretend position's district", reader.Kind, reader.Label)
	}
	if len(reader.Rows) != 2 || reader.Rows[0].Level != "extreme" || reader.Rows[1].Level != "severe" {
		t.Fatalf("reader rows = %+v, want Extreme then Severe", reader.Rows)
	}
	if state.Kind != "state" || state.Label != "Across Andhra Pradesh" {
		t.Errorf("state group = %q %q, want state and Across Andhra Pradesh", state.Kind, state.Label)
	}
	if len(state.Rows) != 1 || state.Rows[0].Level != "moderate" {
		t.Fatalf("state rows = %+v, want one Moderate", state.Rows)
	}

	seen := map[int64]bool{}
	for _, g := range v.Groups {
		for _, r := range g.Rows {
			if !r.Demo || r.Badge != "" {
				t.Errorf("row %d: demo %v badge %q, want a Demo row with no badge", r.ID, r.Demo, r.Badge)
			}
			if seen[r.ID] {
				t.Errorf("row id %d is used twice", r.ID)
			}
			seen[r.ID] = true
			if r.Details.SourceLine == "" || strings.Contains(r.Details.SourceLine, "NDMA") {
				t.Errorf("row %d source line = %q, want a line that does not name NDMA", r.ID, r.Details.SourceLine)
			}
			if r.Headline == "" || r.Issuer == "" {
				t.Errorf("row %d has no headline or issuer", r.ID)
			}
			if !r.ExpiresAt.After(sampleNow) || !r.IssuedAt.Before(sampleNow) {
				t.Errorf("row %d issued %v expires %v, want issued before and expiry after %v", r.ID, r.IssuedAt, r.ExpiresAt, sampleNow)
			}
		}
	}

	b := v.Banner
	if b == nil {
		t.Fatal("no banner for the sample Extreme alert")
	}
	if !b.Demo || b.Level != "extreme" || b.More != 1 {
		t.Errorf("banner = %+v, want the Extreme sample with Demo true and 1 more", *b)
	}
	if b.EventPhrase != "Flood warning" || b.Area != d.Name+" district" {
		t.Errorf("banner words = %q in %q, want Flood warning in %s district", b.EventPhrase, b.Area, d.Name)
	}
	if b.AlertID != reader.Rows[0].ID {
		t.Errorf("banner alert id = %d, want the Extreme row %d", b.AlertID, reader.Rows[0].ID)
	}
}

// TestSampleViewIsRelativeToNow: the times follow the clock handed in, and a nil
// world is accepted (the sample is the same for every visit).
func TestSampleViewIsRelativeToNow(t *testing.T) {
	later := sampleNow.Add(5 * time.Hour)
	a, b := SampleView(nil, sampleNow), SampleView(nil, later)
	ra, rb := a.Groups[0].Rows[0], b.Groups[0].Rows[0]
	if !rb.IssuedAt.Equal(ra.IssuedAt.Add(5 * time.Hour)) {
		t.Errorf("issued %v then %v, want the second 5 hours after the first", ra.IssuedAt, rb.IssuedAt)
	}
	for _, s := range SampleAlerts {
		if !s.Sample || !strings.HasPrefix(s.Key, SampleKeyPrefix) {
			t.Errorf("sample alert %q is not marked as a sample", s.Key)
		}
	}
}
