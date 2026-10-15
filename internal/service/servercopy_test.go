package service_test

// servercopy_test.go holds the one shared closed character set checker for
// every Phase 4 server authored message (04-PLAN-OUTLINE "Server authored
// copy"). It is used by the TestServerCopy family in this package: 04-09
// ratecopy_test.go, 04-12 moderationcopy_test.go, 04-17
// requestid_copy_test.go, 04-20 flag_test.go and 04-23 moderation_test.go.
// A test that needs an unexported symbol goes in its own package service file
// without this helper.

import (
	"fmt"
	"strings"
	"testing"

	"pinalert/internal/service"
)

// serverCopyProblem returns "" when s is non empty and every rune is an ASCII
// letter, an ASCII digit, a space, or one of comma, period, apostrophe,
// question mark, colon, open parenthesis, close parenthesis and equals.
// Otherwise it describes the first problem.
func serverCopyProblem(s string) string {
	if s == "" {
		return "empty message"
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune(" ,.'?:()=", r):
		default:
			return fmt.Sprintf("character %q (U+%04X) at byte %d is outside the closed set", r, r, i)
		}
	}
	return ""
}

// assertServerCopy fails the test for every problem in the server message s.
func assertServerCopy(t *testing.T, s string) {
	t.Helper()
	if problem := serverCopyProblem(s); problem != "" {
		t.Errorf("server copy %q: %s", s, problem)
	}
}

func TestServerCopyCheckerRejectsOutsideCharacters(t *testing.T) {
	bad := []struct {
		name string
		in   string
	}{
		{"the empty string", ""},
		{"a long dash", "a\u2014b"},
		{"a short dash", "a\u2013b"},
		{"a curly apostrophe", "it\u2019s"},
		{"curly double quotes", "\u201cquoted\u201d"},
		{"an ellipsis", "wait\u2026"},
		{"an ASCII hyphen", "a-b"},
		{"an exclamation mark", "Help!"},
		{"a tab", "tab\there"},
		{"a newline", "line\nbreak"},
		{"a no break space", "non\u00a0breaking"},
		{"a non ASCII letter", "\u0c05"},
	}
	for _, tc := range bad {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			if serverCopyProblem(tc.in) == "" {
				t.Errorf("serverCopyProblem(%q) = empty, want a problem", tc.in)
			}
		})
	}

	good := []struct {
		name string
		in   string
	}{
		{"every allowed punctuation character once", "Yes, it's 5 (five): ok? a = b."},
		{"a plain sentence", "You can post again in 15 minutes."},
	}
	for _, tc := range good {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			if p := serverCopyProblem(tc.in); p != "" {
				t.Errorf("serverCopyProblem(%q) = %q, want empty", tc.in, p)
			}
		})
	}
}

func TestRejectMessages(t *testing.T) {
	exact := []struct {
		reason service.RejectReason
		want   string
	}{
		{service.RejectHateful, "This text looks hateful, so it can't be posted. Reword it and try again."},
		{service.RejectHarassing, "This text looks like harassment, so it can't be posted. Reword it and try again."},
		{service.RejectSexual, "This text looks sexual, so it can't be posted. Reword it and try again."},
		{service.RejectGibberish, "Say what is happening so people nearby can help, for example: flood on Main Road, three people on a roof."},
	}
	for _, tc := range exact {
		t.Run(string(tc.reason)+" message is exact", func(t *testing.T) {
			if got := service.RejectMessage(tc.reason); got != tc.want {
				t.Errorf("RejectMessage(%q) = %q, want %q", tc.reason, got, tc.want)
			}
		})
	}

	for _, in := range []service.RejectReason{"", "spam", "Hateful"} {
		t.Run(fmt.Sprintf("no catch all message for %q", in), func(t *testing.T) {
			if got := service.RejectMessage(in); got != "" {
				t.Errorf("RejectMessage(%q) = %q, want empty", in, got)
			}
		})
	}

	t.Run("messages are non empty, distinct, plain and generic", func(t *testing.T) {
		seen := map[string]service.RejectReason{}
		for _, r := range service.RejectReasons {
			m := service.RejectMessage(r)
			if m == "" {
				t.Errorf("RejectMessage(%q) is empty", r)
				continue
			}
			if other, dup := seen[m]; dup {
				t.Errorf("RejectMessage(%q) repeats the message of %q", r, other)
			}
			seen[m] = r
			if r == service.RejectGibberish {
				if !strings.HasPrefix(m, "Say what is happening") {
					t.Errorf("gibberish message %q must begin with the owner wording", m)
				}
			} else if !strings.HasSuffix(m, "Reword it and try again.") {
				t.Errorf("message %q must end with the reword sentence", m)
			}
			lower := strings.ToLower(m)
			for _, banned := range []string{"report", "score"} {
				if strings.Contains(lower, banned) {
					t.Errorf("message %q must not contain %q", m, banned)
				}
			}
			if strings.ContainsAny(m, "0123456789") {
				t.Errorf("message %q must not contain a digit", m)
			}
		}
	})
}

func TestServerCopyRejections(t *testing.T) {
	for _, r := range service.RejectReasons {
		t.Run(string(r), func(t *testing.T) {
			assertServerCopy(t, service.RejectMessage(r))
		})
	}
}
