// moderation.go is the moderator page and API of D-08. Every handler here is
// reached only through requireModerator in internal/api, which puts the
// moderator's account in the request context. There are no swag directives on
// purpose: this is an internal surface and stays out of the public API
// reference (RESEARCH Area B Routes row).

package handlers

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/service"
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

// modFlagReasonJSON is one reason of a row's flags, with the dialog label.
type modFlagReasonJSON struct {
	Reason string `json:"reason"`
	Label  string `json:"label"`
	Count  int64  `json:"count"`
}

// modFlagNoteReasonJSON is one reason a noted flag gave, with its label.
type modFlagNoteReasonJSON struct {
	Reason string `json:"reason"`
	Label  string `json:"label"`
}

// modFlagNoteJSON is one open flag that carries a note: the note text and the
// reasons the same flag gave. It names nobody (T-04-04, plan 04-31). The
// screen puts the note on the page as text, never as markup.
type modFlagNoteJSON struct {
	Reasons []modFlagNoteReasonJSON `json:"reasons"`
	Note    string                  `json:"note"`
}

// modFlagsJSON is the open flag total and the per reason counts. Reasons holds
// only the reasons chosen at least once and is never null; a reason counts
// once per flag it appears in. Notes holds the open flags with a note, newest
// first, at most 20, and is never null.
type modFlagsJSON struct {
	Total   int64               `json:"total"`
	Reasons []modFlagReasonJSON `json:"reasons"`
	Notes   []modFlagNoteJSON   `json:"notes"`
}

// modRowJSON is the ModRow of the wire contract. It names no author, flagger,
// email, account or session (T-04-04).
type modRowJSON struct {
	ID             int64              `json:"id"`
	Category       string             `json:"category"`
	Severity       string             `json:"severity"`
	Description    string             `json:"description"`
	CreatedAt      time.Time          `json:"created_at"`
	ExpiresAt      time.Time          `json:"expires_at"`
	WrittenAt      *time.Time         `json:"written_at,omitempty"`
	Serious        bool               `json:"serious"`
	Moderation     string             `json:"moderation"`
	View           string             `json:"view"`
	VisibilityWord string             `json:"visibility_word,omitempty"`
	Scores         map[string]float64 `json:"scores,omitempty"`
	Model          string             `json:"model,omitempty"`
	Flags          modFlagsJSON       `json:"flags"`
}

// modCountsJSON holds the heading counts, which come from the store's uncapped
// count query and never from the length of a list.
type modCountsJSON struct {
	Review  int64 `json:"review"`
	Flagged int64 `json:"flagged"`
	Pending int64 `json:"pending"`
}

// modQueueJSON is the answer of GET /api/moderation/queue.
type modQueueJSON struct {
	Review  []modRowJSON  `json:"review"`
	Flagged []modRowJSON  `json:"flagged"`
	Pending []modRowJSON  `json:"pending"`
	Counts  modCountsJSON `json:"counts"`
}

// modFindJSON is the answer of GET /api/moderation/reports/{id}.
type modFindJSON struct {
	Report modRowJSON `json:"report"`
	Lists  []string   `json:"lists"`
}

// modRowToJSON copies a service row field by field.
func modRowToJSON(r service.ModerationRow) modRowJSON {
	reasons := make([]modFlagReasonJSON, 0, len(r.FlagReasons))
	for _, fr := range r.FlagReasons {
		reasons = append(reasons, modFlagReasonJSON{Reason: string(fr.Reason), Label: fr.Label, Count: fr.Count})
	}
	notes := make([]modFlagNoteJSON, 0, len(r.FlagNotes))
	for _, fn := range r.FlagNotes {
		nr := make([]modFlagNoteReasonJSON, 0, len(fn.Reasons))
		for _, x := range fn.Reasons {
			nr = append(nr, modFlagNoteReasonJSON{Reason: string(x.Reason), Label: x.Label})
		}
		notes = append(notes, modFlagNoteJSON{Reasons: nr, Note: fn.Note})
	}
	return modRowJSON{
		ID:             r.ID,
		Category:       string(r.Category),
		Severity:       string(r.Severity),
		Description:    r.Description,
		CreatedAt:      r.CreatedAt,
		ExpiresAt:      r.ExpiresAt,
		WrittenAt:      r.WrittenAt,
		Serious:        r.Serious,
		Moderation:     string(r.Moderation),
		View:           r.View,
		VisibilityWord: r.VisibilityWord,
		Scores:         r.Scores,
		Model:          r.Model,
		Flags:          modFlagsJSON{Total: r.FlagTotal, Reasons: reasons, Notes: notes},
	}
}

func modRowsToJSON(rows []service.ModerationRow) []modRowJSON {
	out := make([]modRowJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, modRowToJSON(r))
	}
	return out
}

// modGuard runs the checks every moderator JSON handler starts with: the
// method (a wrong one answers 405 with Allow, reachable only by a moderator),
// and the account that requireModerator put in the context. It sets
// Cache-Control: no-store and returns the moderator's account.
func modGuard(w http.ResponseWriter, r *http.Request, method string) (account.Account, bool) {
	if r.Method != method {
		w.Header().Set("Allow", method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return account.Account{}, false
	}
	acc, ok := account.FromContext(r.Context())
	if !ok {
		// A route mounted outside the moderator group, a programming error.
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return account.Account{}, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return acc, true
}

// ModerationQueue produces the handler of GET /api/moderation/queue: the three
// lists and their heading counts.
func ModerationQueue(svc *service.ModerationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := modGuard(w, r, http.MethodGet); !ok {
			return
		}
		q, err := svc.Queues(r.Context())
		if err != nil {
			log.Printf("handlers: ModerationQueue: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, modQueueJSON{
			Review:  modRowsToJSON(q.Review),
			Flagged: modRowsToJSON(q.Flagged),
			Pending: modRowsToJSON(q.Pending),
			Counts:  modCountsJSON{Review: q.Counts.Review, Flagged: q.Counts.Flagged, Pending: q.Counts.Pending},
		})
	}
}

// modActionJSON is the answer of Keep and Remove: the report as it is now.
type modActionJSON struct {
	Report modRowJSON `json:"report"`
}

// modAct is the shared body of the two POST handlers. act is svc.Keep or
// svc.Remove and logErr writes the one log line of the calling handler for an
// error that is neither a 404 nor a 409.
func modAct(logErr func(error), act func(ctx context.Context, id, moderatorID int64) (service.ModerationRow, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := modGuard(w, r, http.MethodPost)
		if !ok {
			return
		}
		id, ok := parseReportID(w, r)
		if !ok {
			return
		}
		row, err := act(r.Context(), id, acc.ID)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrReportNotFound):
				writeFieldError(w, http.StatusNotFound, "id", service.ModNotFoundMessage)
			case errors.Is(err, service.ErrAlreadyHandled):
				writeFieldError(w, http.StatusConflict, "id", service.ModAlreadyHandledMessage)
			default:
				logErr(err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}
		writeJSON(w, http.StatusOK, modActionJSON{Report: modRowToJSON(row)})
	}
}

// ModerationKeep produces the handler of POST /api/moderation/reports/{id}/keep.
func ModerationKeep(svc *service.ModerationService) http.HandlerFunc {
	return modAct(func(err error) { log.Printf("handlers: ModerationKeep: %v", err) }, svc.Keep)
}

// ModerationRemove produces the handler of POST /api/moderation/reports/{id}/remove.
func ModerationRemove(svc *service.ModerationService) http.HandlerFunc {
	return modAct(func(err error) { log.Printf("handlers: ModerationRemove: %v", err) }, svc.Remove)
}

// ModerationReport produces the handler of GET /api/moderation/reports/{id}:
// any report by id, listed or not, with the lists it is in.
func ModerationReport(svc *service.ModerationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := modGuard(w, r, http.MethodGet); !ok {
			return
		}
		id, ok := parseReportID(w, r)
		if !ok {
			return
		}
		row, lists, err := svc.Find(r.Context(), id)
		if err != nil {
			if errors.Is(err, service.ErrReportNotFound) {
				writeFieldError(w, http.StatusNotFound, "id", service.ModNotFoundMessage)
				return
			}
			log.Printf("handlers: ModerationReport: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, modFindJSON{Report: modRowToJSON(row), Lists: lists})
	}
}
