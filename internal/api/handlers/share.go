// share.go is the HTTP surface of Share (plan 08-03, COORD-08, D-04). One gated
// POST makes, or returns, the share link of a report and answers the absolute
// address, the title and the fixed line. The address is rooted in configuration
// (Deps.PublicBaseURL), never in the request's Host or any forwarded header, so
// a crafted header cannot poison a link someone forwards (T-08-10). The route is
// limited by RateGate.ShareMiddleware, and a cross site POST carries no session
// cookie (DEC-L, SameSite=Lax) and gains nothing from a forced share, so no
// CSRF token is needed.

package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// DefaultPublicBaseURL is the base an empty Deps.PublicBaseURL stands for. It is
// for development and tests only: cmd/server refuses to start outside
// ENV=development unless BASE_URL is an https address.
const DefaultPublicBaseURL = "http://localhost"

// sharePathPrefix is the public page's path, followed by the token.
const sharePathPrefix = "/r/"

// PublicShareURL builds the absolute address of a share token from the
// configured base. A trailing slash on base is trimmed. Plan 08-04 uses it for
// og:url, so the page and the link name the same address.
func PublicShareURL(base, token string) string {
	return strings.TrimRight(base, "/") + sharePathPrefix + token
}

// ShareResponse is the answer of POST /reports/{id}/share: the link, the title
// and the fixed line, and nothing else. It carries no count, no name and no
// place.
type ShareResponse struct {
	URL   string `json:"url" example:"https://pinalert.example/r/Zk3pQ9xV1mTq7nBw5cLd2A"`
	Title string `json:"title" example:"Flood report near Benz Circle"`
	Line  string `json:"line" example:"Open to see if this is still true right now"`
}

// ShareReport produces the handler of POST /reports/{id}/share.
//
// @Summary Make a share link
// @Description Makes, or returns, the share link of one report. The link opens a public page with
// @Description the report's live status. The response carries the absolute link, a title and one
// @Description fixed line, and never a count; repeated calls answer the same link. A report that
// @Description does not exist, is too old, or is not shown to the reader gets 404 and no link is
// @Description made. Sharing is limited per signed in account, and a blocked call gets 429 with a
// @Description Retry-After header. Requires a signed in session; otherwise 401.
// @Tags reports
// @Produce json
// @Param id path int true "Report id"
// @Success 200 {object} ShareResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/share [post]
func ShareReport(svc *service.ShareService, base string) http.HandlerFunc {
	if base == "" {
		base = DefaultPublicBaseURL
	}
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			// A route mounted outside the gated group, a programming error.
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}

		// The request has no body and a body is ignored. The area lookup inside
		// EnsureLink is bounded by service.PushAreaLookupTimeout, well inside the
		// server write timeout, so no write deadline extension is needed.
		link, err := svc.EnsureLink(r.Context(), reportID, acc.ID)
		if err != nil {
			if errors.Is(err, service.ErrReportNotFound) {
				writeFieldError(w, http.StatusNotFound, "report", service.TimelineNotFoundMessage)
				return
			}
			// The report id only: nothing a reader could be traced by.
			log.Printf("handlers: ShareReport: report %d: %v", reportID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, ShareResponse{
			URL:   PublicShareURL(base, link.Token),
			Title: link.Title,
			Line:  link.Line,
		})
	}
}
