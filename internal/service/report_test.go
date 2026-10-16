package service_test

import (
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

func ptrCapacity(s service.CapacityStatus) *service.CapacityStatus { return &s }
func ptrInt(n int) *int                                            { return &n }

func validInput() service.SubmitInput {
	return service.SubmitInput{
		Category:    service.CategoryFlood,
		Severity:    service.SeverityLow,
		Description: "Main road is under a foot of water",
		Latitude:    12.9716,
		Longitude:   77.5946,
	}
}

func TestValidateSubmitInput(t *testing.T) {
	t.Run("every postable category slug is accepted", func(t *testing.T) {
		for _, c := range service.PostableCategories {
			in := validInput()
			in.Category = c
			if c == service.CategoryShelterOpen {
				// shelter_open additionally requires a capacity status;
				// covered on its own in TestShelterCapacityValidation.
				in.ShelterCapacityStatus = ptrCapacity(service.CapacityAvailable)
			}
			if err := service.ValidateSubmitInput(in); err != nil {
				t.Errorf("category %q: expected valid, got error %v", c, err)
			}
		}
	})

	t.Run("rescue_needed is refused at every severity with the retired message", func(t *testing.T) {
		for _, s := range service.Severities {
			in := validInput()
			in.Category = service.CategoryRescueNeeded
			in.Severity = s
			err := service.ValidateSubmitInput(in)
			assertFieldError(t, err, "category")
			if got := err.(service.ValidationError).Message; got != service.RescueNeededRetiredMessage {
				t.Errorf("severity %q: message = %q, want %q", s, got, service.RescueNeededRetiredMessage)
			}
		}
	})

	t.Run("every known category that is not postable is refused with the retired message", func(t *testing.T) {
		refused := 0
		for _, c := range service.Categories {
			if c.Postable() {
				continue
			}
			refused++
			in := validInput()
			in.Category = c
			err := service.ValidateSubmitInput(in)
			assertFieldError(t, err, "category")
			if got := err.(service.ValidationError).Message; got != service.RescueNeededRetiredMessage {
				t.Errorf("category %q: message = %q, want %q", c, got, service.RescueNeededRetiredMessage)
			}
		}
		if refused == 0 {
			t.Error("no known category is refused, so the retired path is not exercised")
		}
	})

	t.Run("an unknown category is rejected naming the category field", func(t *testing.T) {
		in := validInput()
		in.Category = "not_a_real_category"
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "category")
		if got := err.(service.ValidationError).Message; got != "Choose a category to continue." {
			t.Errorf("unknown category message = %q, want %q", got, "Choose a category to continue.")
		}
	})

	t.Run("every severity value is accepted", func(t *testing.T) {
		for _, s := range service.Severities {
			in := validInput()
			in.Severity = s
			if err := service.ValidateSubmitInput(in); err != nil {
				t.Errorf("severity %q: expected valid, got error %v", s, err)
			}
		}
	})

	t.Run("an unknown severity is rejected naming the severity field", func(t *testing.T) {
		in := validInput()
		in.Severity = "urgent"
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "severity")
	})

	t.Run("severity comparison is exact, no case-insensitive coercion", func(t *testing.T) {
		in := validInput()
		in.Severity = "LOW"
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "severity")
	})

	t.Run("a description shorter than 10 characters after trimming is rejected", func(t *testing.T) {
		in := validInput()
		in.Description = "   short  "
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "description")
	})

	t.Run("a description longer than 1000 characters is rejected", func(t *testing.T) {
		in := validInput()
		in.Description = strings.Repeat("a", 1001)
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "description")
	})

	t.Run("a description of exactly 1000 characters is accepted", func(t *testing.T) {
		in := validInput()
		in.Description = strings.Repeat("a", 1000)
		if err := service.ValidateSubmitInput(in); err != nil {
			t.Errorf("expected 1000-char description to be valid, got %v", err)
		}
	})

	t.Run("latitude outside -90..90 is rejected naming the latitude field", func(t *testing.T) {
		for _, lat := range []float64{-90.1, 90.1, 200, -200} {
			in := validInput()
			in.Latitude = lat
			err := service.ValidateSubmitInput(in)
			assertFieldError(t, err, "latitude")
		}
	})

	t.Run("longitude outside -180..180 is rejected naming the longitude field", func(t *testing.T) {
		for _, lon := range []float64{-180.1, 180.1, 400, -400} {
			in := validInput()
			in.Longitude = lon
			err := service.ValidateSubmitInput(in)
			assertFieldError(t, err, "longitude")
		}
	})

	t.Run("boundary latitude/longitude values are accepted", func(t *testing.T) {
		in := validInput()
		in.Latitude = 90
		in.Longitude = 180
		if err := service.ValidateSubmitInput(in); err != nil {
			t.Errorf("expected boundary lat/lon to be valid, got %v", err)
		}
	})
}

func TestShelterCapacityValidation(t *testing.T) {
	t.Run("shelter_open with no capacity status is rejected", func(t *testing.T) {
		in := validInput()
		in.Category = service.CategoryShelterOpen
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "shelter_capacity_status")
	})

	t.Run("shelter_open with an invalid capacity status is rejected", func(t *testing.T) {
		in := validInput()
		in.Category = service.CategoryShelterOpen
		bad := service.CapacityStatus("overflowing")
		in.ShelterCapacityStatus = &bad
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "shelter_capacity_status")
	})

	t.Run("shelter_open with a valid capacity status is accepted", func(t *testing.T) {
		for _, cs := range service.CapacityStatuses {
			in := validInput()
			in.Category = service.CategoryShelterOpen
			in.ShelterCapacityStatus = ptrCapacity(cs)
			if err := service.ValidateSubmitInput(in); err != nil {
				t.Errorf("capacity status %q: expected valid, got error %v", cs, err)
			}
		}
	})

	t.Run("a capacity status supplied for a non-shelter category is rejected", func(t *testing.T) {
		in := validInput()
		in.Category = service.CategoryFlood
		in.ShelterCapacityStatus = ptrCapacity(service.CapacityAvailable)
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "shelter_capacity_status")
	})

	t.Run("a headcount supplied for a non-shelter category is rejected", func(t *testing.T) {
		in := validInput()
		in.Category = service.CategoryFlood
		in.ShelterHeadcount = ptrInt(10)
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "shelter_headcount")
	})

	t.Run("a negative headcount is rejected", func(t *testing.T) {
		in := validInput()
		in.Category = service.CategoryShelterOpen
		in.ShelterCapacityStatus = ptrCapacity(service.CapacityAvailable)
		in.ShelterHeadcount = ptrInt(-1)
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "shelter_headcount")
	})

	t.Run("an absurdly large headcount is rejected", func(t *testing.T) {
		in := validInput()
		in.Category = service.CategoryShelterOpen
		in.ShelterCapacityStatus = ptrCapacity(service.CapacityAvailable)
		in.ShelterHeadcount = ptrInt(10_000_000)
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "shelter_headcount")
	})

	t.Run("a reasonable headcount on a shelter_open report is accepted", func(t *testing.T) {
		in := validInput()
		in.Category = service.CategoryShelterOpen
		in.ShelterCapacityStatus = ptrCapacity(service.CapacityLimited)
		in.ShelterHeadcount = ptrInt(42)
		if err := service.ValidateSubmitInput(in); err != nil {
			t.Errorf("expected valid, got error %v", err)
		}
	})
}

func TestExpiryDuration(t *testing.T) {
	cases := []struct {
		severity service.Severity
		want     time.Duration
	}{
		{service.SeverityCritical, 24 * time.Hour},
		{service.SeverityLow, 8 * time.Hour},
		{service.SeverityMedium, 8 * time.Hour},
	}
	for _, c := range cases {
		if got := service.ExpiryDuration(c.severity); got != c.want {
			t.Errorf("ExpiryDuration(%q) = %v, want %v", c.severity, got, c.want)
		}
	}
}

func TestBoundingBox(t *testing.T) {
	t.Run("returns a symmetric box around the query point", func(t *testing.T) {
		latMin, latMax, lonMin, lonMax := service.BoundingBox(12.9716, 77.5946, 10)
		if latMin >= 12.9716 || latMax <= 12.9716 {
			t.Errorf("expected latitude bounds to straddle the query point, got [%v, %v]", latMin, latMax)
		}
		if lonMin >= 77.5946 || lonMax <= 77.5946 {
			t.Errorf("expected longitude bounds to straddle the query point, got [%v, %v]", lonMin, lonMax)
		}
	})

	t.Run("clamps latitude to +/-90", func(t *testing.T) {
		latMin, latMax, _, _ := service.BoundingBox(89.9, 0, 50)
		if latMax > 90 {
			t.Errorf("expected latMax clamped to 90, got %v", latMax)
		}
		latMin2, _, _, _ := service.BoundingBox(-89.9, 0, 50)
		if latMin2 < -90 {
			t.Errorf("expected latMin clamped to -90, got %v", latMin2)
		}
		_ = latMin
	})

	t.Run("falls back to the full longitude range near the poles", func(t *testing.T) {
		_, _, lonMin, lonMax := service.BoundingBox(89.999, 0, 50)
		if lonMin != -180 || lonMax != 180 {
			t.Errorf("expected full longitude range near the pole, got [%v, %v]", lonMin, lonMax)
		}
	})
}

// TestCategoryLists pins the two category lists. Categories is every category
// a stored report may carry; PostableCategories is what a new report may use.
// rescue_needed was retired for new reports on 2026-10-16 by owner decision
// and traffic took its place, so stored rescue_needed rows are still valid.
func TestCategoryLists(t *testing.T) {
	known := []service.Category{
		"flood", "earthquake", "fire", "storm_cyclone", "road_blocked",
		"power_outage", "shelter_open", "traffic", "rescue_needed", "other",
	}
	postable := []service.Category{
		"flood", "earthquake", "fire", "storm_cyclone", "road_blocked",
		"power_outage", "shelter_open", "traffic", "other",
	}
	if len(service.Categories) != len(known) {
		t.Fatalf("Categories = %v, want %v", service.Categories, known)
	}
	for i, c := range known {
		if service.Categories[i] != c {
			t.Errorf("Categories[%d] = %q, want %q", i, service.Categories[i], c)
		}
	}
	if len(service.PostableCategories) != len(postable) {
		t.Fatalf("PostableCategories = %v, want %v", service.PostableCategories, postable)
	}
	for i, c := range postable {
		if service.PostableCategories[i] != c {
			t.Errorf("PostableCategories[%d] = %q, want %q", i, service.PostableCategories[i], c)
		}
	}

	for _, c := range []service.Category{service.CategoryRescueNeeded, service.CategoryTraffic} {
		if !c.Valid() {
			t.Errorf("%q.Valid() = false, want true", c)
		}
	}
	if service.CategoryRescueNeeded.Postable() {
		t.Error("rescue_needed is postable, want retired for new reports")
	}
	if !service.CategoryTraffic.Postable() {
		t.Error("traffic is not postable")
	}
	if service.Category("not_a_real_category").Postable() {
		t.Error("an unknown slug is postable")
	}

	// PostableCategories is Categories with exactly rescue_needed removed.
	var withoutRescue []service.Category
	for _, c := range service.Categories {
		if c != service.CategoryRescueNeeded {
			withoutRescue = append(withoutRescue, c)
		}
	}
	if len(withoutRescue) != len(service.PostableCategories) {
		t.Fatalf("Categories without rescue_needed = %v, PostableCategories = %v", withoutRescue, service.PostableCategories)
	}
	for i := range withoutRescue {
		if withoutRescue[i] != service.PostableCategories[i] {
			t.Errorf("position %d: %q vs %q", i, withoutRescue[i], service.PostableCategories[i])
		}
	}
}

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a validation error naming field %q, got nil", field)
	}
	ve, ok := err.(service.ValidationError)
	if !ok {
		t.Fatalf("expected a service.ValidationError, got %T: %v", err, err)
	}
	if ve.Field != field {
		t.Fatalf("expected error to name field %q, got %q (%v)", field, ve.Field, ve)
	}
}

// TestValidateSubmitInputCountsRunes is V-07 (R-06): the description limits are
// counted in characters after trimming, not in bytes. A Telugu letter takes 3
// bytes in UTF-8, so a byte count would accept four letters and refuse 400.
func TestValidateSubmitInputCountsRunes(t *testing.T) {
	const (
		shortMsg = "Add a short description (at least 10 characters)."
		longMsg  = "Description must be 1000 characters or fewer."
	)
	scripts := []struct {
		name   string
		letter string
	}{
		{"english", "a"},
		{"telugu", "న"},
	}
	check := func(t *testing.T, desc, wantMsg string) {
		t.Helper()
		in := validInput()
		in.Description = desc
		err := service.ValidateSubmitInput(in)
		if wantMsg == "" {
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			return
		}
		assertFieldError(t, err, "description")
		if ve := err.(service.ValidationError); ve.Message != wantMsg {
			t.Fatalf("message %q, want %q", ve.Message, wantMsg)
		}
	}
	for _, sc := range scripts {
		t.Run(sc.name, func(t *testing.T) {
			check(t, strings.Repeat(sc.letter, 9), shortMsg)
			check(t, strings.Repeat(sc.letter, 10), "")
			check(t, strings.Repeat(sc.letter, 1000), "")
			check(t, strings.Repeat(sc.letter, 1001), longMsg)
		})
	}
	t.Run("four telugu letters are 12 bytes and still too short", func(t *testing.T) {
		check(t, strings.Repeat("న", 4), shortMsg)
	})
	t.Run("ten telugu letters padded with spaces and a newline pass", func(t *testing.T) {
		check(t, "  \n"+strings.Repeat("న", 10)+" \n ", "")
	})
}

func ptrHappening(h service.Happening) *service.Happening { return &h }

// TestValidateSubmitHappening pins the What is happening pick (D-24, D-24a):
// one of seven values, accepted only with severity critical, optional always.
func TestValidateSubmitHappening(t *testing.T) {
	want := []service.Happening{
		service.HappeningWaterRising, service.HappeningPeopleTrapped, service.HappeningFire,
		service.HappeningBuildingCollapsed, service.HappeningLandslide, service.HappeningMedicalEmergency,
		service.HappeningOther,
	}
	if len(service.Happenings) != 7 {
		t.Fatalf("Happenings has %d values, want 7", len(service.Happenings))
	}
	for i, h := range want {
		if service.Happenings[i] != h {
			t.Errorf("Happenings[%d] = %q, want %q", i, service.Happenings[i], h)
		}
	}
	slugs := []string{"water_rising", "people_trapped", "fire", "building_collapsed", "landslide", "medical_emergency", "other"}
	for i, s := range slugs {
		if string(want[i]) != s {
			t.Errorf("constant %d is %q, want %q", i, want[i], s)
		}
	}

	t.Run("each of the seven passes with critical", func(t *testing.T) {
		for _, h := range service.Happenings {
			in := validInput()
			in.Severity = service.SeverityCritical
			in.Happening = ptrHappening(h)
			if err := service.ValidateSubmitInput(in); err != nil {
				t.Errorf("happening %q with critical: %v", h, err)
			}
		}
	})
	t.Run("any value with low or medium is refused on happening", func(t *testing.T) {
		for _, sev := range []service.Severity{service.SeverityLow, service.SeverityMedium} {
			for _, h := range service.Happenings {
				in := validInput()
				in.Severity = sev
				in.Happening = ptrHappening(h)
				assertFieldError(t, service.ValidateSubmitInput(in), "happening")
			}
		}
	})
	t.Run("an unknown, wrong case or empty value is refused on happening", func(t *testing.T) {
		for _, bad := range []service.Happening{"flood", "Fire", "FIRE", "people trapped", "", " fire"} {
			in := validInput()
			in.Severity = service.SeverityCritical
			in.Happening = ptrHappening(bad)
			assertFieldError(t, service.ValidateSubmitInput(in), "happening")
		}
	})
	t.Run("no happening passes for every severity", func(t *testing.T) {
		for _, sev := range service.Severities {
			in := validInput()
			in.Severity = sev
			if err := service.ValidateSubmitInput(in); err != nil {
				t.Errorf("severity %q without happening: %v", sev, err)
			}
		}
	})
	t.Run("Valid is an exact match", func(t *testing.T) {
		if !service.HappeningFire.Valid() || service.Happening("Fire").Valid() || service.Happening("").Valid() {
			t.Error("Valid must accept the seven slugs and nothing else")
		}
	})
}
