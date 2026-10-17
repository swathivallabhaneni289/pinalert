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

// welcomeSteps are the names of the steps that render in Phase 3.4, in order.
// The laptop step list is drawn from them and the count shown as "Step n of 4"
// is their number. The Alerts step waits for Phase 3.3; when it adds a step it
// adds a name here and a section to the template, and the count follows.
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
	// StartStep is 1, or 3 when the current terms are already agreed, so a
	// person who left after Agree does not read and agree twice.
	StartStep int
	Steps     []welcomeStepView
	// DisplayName and HomeAreaName are what a returning account already has
	// stored; step 4 shows them and html/template escapes both. The home area's
	// coarse cell is never read into the page.
	DisplayName  string
	HomeAreaName string
	// AccountID is the signed in account's numeric id, rendered as
	// data-account-id on the flow root so the position saved in step 3 has an
	// owner (skeptic S-2). It is a number, never the address.
	AccountID int64
}

// welcomeStartStep is the step a person lands on. Agreement is to a text, so
// only the current terms version skips step 2.
func welcomeStartStep(acc account.Account) int {
	if acc.TermsVersion == service.CurrentTermsVersion {
		return 3
	}
	return 1
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

		if acc.TermsVersion == service.CurrentTermsVersion && acc.FirstRunDone {
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

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, welcomeTemplateName, vm); err != nil {
			log.Printf("handlers: WelcomePage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
