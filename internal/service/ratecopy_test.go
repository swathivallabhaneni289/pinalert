package service_test

// ratecopy_test.go is the TestServerCopy family member for the server
// composed rate limit messages and the wait phrase they share (04-PLAN-OUTLINE
// "Server authored copy", plan 04-09). It reuses assertServerCopy from
// servercopy_test.go and declares no fake.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

func TestWaitPhrase(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "1 second"},
		{"negative", -5 * time.Second, "1 second"},
		{"one nanosecond", time.Nanosecond, "1 second"},
		{"one second", time.Second, "1 second"},
		{"one and a half seconds", 1500 * time.Millisecond, "2 seconds"},
		{"fifty nine seconds", 59 * time.Second, "59 seconds"},
		{"fifty nine and a half seconds", 59*time.Second + 500*time.Millisecond, "1 minute"},
		{"sixty seconds", 60 * time.Second, "1 minute"},
		{"sixty one seconds", 61 * time.Second, "2 minutes"},
		{"five minutes", 5 * time.Minute, "5 minutes"},
		{"fifteen minutes", 15 * time.Minute, "15 minutes"},
		{"3599 seconds", 3599 * time.Second, "60 minutes"},
		{"3600 seconds", 3600 * time.Second, "1 hour"},
		{"3601 seconds", 3601 * time.Second, "2 hours"},
		{"two hours", 2 * time.Hour, "2 hours"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.WaitPhrase(tc.d); got != tc.want {
				t.Errorf("WaitPhrase(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

func TestServerCopyRateLimits(t *testing.T) {
	exact := []struct {
		name string
		got  string
		want string
	}{
		{"report", service.ReportLimitMessage(false, 15*time.Minute), "You've posted several reports in a short time. You can post again in 15 minutes. Your text is still here."},
		{"serious report", service.ReportLimitMessage(true, 5*time.Minute), "You've posted several serious reports in a short time. You can post again in 5 minutes. Your text is still here."},
		{"vote", service.VoteLimitMessage(15 * time.Second), "You're voting very quickly. You can vote again in 15 seconds."},
		{"vote change", service.VoteChangeLimitMessage(30 * time.Second), "You changed this vote a moment ago. You can change it again in 30 seconds."},
		{"flag", service.FlagLimitMessage(2 * time.Minute), "You've flagged several reports in a short time. You can flag another in 2 minutes."},
		{"network", service.NetworkLimitMessage(false, 15*time.Second), "A lot of requests are coming from your network right now. You can try again in 15 seconds."},
		{"network report", service.NetworkLimitMessage(true, time.Minute), "A lot of requests are coming from your network right now. You can try again in 1 minute. Your text is still here."},
	}
	for _, tc := range exact {
		t.Run("exact "+tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("message = %q, want %q", tc.got, tc.want)
			}
		})
	}

	forms := []struct {
		name string
		fn   func(time.Duration) string
	}{
		{"report", func(w time.Duration) string { return service.ReportLimitMessage(false, w) }},
		{"serious report", func(w time.Duration) string { return service.ReportLimitMessage(true, w) }},
		{"vote", service.VoteLimitMessage},
		{"vote change", service.VoteChangeLimitMessage},
		{"flag", service.FlagLimitMessage},
		{"network", func(w time.Duration) string { return service.NetworkLimitMessage(false, w) }},
		{"network report", func(w time.Duration) string { return service.NetworkLimitMessage(true, w) }},
	}
	waits := []time.Duration{time.Second, 15 * time.Second, 30 * time.Second, 5 * time.Minute, 15 * time.Minute, 2 * time.Hour}
	for _, f := range forms {
		for _, w := range waits {
			t.Run(fmt.Sprintf("%s at %v", f.name, w), func(t *testing.T) {
				msg := f.fn(w)
				assertServerCopy(t, msg)
				phrase := service.WaitPhrase(w)
				if n := strings.Count(msg, phrase); n != 1 {
					t.Errorf("message %q holds the wait phrase %q %d times, want once", msg, phrase, n)
				}
				lower := strings.ToLower(msg)
				if strings.Contains(msg, "112") {
					t.Errorf("message %q must not hold the emergency number", msg)
				}
				if strings.Contains(lower, "emergency") {
					t.Errorf("message %q must not mention an emergency", msg)
				}
				if strings.Contains(lower, "too many") {
					t.Errorf("message %q must not say too many", msg)
				}
			})
		}
	}

	t.Run("serious differs from other", func(t *testing.T) {
		for _, w := range waits {
			if service.ReportLimitMessage(true, w) == service.ReportLimitMessage(false, w) {
				t.Errorf("serious and other report messages are equal at %v", w)
			}
		}
	})
}

// TestShareLimitMessage pins the Share window's wait sentence (Phase 8 plan 03,
// A-P8-4): the exact words for one minute and for twenty seconds, one wait
// phrase, and no dash.
func TestShareLimitMessage(t *testing.T) {
	cases := []struct {
		wait time.Duration
		want string
	}{
		{time.Minute, "You've shared several reports in a short time. You can share again in 1 minute."},
		{20 * time.Second, "You've shared several reports in a short time. You can share again in 20 seconds."},
	}
	for _, tc := range cases {
		got := service.ShareLimitMessage(tc.wait)
		if got != tc.want {
			t.Errorf("ShareLimitMessage(%v) = %q, want %q", tc.wait, got, tc.want)
		}
		assertServerCopy(t, got)
		if strings.ContainsRune(got, rune(0x2013)) || strings.ContainsRune(got, rune(0x2014)) {
			t.Errorf("message %q holds a dash", got)
		}
	}
}

// TestCommentLimitMessages pins the comment and like wait sentences (Phase 8
// plan 12, D-13): the exact words for one minute and for twenty seconds, one
// wait phrase each, and no dash.
func TestCommentLimitMessages(t *testing.T) {
	cases := []struct {
		name string
		fn   func(time.Duration) string
		wait time.Duration
		want string
	}{
		{"comment minute", service.CommentLimitMessage, time.Minute, "You're commenting very quickly. You can comment again in 1 minute. Your words are still here."},
		{"comment seconds", service.CommentLimitMessage, 20 * time.Second, "You're commenting very quickly. You can comment again in 20 seconds. Your words are still here."},
		{"like minute", service.LikeLimitMessage, time.Minute, "You're reacting very quickly. You can try again in 1 minute."},
		{"like seconds", service.LikeLimitMessage, 20 * time.Second, "You're reacting very quickly. You can try again in 20 seconds."},
	}
	for _, tc := range cases {
		got := tc.fn(tc.wait)
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
		}
		assertServerCopy(t, got)
		if strings.ContainsRune(got, rune(0x2013)) || strings.ContainsRune(got, rune(0x2014)) {
			t.Errorf("message %q holds a dash", got)
		}
	}
}
