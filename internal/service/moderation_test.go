package service_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"pinalert/internal/service"
)

// TestParseModeratorEmails pins the rules of the MODERATOR_EMAILS list parser
// (D-08, T-04-07): split on commas, trim, skip empty items, normalise with the
// sign in normaliser, dedupe, and name a bad entry by position only.
func TestParseModeratorEmails(t *testing.T) {
	ok := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string", "", nil},
		{"only separators and spaces", "  ,  ", nil},
		{"mixed case is lowered", "Mod@Example.COM", []string{"mod@example.com"}},
		{"spaces around items", "  a@example.com , b@example.com  ", []string{"a@example.com", "b@example.com"}},
		{"empty items are skipped", "a@example.com,,b@example.com,", []string{"a@example.com", "b@example.com"}},
		{"duplicates collapse after normalising", "a@example.com,A@EXAMPLE.com", []string{"a@example.com"}},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.ParseModeratorEmails(tc.in)
			if err != nil {
				t.Fatalf("ParseModeratorEmails(%q) error = %v, want nil", tc.in, err)
			}
			if got == nil {
				t.Fatalf("ParseModeratorEmails(%q) = nil map, want a non nil map", tc.in)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ParseModeratorEmails(%q) has %d entries, want %d", tc.in, len(got), len(tc.want))
			}
			for _, w := range tc.want {
				if _, in := got[w]; !in {
					t.Errorf("ParseModeratorEmails(%q) is missing %q", tc.in, w)
				}
			}
		})
	}

	bad := []struct {
		name     string
		in       string
		position string
		secrets  []string
	}{
		{"display name form", "a@example.com,Someone <b@example.com>", "2", []string{"b@example.com", "Someone"}},
		{"not an address", "a@example.com,not an address", "2", []string{"not an address"}},
		{"position counts empty items", ",x", "2", []string{"x@"}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.ParseModeratorEmails(tc.in)
			if err == nil {
				t.Fatalf("ParseModeratorEmails(%q) error = nil, want an error", tc.in)
			}
			if got != nil {
				t.Errorf("ParseModeratorEmails(%q) map = %v, want nil on error", tc.in, got)
			}
			if !strings.Contains(err.Error(), tc.position) {
				t.Errorf("error %q does not name position %s", err.Error(), tc.position)
			}
			for _, s := range tc.secrets {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q leaks the entry text %q", err.Error(), s)
				}
			}
		})
	}
}

// TestNormalizeEmailMatchesSignIn pins that the exported wrapper returns what
// the sign in normaliser returns, so one person is one identity in both places.
func TestNormalizeEmailMatchesSignIn(t *testing.T) {
	got, err := service.NormalizeEmail(" Mod@Example.COM ")
	if err != nil || got != "mod@example.com" {
		t.Fatalf("NormalizeEmail(padded mixed case) = %q, %v, want mod@example.com, nil", got, err)
	}

	got, err = service.NormalizeEmail("Someone <a@example.com>")
	if got != "" {
		t.Errorf("NormalizeEmail(display name) = %q, want empty", got)
	}
	var ve service.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("NormalizeEmail(display name) error = %v, want a ValidationError", err)
	}
	want := service.ValidationError{Field: "email", Message: "Enter a valid email address."}
	if !reflect.DeepEqual(ve, want) {
		t.Errorf("NormalizeEmail(display name) error = %#v, want %#v", ve, want)
	}
}
