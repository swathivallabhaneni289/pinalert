package web

import (
	"bytes"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

// TestTrafficReplacesRescueInTheForm pins the 2026-10-16 owner decision in the
// browser: the report form offers nine tiles with Traffic in the old Rescue
// needed slot, while a report stored as rescue_needed keeps its label, its icon
// and its chip. The form list and the retired list never share a slug.
func TestTrafficReplacesRescueInTheForm(t *testing.T) {
	appBytes, err := fs.ReadFile(StaticFS, "static/js/app.js")
	if err != nil {
		t.Fatalf("static/js/app.js: %v", err)
	}
	app := string(appBytes)

	wantForm := []string{
		"flood", "earthquake", "fire", "storm_cyclone", "road_blocked",
		"power_outage", "shelter_open", "traffic", "other",
	}
	if got := extractCategoryList(t, app, "var CATEGORIES = ["); !reflect.DeepEqual(got, wantForm) {
		t.Errorf("CATEGORIES = %v, want %v", got, wantForm)
	}
	wantRetired := []string{"rescue_needed"}
	if got := extractCategoryList(t, app, "var RETIRED_CATEGORIES = ["); !reflect.DeepEqual(got, wantRetired) {
		t.Errorf("RETIRED_CATEGORIES = %v, want %v", got, wantRetired)
	}

	for _, want := range []string{
		"var KNOWN_CATEGORIES = CATEGORIES.concat(RETIRED_CATEGORIES);",
		"KNOWN_CATEGORIES: KNOWN_CATEGORIES",
		"traffic: 'Traffic',",
		"rescue_needed: 'Rescue needed',",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("static/js/app.js lacks %q", want)
		}
	}

	traffic, err := fs.ReadFile(StaticFS, "static/icons/traffic.svg")
	if err != nil {
		t.Fatalf("static/icons/traffic.svg: %v", err)
	}
	road, err := fs.ReadFile(StaticFS, "static/icons/road_blocked.svg")
	if err != nil {
		t.Fatalf("static/icons/road_blocked.svg: %v", err)
	}
	licenseLine := func(b []byte) string {
		line, _, _ := strings.Cut(string(b), "\n")
		return line
	}
	if licenseLine(traffic) != licenseLine(road) {
		t.Errorf("traffic.svg license line = %q, want %q", licenseLine(traffic), licenseLine(road))
	}
	for _, want := range []string{
		`viewBox="0 0 24 24"`, `stroke-width="2"`, `aria-hidden="true"`, `focusable="false"`, "lucide-car-front",
	} {
		if !strings.Contains(string(traffic), want) {
			t.Errorf("traffic.svg lacks %q", want)
		}
	}
	if bytes.Equal(traffic, road) {
		t.Error("traffic.svg is byte equal to road_blocked.svg")
	}
	if _, err := fs.Stat(StaticFS, "static/icons/rescue_needed.svg"); err != nil {
		t.Errorf("static/icons/rescue_needed.svg must stay for stored rescue reports: %v", err)
	}

	form, err := fs.ReadFile(StaticFS, "static/js/report-form.js")
	if err != nil {
		t.Fatalf("static/js/report-form.js: %v", err)
	}
	if !strings.Contains(string(form), "Pinalert.CATEGORIES.forEach") {
		t.Error("static/js/report-form.js no longer builds its grid from Pinalert.CATEGORIES.forEach")
	}
}
