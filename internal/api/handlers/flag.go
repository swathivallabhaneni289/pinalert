// flag.go is the HTTP surface of the Flag control (D-09). It decodes the
// request, calls service.FlagService and encodes the answer; every rule lives
// in the service. The route is gated, limited by RateGate.FlagMiddleware, and a
// cross site POST carries no session cookie (DEC-L, SameSite=Lax), so no CSRF
// token is needed.

package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// maxFlagBodyBytes caps the flag POST body, for the same denial of service
// reason as maxVoteBodyBytes. A flag body is at most five reason slugs and a
// note of 300 characters; 300 characters of four bytes each, or of six byte
// JSON escapes, still fit well inside 4 KiB.
const maxFlagBodyBytes = 4096

// FlagRequest is the flag request body: one or more of the five reasons and an
// optional note for the moderators (plan 04-31). The note is screened by the
// same text check as a report description before it is stored, and only
// moderators ever see it.
//
// Reason is the older single reason field, still accepted so a page cached
// before plan 04-31 can flag: alone it is read as a one element Reasons. Sent
// together with Reasons it is a 400, so a body never means two things.
type FlagRequest struct {
	Reasons []string `json:"reasons" enums:"spam,abusive,false,dangerous,other" example:"spam,false"`
	Note    string   `json:"note,omitempty" maxLength:"300" example:"The photo is from a flood in 2019."`
	Reason  string   `json:"reason,omitempty" enums:"spam,abusive,false,dangerous,other" example:"spam"`
}

// FlagResponse is the answer to a stored flag. It says nothing else: no count
// and no identity of any flagger.
type FlagResponse struct {
	Flagged bool `json:"flagged" example:"true"`
}

// FlagReport produces the handler of POST /reports/{id}/flag.
//
// @Summary Flag a report for the moderators
// @Description Sends a report to the moderators with one or more of five reasons: spam, abusive,
// @Description false, dangerous or other, each at most once, and an optional note of at most 300
// @Description characters that only moderators see. The note goes through the same text check as a
// @Description report description, and a refused note gets 422. The older single reason field is
// @Description still accepted on its own as a one reason flag. A flag never hides or changes the report. Each account can flag
// @Description a report once, and a repeat gets 409. Your own report gets 403. A report that does
// @Description not exist, or that is not shown in any feed, gets 404. Flagging is limited per
// @Description verified account, and a blocked flag gets 429 with a Retry-After header.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags reports
// @Accept json
// @Produce json
// @Param id path int true "Report id"
// @Param body body FlagRequest true "One or more of the five reasons and an optional note"
// @Success 201 {object} FlagResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse "The note was refused by the text check"
// @Failure 429 {object} RateLimitResponse "Blocked by a limit, see Retry-After"
// @Header 429 {integer} Retry-After "Whole seconds to wait before trying again"
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/flag [post]
func FlagReport(svc *service.FlagService) http.HandlerFunc {
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

		r.Body = http.MaxBytesReader(w, r.Body, maxFlagBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req FlagRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		if req.Reason != "" && req.Reasons != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}
		raw := req.Reasons
		if req.Reason != "" {
			raw = []string{req.Reason}
		}
		reasons := make([]service.FlagReason, len(raw))
		for i, s := range raw {
			reasons[i] = service.FlagReason(s)
		}

		err := svc.Flag(r.Context(), service.FlagInput{
			ReportID:  reportID,
			AccountID: acc.ID,
			Reasons:   reasons,
			Note:      req.Note,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrFlagOwnReport):
				writeFieldError(w, http.StatusForbidden, "report", service.FlagOwnReportMessage)
				return
			case errors.Is(err, service.ErrReportNotFound):
				writeFieldError(w, http.StatusNotFound, "report", service.FlagNotFoundMessage)
				return
			case errors.Is(err, service.ErrAlreadyFlagged):
				writeFieldError(w, http.StatusConflict, "report", service.FlagAlreadyMessage)
				return
			}
			var nr service.FlagNoteRejectedError
			if errors.As(err, &nr) {
				writeFieldError(w, http.StatusUnprocessableEntity, "note", service.RejectMessage(nr.Reason))
				return
			}
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: FlagReport: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, FlagResponse{Flagged: true})
	}
}
