package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

// These tests belong to the pure helpers of the updates handlers (plan
// 03.2-07): the row's accessible name, the full words time and the wire mapping.
// None of them needs a database.

var p32Now = time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)

// TestUpdateRowLabel pins the row's accessible name (UI-SPEC P32-C8): "New. "
// only when unread, the event line, the report line as its own sentence, the
// category and severity words, the trust sentence and the time in full words.
func TestUpdateRowLabel(t *testing.T) {
	ago := func(d time.Duration) time.Time { return p32Now.Add(-d) }
	id := int64(7)
	cases := []struct {
		name string
		item service.UpdateItem
		want string
	}{
		{
			name: "unread reached live",
			item: service.UpdateItem{
				Kind: service.UpdateReachedLive, ReportID: &id, Available: true, Unread: true,
				CreatedAt: ago(12 * time.Minute), Event: "Now confirmed",
				Category: service.CategoryFlood, Severity: service.SeverityCritical,
				Trust: "Confirmed by 3 people at different spots.",
			},
			want: "New. Now confirmed. Flood, Critical. Confirmed by 3 people at different spots. 12 minutes ago",
		},
		{
			name: "report line follows the event line as its own sentence",
			item: service.UpdateItem{
				Kind: service.UpdateReachedLive, ReportID: &id, Available: true, Unread: true,
				CreatedAt: ago(12 * time.Minute), Event: "Now confirmed", ReportLine: "Water over the bridge",
				Category: service.CategoryFlood, Severity: service.SeverityCritical,
				Trust: "Confirmed by 3 people at different spots.",
			},
			want: "New. Now confirmed. Water over the bridge. Flood, Critical. Confirmed by 3 people at different spots. 12 minutes ago",
		},
		{
			name: "a read item has no New prefix",
			item: service.UpdateItem{
				Kind: service.UpdateHidden, ReportID: &id, Available: true, Unread: false,
				CreatedAt: ago(3 * time.Hour), Event: "Now disputed",
				Category: service.CategoryRoadBlocked, Severity: service.SeverityMedium,
				Trust: "2 people nearby disputed this.",
			},
			want: "Now disputed. Road blocked, Medium. 2 people nearby disputed this. 3 hours ago",
		},
		{
			name: "an event line's own final stop is not doubled",
			item: service.UpdateItem{
				Kind: service.UpdateReliableNearby, ReportID: &id, Available: true, Unread: false,
				CreatedAt: ago(10 * time.Second), Event: "Reported nearby by a Reliable reporter.",
				Category: service.CategoryFlood, Severity: service.SeverityCritical,
				Trust: "Not confirmed yet.",
			},
			want: "Reported nearby by a Reliable reporter. Flood, Critical. Not confirmed yet. just now",
		},
		{
			name: "an unavailable item",
			item: service.UpdateItem{
				Kind: service.UpdateHidden, Available: false, CreatedAt: ago(2 * time.Hour),
				Event: "This report is no longer available",
			},
			want: "This report is no longer available. 2 hours ago",
		},
		{
			name: "a more item",
			item: service.UpdateItem{
				Kind: service.UpdateMore, Available: true, MoreCount: 4, CreatedAt: ago(time.Hour),
				Event: "And 4 more changes",
			},
			want: "And 4 more changes. 1 hour ago",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := updateRowLabel(tc.item, p32Now); got != tc.want {
				t.Errorf("updateRowLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRelativeTimeLong pins the full words time: profileRelativeTime's buckets
// and rounding, with whole words instead of "min".
func TestRelativeTimeLong(t *testing.T) {
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{-5 * time.Minute, "just now"},
		{0, "just now"},
		{59 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{89 * time.Second, "1 minute ago"},
		{12 * time.Minute, "12 minutes ago"},
		{time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{48 * time.Hour, "2 days ago"},
	}
	for _, tc := range cases {
		if got := relativeTimeLong(p32Now.Add(-tc.ago), p32Now); got != tc.want {
			t.Errorf("relativeTimeLong(%v ago) = %q, want %q", tc.ago, got, tc.want)
		}
	}
}

// TestUpdateWire maps the closed words through the page tables and omits the
// optional keys when empty.
func TestUpdateWire(t *testing.T) {
	id := int64(9)
	full := updateWire(service.UpdateItem{
		ID: 41, Kind: service.UpdateReachedLive, ReportID: &id, Available: true, Unread: true,
		CreatedAt: p32Now.Add(-5 * time.Minute), Event: "Now confirmed", ReportLine: "Road under water",
		Category: service.CategoryStormCyclone, Severity: service.SeverityMedium,
		Trust: "Confirmed by 2 people at different spots.",
	}, p32Now)
	if full.ID != 41 || full.Kind != "reached_live" || full.ReportID == nil || *full.ReportID != 9 || !full.Available || !full.Unread {
		t.Errorf("wire identity fields = %+v", full)
	}
	if full.Category != "Storm/Cyclone damage" || full.Severity != "Medium" || full.SeverityLevel != "medium" {
		t.Errorf("wire words = %q / %q / %q", full.Category, full.Severity, full.SeverityLevel)
	}
	if full.Event != "Now confirmed" || full.ReportLine != "Road under water" || full.Trust != "Confirmed by 2 people at different spots." || full.When != "5 min ago" {
		t.Errorf("wire lines = %+v", full)
	}
	if want := "New. Now confirmed. Road under water. Storm/Cyclone damage, Medium. Confirmed by 2 people at different spots. 5 minutes ago"; full.Label != want {
		t.Errorf("wire label = %q, want %q", full.Label, want)
	}

	// An unavailable item: no report id, no report line, no category, no more
	// count, and those keys are absent from the JSON.
	gone := updateWire(service.UpdateItem{
		ID: 42, Kind: service.UpdateHidden, CreatedAt: p32Now.Add(-2 * time.Hour), Event: "This report is no longer available",
	}, p32Now)
	raw, err := json.Marshal(gone)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, k := range []string{"report_id", "report_line", "more_count"} {
		if _, ok := keys[k]; ok {
			t.Errorf("an unavailable item carries %q: %s", k, raw)
		}
	}
	if gone.SeverityLevel != "" || gone.Severity != "" || gone.Category != "" {
		t.Errorf("an unavailable item carries words: %+v", gone)
	}

	// The rolled up row keeps its count.
	more := updateWire(service.UpdateItem{
		ID: 43, Kind: service.UpdateMore, Available: true, MoreCount: 4, CreatedAt: p32Now.Add(-time.Hour), Event: "And 4 more changes",
	}, p32Now)
	if more.MoreCount != 4 || more.ReportID != nil {
		t.Errorf("more row = %+v", more)
	}
	if strings.Contains(more.Label, "New.") {
		t.Errorf("a read more row label = %q", more.Label)
	}
}
