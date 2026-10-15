package service_test

// requestid_copy_test.go holds the 04-17 service tests: the request id message
// (D-19, R-05), its validation and, from the second task, the written time
// clamp. None of them needs a database.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

func TestServerCopyRequestID(t *testing.T) {
	const want = "This report couldn't be sent as it was. Reload the page and post it again."
	if service.RequestIDInvalidMessage != want {
		t.Fatalf("RequestIDInvalidMessage = %q, want %q", service.RequestIDInvalidMessage, want)
	}
	assertServerCopy(t, service.RequestIDInvalidMessage)
}

// p17ValidInput is a submission that passes every check except the ones a
// test changes.
func p17ValidInput() service.SubmitInput {
	return service.SubmitInput{
		Category:    service.CategoryFlood,
		Severity:    service.SeverityLow,
		Description: "Water is rising near the market bridge.",
		Latitude:    12.9716,
		Longitude:   77.5946,
	}
}

func TestValidateSubmitInputRequestID(t *testing.T) {
	accepted := []struct {
		name string
		id   string
	}{
		{"absent", ""},
		{"sixteen characters", "abcdefghij-12345"},
		{"uuid", "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c10"},
		{"sixty four characters", strings.Repeat("a", 64)},
	}
	for _, c := range accepted {
		t.Run("accepts_"+c.name, func(t *testing.T) {
			in := p17ValidInput()
			in.RequestID = c.id
			if err := service.ValidateSubmitInput(in); err != nil {
				t.Fatalf("RequestID %q: unexpected error %v", c.id, err)
			}
		})
	}

	rejected := []struct {
		name string
		id   string
	}{
		{"fifteen characters", "abcdefghij-1234"},
		{"sixty five characters", strings.Repeat("a", 65)},
		{"a space", "abc def ghi jkl mno"},
		{"an underscore", "under_score_id_0000"},
		{"a non ASCII letter", "abcdefghij-1234é"},
		{"a trailing newline", "abcdefghij-12345\n"},
	}
	for _, c := range rejected {
		t.Run("rejects_"+c.name, func(t *testing.T) {
			in := p17ValidInput()
			in.RequestID = c.id
			err := service.ValidateSubmitInput(in)
			var ve service.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("RequestID %q: error %v is not a ValidationError", c.id, err)
			}
			if ve.Field != "request_id" || ve.Message != service.RequestIDInvalidMessage {
				t.Fatalf("RequestID %q: got %+v", c.id, ve)
			}
		})
	}

	t.Run("runs_after_every_other_check", func(t *testing.T) {
		in := p17ValidInput()
		in.Category = ""
		in.RequestID = "short"
		err := service.ValidateSubmitInput(in)
		var ve service.ValidationError
		if !errors.As(err, &ve) || ve.Field != "category" {
			t.Fatalf("got %v, want a category error first", err)
		}
	})
}

func TestClampWrittenAt(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	at := func(v time.Time) *time.Time { return &v }

	if service.WrittenAtMaxAge != 14*24*time.Hour {
		t.Fatalf("WrittenAtMaxAge = %v, want 14 days", service.WrittenAtMaxAge)
	}

	cases := []struct {
		name string
		in   *time.Time
		want *time.Time
	}{
		{"nil stays nil", nil, nil},
		{"a future value is set to now", at(now.Add(time.Hour)), at(now)},
		{"a recent value is kept", at(now.Add(-time.Hour)), at(now.Add(-time.Hour))},
		{"exactly 14 days is kept", at(now.Add(-service.WrittenAtMaxAge)), at(now.Add(-service.WrittenAtMaxAge))},
		{"one second past 14 days is clamped", at(now.Add(-service.WrittenAtMaxAge - time.Second)), at(now.Add(-service.WrittenAtMaxAge))},
		{"20 days is clamped to 14 days", at(now.Add(-20 * 24 * time.Hour)), at(now.Add(-service.WrittenAtMaxAge))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := service.ClampWrittenAt(c.in, now)
			if c.want == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", *got)
				}
				return
			}
			if got == nil || !got.Equal(*c.want) {
				t.Fatalf("got %v, want %v", got, *c.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("location = %v, want UTC", got.Location())
			}
		})
	}

	t.Run("a zone offset gives the same instant in UTC", func(t *testing.T) {
		ist := time.FixedZone("IST", 5*3600+1800)
		in := now.Add(-time.Hour).In(ist)
		got := service.ClampWrittenAt(&in, now)
		if got == nil || !got.Equal(in) || got.Location() != time.UTC {
			t.Fatalf("got %v, want the instant %v in UTC", got, in)
		}
	})

	t.Run("the result is never the input pointer", func(t *testing.T) {
		in := now.Add(-time.Hour)
		got := service.ClampWrittenAt(&in, now)
		if got == &in {
			t.Fatal("returned the input pointer")
		}
		in = now.Add(-2 * time.Hour)
		if !got.Equal(now.Add(-time.Hour)) {
			t.Fatalf("changing the input changed the result: %v", got)
		}
	})
}
