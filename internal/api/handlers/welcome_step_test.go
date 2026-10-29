package handlers

// welcome_step_test.go: the start step and the step after Agree for the one
// time fill in (Phase 7.1 plan 10, D-08, SC5). No database. The template reads
// AfterTermsStep and FillingIn only from plan 13 on, so these functions are
// pinned here, where the rendered page cannot yet show them.

import (
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

func TestWelcomeStartStepCases(t *testing.T) {
	current := service.CurrentTermsVersion
	cases := []struct {
		name      string
		acc       account.Account
		wantStart int
		wantAfter int
	}{
		{"new account", account.Account{}, 1, 3},
		{"agreed, flow not finished", account.Account{TermsVersion: current}, 3, 3},
		{"older terms, flow not finished", account.Account{TermsVersion: "2020-01-01"}, 1, 3},
		{"current terms, finished before, profile missing", account.Account{TermsVersion: current, FirstRunDone: true}, 4, 4},
		{"older terms, finished before", account.Account{TermsVersion: "2020-01-01", FirstRunDone: true}, 2, 4},
	}
	for _, c := range cases {
		if got := welcomeStartStep(c.acc); got != c.wantStart {
			t.Errorf("%s: start step = %d, want %d", c.name, got, c.wantStart)
		}
		if got := welcomeAfterTermsStep(c.acc); got != c.wantAfter {
			t.Errorf("%s: step after Agree = %d, want %d", c.name, got, c.wantAfter)
		}
	}
}

// TestWelcomeStateMatchesTheGate pins that the page reads the account into the
// one predicate the same way the gate does: a complete finished account is done,
// and each missing fact makes it not done.
func TestWelcomeStateMatchesTheGate(t *testing.T) {
	full := account.Account{
		TermsVersion: service.CurrentTermsVersion, FirstRunDone: true,
		HasUsername: true, HasName: true, HasHomeArea: true,
	}
	if !service.OnboardingDone(welcomeOnboardingState(full)) {
		t.Fatal("a complete finished account must be done")
	}
	for label, mutate := range map[string]func(*account.Account){
		"no username": func(a *account.Account) { a.HasUsername = false },
		"no name":     func(a *account.Account) { a.HasName = false },
		"no home":     func(a *account.Account) { a.HasHomeArea = false },
		"not done":    func(a *account.Account) { a.FirstRunDone = false },
		"old terms":   func(a *account.Account) { a.TermsVersion = "2020-01-01" },
	} {
		a := full
		mutate(&a)
		if service.OnboardingDone(welcomeOnboardingState(a)) {
			t.Errorf("%s: the account must not be done", label)
		}
	}
}
