package service_test

// requestid_copy_test.go holds the 04-17 service tests: the request id message
// (D-19, R-05), its validation and, from the second task, the written time
// clamp. None of them needs a database.

import (
	"errors"
	"strings"
	"testing"

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
