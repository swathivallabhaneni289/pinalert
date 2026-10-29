package service_test

// sharecopy_test.go proves the words of Share and of the public status page
// without a database: the title, the relative time, the token shape and the
// closed character set.

import (
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

func TestShareTitle(t *testing.T) {
	cases := []struct {
		category service.Category
		area     string
		want     string
	}{
		{service.CategoryFlood, "Benz Circle", "Flood report near Benz Circle"},
		{service.CategoryFlood, "  Benz Circle  ", "Flood report near Benz Circle"},
		{service.CategoryFlood, "", "Flood report"},
		{service.CategoryFlood, "   ", "Flood report"},
		{service.CategoryStormCyclone, "Machilipatnam", "Storm/Cyclone damage report near Machilipatnam"},
		{service.CategoryShelterOpen, "", "Shelter open report"},
		{service.Category("nonsense"), "", "Report"},
	}
	for _, c := range cases {
		if got := service.ShareTitle(c.category, c.area); got != c.want {
			t.Errorf("ShareTitle(%q, %q) = %q, want %q", c.category, c.area, got, c.want)
		}
	}
	long := strings.Repeat("Long Name ", 10)
	if got := service.ShareTitle(service.CategoryFlood, long); len([]rune(got)) > len("Flood report near ")+service.AreaMaxRunes {
		t.Errorf("a long area is not cut: %q", got)
	}
}

func TestPublicAgo(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{-time.Minute, "just now"},
		{0, "just now"},
		{59 * time.Second, "just now"},
		{60 * time.Second, "1 minute ago"},
		{90 * time.Second, "1 minute ago"},
		{2 * time.Minute, "2 minutes ago"},
		{59 * time.Minute, "59 minutes ago"},
		{time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{23*time.Hour + 59*time.Minute, "23 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{5 * 24 * time.Hour, "5 days ago"},
	}
	for _, c := range cases {
		if got := service.PublicAgo(now.Add(-c.ago), now); got != c.want {
			t.Errorf("PublicAgo(now - %v) = %q, want %q", c.ago, got, c.want)
		}
	}
}

func TestValidShareToken(t *testing.T) {
	good := []string{
		"AbCdEfGhIjKlMnOpQr_-12",
		"aaaaaaaaaaaaaaaaaaaaaa",
		"0000000000000000000000",
		"______________________",
		"----------------------",
	}
	for _, s := range good {
		if !service.ValidShareToken(s) {
			t.Errorf("ValidShareToken(%q) = false, want true", s)
		}
	}
	bad := []string{
		"",
		"abc",
		"12",
		"123456",
		"AbCdEfGhIjKlMnOpQr_-1",   // 21
		"AbCdEfGhIjKlMnOpQr_-123", // 23
		"AbCdEfGhIjKlMnOpQr+-12",
		"AbCdEfGhIjKlMnOpQr/-12",
		"AbCdEfGhIjKlMnOpQr=-12",
		"AbCdEfGhIjKlMnOpQr -12",
		"AbCdEfGhIjKlMnOpQr_-1\n",
		"AbCdEfGhIjKlMnOpQr_-" + string(rune(0xe9)), // 21 characters but 22 bytes
	}
	for _, s := range bad {
		if service.ValidShareToken(s) {
			t.Errorf("ValidShareToken(%q) = true, want false", s)
		}
	}
	for i := 0; i < 50; i++ {
		tok, err := service.NewShareToken()
		if err != nil {
			t.Fatalf("NewShareToken: %v", err)
		}
		if !service.ValidShareToken(tok) {
			t.Fatalf("NewShareToken made %q, which is not a valid token", tok)
		}
	}
}

func TestShareCopyIsPlain(t *testing.T) {
	allowed := func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return true
		}
		return strings.ContainsRune(" ,.'/", r)
	}
	words := map[string]string{
		"ShareFixedLine":         service.ShareFixedLine,
		"PublicChipActive":       service.PublicChipActive,
		"PublicChipDisputed":     service.PublicChipDisputed,
		"PublicChipResolved":     service.PublicChipResolved,
		"PublicChipEnded":        service.PublicChipEnded,
		"PublicRemovedTitle":     service.PublicRemovedTitle,
		"PublicUnavailableTitle": service.PublicUnavailableTitle,
		"PublicTooManyTitle":     service.PublicTooManyTitle,
		"PublicTooManyLine":      service.PublicTooManyLine,
		"PublicEndedSentence":    service.PublicEndedSentence,
		"PublicAgo just now":     service.PublicAgo(time.Now(), time.Now()),
		"PublicAgo minute":       service.PublicAgo(time.Now().Add(-90*time.Second), time.Now()),
		"PublicAgo hours":        service.PublicAgo(time.Now().Add(-3*time.Hour), time.Now()),
		"PublicAgo days":         service.PublicAgo(time.Now().Add(-49*time.Hour), time.Now()),
	}
	for _, c := range service.Categories {
		words["ShareTitle "+string(c)] = service.ShareTitle(c, "Benz Circle")
	}
	for name, w := range words {
		if w == "" {
			t.Errorf("%s is empty", name)
		}
		for _, r := range w {
			if !allowed(r) {
				t.Errorf("%s = %q holds %q, outside the plain character set", name, w, r)
				break
			}
		}
		if strings.Contains(strings.ToLower(w), "verified") {
			t.Errorf("%s = %q uses the word verified", name, w)
		}
	}
	for _, r := range service.ShareFixedLine {
		if r >= '0' && r <= '9' {
			t.Errorf("ShareFixedLine holds a digit: %q", service.ShareFixedLine)
			break
		}
	}
	if service.ShareFixedLine != "Open to see if this is still true right now" {
		t.Errorf("ShareFixedLine = %q", service.ShareFixedLine)
	}
	if got := service.PublicAgo(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(-90*time.Second), time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)); got != "1 minute ago" {
		t.Errorf("PublicAgo(90 seconds) = %q, want 1 minute ago", got)
	}
	if got := service.PublicAgo(time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)); got != "3 hours ago" {
		t.Errorf("PublicAgo(3 hours) = %q, want 3 hours ago", got)
	}
}
