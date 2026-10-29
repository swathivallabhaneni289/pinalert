package service

// profile_race_comment_test.go pins SK-I01 of the Phase 9 skeptic round
// (2026-10-30): the comments on the lost Name race (WR-S04) say what really
// happens. When the Name loses the race, the Name is not saved, but the username
// (written first) and the home area (the same UPDATE) of the request are, and
// the person is told to try again. No database.

import (
	"os"
	"strings"
	"testing"
)

func TestLostNameRaceCommentsAreTrue(t *testing.T) {
	for _, path := range []string{
		"../store/queries/accounts.sql",
		"../store/sqlc/accounts.sql.go",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		src := plainWords(string(raw))
		if strings.Contains(src, "A Name write that loses this race changes nothing") {
			t.Errorf("%s still says a lost Name race changes nothing", path)
		}
		if !strings.Contains(src, "A Name write that loses this race leaves the Name as it was, but the home area of the same statement is still written") {
			t.Errorf("%s does not say what a lost Name race really does", path)
		}
	}
	raw, err := os.ReadFile("profile.go")
	if err != nil {
		t.Fatalf("reading profile.go: %v", err)
	}
	src := plainWords(string(raw))
	if strings.Contains(src, "so a refusal changes nothing") {
		t.Error("profile.go still says every refusal changes nothing")
	}
	for _, want := range []string{
		"a refusal by the screen changes nothing",
		"Name that loses the race with the switch is the exception",
		"the username written first and the home area of the same statement have already landed",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("profile.go lacks %q", want)
		}
	}
}

// plainWords joins the words of a source file with single spaces and drops the
// comment markers, so a sentence that wraps over comment lines reads as one.
func plainWords(s string) string {
	var words []string
	for _, w := range strings.Fields(s) {
		if w == "//" || w == "--" {
			continue
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}
