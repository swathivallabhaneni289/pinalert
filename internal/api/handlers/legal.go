// This file serves the two public legal pages, the Disclaimer and the Policy
// (D-23). Both are readable without logging in, render no account data (R-07)
// and are cacheable only in the browser.

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
// together with the text, never to an invented figure.
const policyLastUpdated = "16 October 2026"

// legalTemplates maps a page name to its template file.
var legalTemplates = map[string]string{
	LegalDisclaimerPage: "disclaimer.html.tmpl",
	LegalPolicyPage:     "policy.html.tmpl",
}

// legalViewModel is exactly what the two legal templates read. It has no
// email field on purpose (R-07).
type legalViewModel struct {
	AssetVersion      string
	Chrome            Chrome
	CallHref          template.URL
	ContactHref       template.URL
	OlderLabel        string
	PolicyLastUpdated string
	// ExpiryHours and CriticalExpiryHours are read from service.ExpiryDuration
	// so the Reports expire section follows the constants that expire reports.
	ExpiryHours         int
	CriticalExpiryHours int
	Glossary            []service.GlossaryEntry
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
			// html/template rewrites an untyped tel URL to a placeholder. This
			// value is safe because it always starts with the literal tel
			// scheme and the primary is digits only, validated by
			// LoadEmergencyNumbers (D-22).
			CallHref:            chrome.Numbers.TelHref(),
			OlderLabel:          service.OlderNumbersLabel(chrome.Numbers.Older),
			PolicyLastUpdated:   policyLastUpdated,
			ExpiryHours:         int(service.ExpiryDuration(service.SeverityLow).Hours()),
			CriticalExpiryHours: int(service.ExpiryDuration(service.SeverityCritical).Hours()),
			Glossary:            service.DisclaimerGlossary(),
		}
		if chrome.ContactEmail != "" {
			// main.go validates the address as a bare email address (D-24).
			vm.ContactHref = template.URL("mailto:" + chrome.ContactEmail)
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
