// moderation.go is the moderator page and API of D-08. Every handler here is
// reached only through requireModerator in internal/api, which puts the
// moderator's account in the request context. There are no swag directives on
// purpose: this is an internal surface and stays out of the public API
// reference (RESEARCH Area B Routes row).

package handlers

import (
	"html/template"
	"log"
	"net/http"

	"pinalert/internal/account"
)

// moderateTemplateName is the file executed by ModeratePage.
const moderateTemplateName = "moderate.html.tmpl"

// moderatePageViewModel is exactly what moderate.html.tmpl reads. It carries
// no report text: the lists are filled by script from the JSON API.
type moderatePageViewModel struct {
	AssetVersion string
	Email        string
	Chrome       Chrome
}

// ModeratePage renders the moderator screen shell. It is only reachable
// through requireModerator, so the context always holds the moderator's
// account; a miss is a routing bug and answers 500, like Page.
func ModeratePage(tmpl *template.Template, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: ModeratePage: no verified account in request context")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		vm := moderatePageViewModel{
			AssetVersion: cfg.AssetVersion,
			Email:        acc.Email,
			Chrome:       cfg.Chrome.withDefaults(),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, moderateTemplateName, vm); err != nil {
			log.Printf("handlers: ModeratePage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
