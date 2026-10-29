package service

import (
	"errors"
	"strings"
	"testing"
)

func TestOnboardingDone(t *testing.T) {
	full := OnboardingState{
		TermsVersion: CurrentTermsVersion,
		FirstRunDone: true,
		HasUsername:  true,
		HasName:      true,
		HasHomeArea:  true,
	}
	cases := []struct {
		name string
		mut  func(s *OnboardingState)
		want bool
	}{
		{"everything present", func(s *OnboardingState) {}, true},
		{"older terms version", func(s *OnboardingState) { s.TermsVersion = "2020-01-01" }, false},
		{"no terms version", func(s *OnboardingState) { s.TermsVersion = "" }, false},
		{"first run not done", func(s *OnboardingState) { s.FirstRunDone = false }, false},
		{"no username", func(s *OnboardingState) { s.HasUsername = false }, false},
		{"no name", func(s *OnboardingState) { s.HasName = false }, false},
		{"no home area", func(s *OnboardingState) { s.HasHomeArea = false }, false},
		{"nothing at all", func(s *OnboardingState) { *s = OnboardingState{} }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := full
			c.mut(&s)
			if got := OnboardingDone(s); got != c.want {
				t.Fatalf("OnboardingDone(%+v) = %v, want %v", s, got, c.want)
			}
		})
	}
}

func TestProfileComplete(t *testing.T) {
	cases := []struct {
		name string
		s    OnboardingState
		want bool
	}{
		{"all three", OnboardingState{HasUsername: true, HasName: true, HasHomeArea: true}, true},
		{"terms and first run do not matter", OnboardingState{TermsVersion: "x", FirstRunDone: false, HasUsername: true, HasName: true, HasHomeArea: true}, true},
		{"no username", OnboardingState{HasName: true, HasHomeArea: true}, false},
		{"no name", OnboardingState{HasUsername: true, HasHomeArea: true}, false},
		{"no home area", OnboardingState{HasUsername: true, HasName: true}, false},
		{"none", OnboardingState{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ProfileComplete(c.s); got != c.want {
				t.Fatalf("ProfileComplete(%+v) = %v, want %v", c.s, got, c.want)
			}
		})
	}
	if ProfileIncompleteMessage != "Add your username, name and home area." {
		t.Errorf("ProfileIncompleteMessage = %q", ProfileIncompleteMessage)
	}
	if strings.ContainsRune(ProfileIncompleteMessage, rune(0x2014)) || strings.ContainsRune(ProfileIncompleteMessage, rune(0x2013)) {
		t.Errorf("ProfileIncompleteMessage contains a dash character")
	}
	if ErrProfileIncomplete == nil || !errors.Is(ErrProfileIncomplete, ErrProfileIncomplete) {
		t.Errorf("ErrProfileIncomplete must be a non nil sentinel")
	}
}
