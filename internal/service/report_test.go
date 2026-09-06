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
	t.Run("every category slug is accepted", func(t *testing.T) {
		for _, c := range service.Categories {
			in := validInput()
			in.Category = c
			if err := service.ValidateSubmitInput(in); err != nil {
				t.Errorf("category %q: expected valid, got error %v", c, err)
			}
		}
	})

	t.Run("an unknown category is rejected naming the category field", func(t *testing.T) {
		in := validInput()
		in.Category = "not_a_real_category"
		err := service.ValidateSubmitInput(in)
		assertFieldError(t, err, "category")
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
