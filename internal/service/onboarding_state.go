// onboarding_state.go holds the one predicate that decides whether a signed in
// account has finished the first-time flow (Phase 7.1, D-06, D-08).

package service

import "errors"

// OnboardingState is what the predicate reads. The callers fill it from the
// account row: the terms version the person agreed to, the first-time flow
// flag, and whether each of the three required profile fields is present.
type OnboardingState struct {
	TermsVersion string
	FirstRunDone bool
	HasUsername  bool
	HasName      bool
	HasHomeArea  bool
}

// ProfileComplete reports whether all three required profile fields are
// present: a username, a name and a home area.
func ProfileComplete(s OnboardingState) bool {
	return s.HasUsername && s.HasName && s.HasHomeArea
}

// OnboardingDone is the one predicate of research section 6. It is true only
// when the account agreed to CurrentTermsVersion, finished the first-time flow
// and has all three profile fields.
//
// It is called by api.onboarded (the gate), handlers.WelcomePage and
// AccountService.MarkFirstRunDone (wired in plan 10). Using one function in all
// three is what stops a legacy account from looping between / and /welcome
// (research Pitfall 3). /api/ paths keep passing requireOnboarded for every
// account, so a Phase 8 API route that needs a username must check it itself.
func OnboardingDone(s OnboardingState) bool {
	return s.TermsVersion == CurrentTermsVersion && s.FirstRunDone && ProfileComplete(s)
}

// ErrProfileIncomplete is returned when the first-time flow is finished
// without all three required profile fields.
var ErrProfileIncomplete = errors.New("service: username, name and home area are required")

// ProfileIncompleteMessage is the sentence shown for ErrProfileIncomplete.
const ProfileIncompleteMessage = "Add your username, name and home area."
