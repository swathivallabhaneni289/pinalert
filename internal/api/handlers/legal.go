// This file serves the two public legal pages, the Disclaimer and the Policy
// (D-23), and builds the LegalText the in app reader window shares with them
// (Phase 3.4, D-31). Both pages are readable without logging in, render no
// account data (R-07) and are cacheable only in the browser.

package handlers

import (
	"bytes"
	"html/template"
	"log"
	"net/http"

	"pinalert/internal/service"
)

// The two page names Legal accepts.
const (
	LegalDisclaimerPage = "disclaimer"
	LegalPolicyPage     = "policy"
)

// legalCacheControl keeps the pages out of any shared cache: the session
// middleware sets a pinalert_session cookie on a first visit, even for a
// reader who is not logged in, so a shared cache could hand one reader's
// cookie to another (T-04-12).
const legalCacheControl = "private, max-age=300"

// policyLastUpdated is the real date the Policy text was last written. Edit it
// together with the text, never to an invented figure, and change
// service.CurrentTermsVersion to the same day so everyone is asked to agree to
// the new text (D-27, TestTermsVersionFollowsPolicyDate).
const policyLastUpdated = "17 October 2026"

// readerTemplateName is the one standalone page both addresses render (D-31):
// a full width reader holding the Disclaimer and then the Policy.
const readerTemplateName = "reader.html.tmpl"

// legalTemplates maps a page name to its template file. Both pages are the same
// reader; Current says which part is the current one.
var legalTemplates = map[string]string{
	LegalDisclaimerPage: readerTemplateName,
	LegalPolicyPage:     readerTemplateName,
}

// LegalText is exactly what the shared legal body partials read (legal_body
// .html.tmpl). It is the one source of the Disclaimer and Policy text for the
// standalone pages, the in app reader window and the first time flow, so it has
// no email, no account id and nothing taken from a session (R-07, T-34-34).
type LegalText struct {
	Numbers EmergencyNumbers
	// CallHref is safe because it always starts with the literal tel scheme and
	// the primary number is digits only, validated by LoadEmergencyNumbers (D-22).
	CallHref    template.URL
	ContactHref template.URL
	// ContactEmail is the moderation contact address, empty when none is set.
	ContactEmail      string
	OlderLabel        string
	PolicyLastUpdated string
	// ExpiryHours and CriticalExpiryHours are read from service.ExpiryDuration
	// so the Reports expire section follows the constants that expire reports.
	ExpiryHours         int
	CriticalExpiryHours int
	Glossary            []service.GlossaryEntry
}

// legalTextFor builds the legal text values from the page chrome. chrome must
// already have its defaults applied.
func legalTextFor(chrome Chrome) LegalText {
	text := LegalText{
		Numbers: chrome.Numbers,
		// html/template rewrites an untyped tel URL to a placeholder. This
		// value is safe because it always starts with the literal tel
		// scheme and the primary is digits only, validated by
		// LoadEmergencyNumbers (D-22).
		CallHref:            chrome.Numbers.TelHref(),
		ContactEmail:        chrome.ContactEmail,
		OlderLabel:          service.OlderNumbersLabel(chrome.Numbers.Older),
		PolicyLastUpdated:   policyLastUpdated,
		ExpiryHours:         int(service.ExpiryDuration(service.SeverityLow).Hours()),
		CriticalExpiryHours: int(service.ExpiryDuration(service.SeverityCritical).Hours()),
		Glossary:            service.DisclaimerGlossary(),
	}
	if chrome.ContactEmail != "" {
		// main.go validates the address as a bare email address (D-24).
		text.ContactHref = template.URL("mailto:" + chrome.ContactEmail)
	}
	return text
}

// legalViewModel is exactly what the reader page template reads. It has no
// email field on purpose (R-07).
type legalViewModel struct {
	AssetVersion string
	Chrome       Chrome
	// Current is the part the address asked for, disclaimer or policy.
	Current string
	Legal   LegalText
}

// Legal serves one legal page. Any page name other than the two constants
// answers 500, because it is a wiring mistake and never a visitor mistake.
func Legal(tmpl *template.Template, cfg AuthConfig, page string) http.HandlerFunc {
	name, known := legalTemplates[page]
	return func(w http.ResponseWriter, r *http.Request) {
		if !known {
			log.Printf("handlers: Legal: unknown page %q", page)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		chrome := cfg.Chrome.withDefaults()

		vm := legalViewModel{
			AssetVersion: cfg.AssetVersion,
			Chrome:       chrome,
			Current:      page,
			Legal:        legalTextFor(chrome),
		}

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, vm); err != nil {
			log.Printf("handlers: Legal: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", legalCacheControl)
		_, _ = w.Write(buf.Bytes())
	}
}
