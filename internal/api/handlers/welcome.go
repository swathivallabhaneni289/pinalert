// This file serves the first-time flow at /welcome (Phase 3.4, plan 10, D-27,
// D-28, orchestrator decision 5): four steps on a page of their own, shown to an
// account that has not agreed to the current terms or not finished the flow.

package handlers

import (
	"html/template"
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// welcomeTemplateName is the file executed by WelcomePage's ExecuteTemplate
// call.
const welcomeTemplateName = "welcome.html.tmpl"

// welcomeRedirectTarget is where an account that is already through the flow is
// sent. It is a constant, so no part of a request can build the target.
const welcomeRedirectTarget = "/"

// welcomeSteps are the names of the steps of the first-time flow, in order.
// The laptop step list is drawn from them and the count shown as "Step n of 4"
// is their number. The alerts question is not a step: it is the pop-up after
// step 4 (quick 261027-t5w, owner, 2026-10-27, which replaced Phase 3.3 plan
// 13's fifth step). A later step adds a name here and a section to the template.
var welcomeSteps = []string{"Welcome", "Disclaimer and policy", "Your location", "About you"}

// welcomeStepView is one entry of the laptop step list.
type welcomeStepView struct {
	Number int
	Name   string
}

// welcomeViewModel is exactly what welcome.html.tmpl reads. It carries no
// email, and no position: the page never learns where the person is.
type welcomeViewModel struct {
	AssetVersion string
	Chrome       Chrome
	// Legal is the same text the reader shows (the shared body partials).
	Legal LegalText
	// StartStep is the first step that still has something to do (see
	// welcomeStartStep): 1 for a new account, 3 when the current terms are
	// agreed but the flow is not finished (so a person who left after Agree does
	// not read and agree twice), 2 for a finished account whose terms are out of
	// date, and 4 for a finished account on the current terms that lacks a
	// username, a name or a home area (the one time fill in, Phase 7.1 D-08).
	StartStep int
	// AfterTermsStep is the step after Agree: 4 for an account that finished the
	// flow before (it already knows steps 1 and 3 and goes straight on to the
	// missing About you fields), else 3. Rendered by plan 13 as a data attribute.
	AfterTermsStep int
	// FillingIn is true for an account that finished the flow before (FirstRunDone)
	// and is back only to add what is missing. It switches the step 4 lead to
	// the fill in sentence (UI-SPEC Surface 7).
	FillingIn bool
	Steps     []welcomeStepView
	// DisplayName, HomeAreaName and Username are what a returning account
	// already has stored; step 4 shows them and html/template escapes all three.
	// The home area's coarse cell is never read into the page. Username prefills
	// the Username box (Phase 7.1 plan 10).
	DisplayName  string
	HomeAreaName string
	Username     string
	// AccountID is the signed in account's numeric id, rendered as
	// data-account-id on the flow root so the position saved in step 3 has an
	// owner (skeptic S-2). It is a number, never the address.
	AccountID int64
	// PushPublicKey is the public VAPID key, rendered as data-push-key on the
	// flow root so the pop-up after step 4 can turn alerts on (Phase 3.3, quick
	// 261027-t5w). It is public by design and empty when push is off, which
	// welcome.js reads as no pop-up.
	PushPublicKey string
}

// welcomeStartStep is the step a person lands on: the first missing step among
// steps 1 to 4. Agreement is to a text, so only the current terms version skips
// step 2.
//
// Amended 2026-10-29 by Phase 7.1 plan 10 (D-08, SC5). Four cases:
//   - current terms and the flow finished before: 4. Only the About you fields
//     are missing (the account would have been sent away otherwise), so this is
//     the one time fill in.
//   - current terms and the flow not finished: 3, as before.
//   - older terms and the flow finished before: 2. The person agrees to the
//     new text and then goes straight on to step 4 (AfterTermsStep).
//   - anything else (older or no terms and the flow not finished): 1.
func welcomeStartStep(acc account.Account) int {
	current := acc.TermsVersion == service.CurrentTermsVersion
	switch {
	case current && acc.FirstRunDone:
		return 4
	case current:
		return 3
	case acc.FirstRunDone:
		return 2
	}
	return 1
}

// welcomeAfterTermsStep is the step after Agree: 4 for an account that finished
// the flow before, else 3.
func welcomeAfterTermsStep(acc account.Account) int {
	if acc.FirstRunDone {
		return 4
	}
	return 3
}

// welcomeOnboardingState reads the account the gate resolved into the one
// predicate's input. It is the same mapping api.onboarded uses.
func welcomeOnboardingState(acc account.Account) service.OnboardingState {
	return service.OnboardingState{
		TermsVersion: acc.TermsVersion,
		FirstRunDone: acc.FirstRunDone,
		HasUsername:  acc.HasUsername,
		HasName:      acc.HasName,
		HasHomeArea:  acc.HasHomeArea,
	}
}

// WelcomePage handles GET /welcome. An account that agreed to the current terms
// and finished the flow gets a 303 to the constant "/" (the onboarding
// middleware lets /welcome itself through, so this cannot loop). Anyone else
// gets the flow, with no-store because the page holds the person's own stored
// values.
//
// It sits in the gated group, so a missing account in the context is a routing
// bug: logged and answered with a 500.
//
// Deliberately carries no swag annotation: it serves text/html and lives
// outside @BasePath /api.
func WelcomePage(tmpl *template.Template, accounts *service.AccountService, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: WelcomePage: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// The one predicate (Phase 7.1 plan 10): current terms, the flow
		// finished, and a username, a name and a home area. The gate and
		// first-run-done read the same function, so an account that is not done
		// here is never let through there, and the two pages cannot send an
		// account back and forth.
		if service.OnboardingDone(welcomeOnboardingState(acc)) {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, welcomeRedirectTarget, http.StatusSeeOther)
			return
		}

		profile, err := accounts.Profile(r.Context(), acc.ID)
		if err != nil {
			log.Printf("handlers: WelcomePage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		chrome := cfg.Chrome.withDefaults()
		vm := welcomeViewModel{
			AssetVersion: cfg.AssetVersion,
			Chrome:       chrome,
			Legal:        legalTextFor(chrome),
			StartStep:    welcomeStartStep(acc),
			AccountID:    acc.ID,

			AfterTermsStep: welcomeAfterTermsStep(acc),
			FillingIn:      acc.FirstRunDone,

			PushPublicKey: cfg.PushPublicKey,
		}
		for i, name := range welcomeSteps {
			vm.Steps = append(vm.Steps, welcomeStepView{Number: i + 1, Name: name})
		}
		if profile.DisplayName != nil {
			vm.DisplayName = *profile.DisplayName
		}
		if profile.HomeAreaName != nil {
			vm.HomeAreaName = *profile.HomeAreaName
		}
		if profile.Username != nil {
			vm.Username = *profile.Username
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, welcomeTemplateName, vm); err != nil {
			log.Printf("handlers: WelcomePage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
