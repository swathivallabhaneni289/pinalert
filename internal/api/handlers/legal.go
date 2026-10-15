package handlers

import (
	"html/template"
	"net/http"
)

// The two public legal pages (D-23).
const (
	LegalDisclaimerPage = "disclaimer"
	LegalPolicyPage     = "policy"
)

// Legal is a declaration only until the GREEN step.
func Legal(tmpl *template.Template, cfg AuthConfig, page string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
