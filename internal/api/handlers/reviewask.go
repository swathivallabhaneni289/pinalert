// reviewask.go is the HTTP surface of the author's box of quick 261118-rz6: why
// one's own removed report was removed, and Ask for review. Every rule lives in
// service.ReviewAskService. The routes are gated like the flag route, and a
// cross site POST carries no session cookie (DEC-L, SameSite=Lax).

package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// maxReviewAskBodyBytes caps the ask body: a note of 200 characters of four
// bytes each, or of six byte JSON escapes, fits well inside 2 KiB.
const maxReviewAskBodyBytes = 2048

// ReviewAskRequest is the body of an ask for review: an optional note for the
// moderators, at most 200 characters after trimming. The note is stored as
// plain text, only moderators see it, and it is never sent to the text check.
type ReviewAskRequest struct {
	Note string `json:"note,omitempty" maxLength:"200" example:"The road really was flooded, I took the photo myself."`
}

// ReviewAskResponse is the author's box: the Why line, absent when the removal
// has no recorded reason, and the state. It never names who flagged, how many
// flagged, from where, or the moderator.
type ReviewAskResponse struct {
	Why   string `json:"why,omitempty" example:"Why: False"`
	State string `json:"state" enums:"can_ask,asked,no_asks_left" example:"can_ask"`
}

func reviewAskWire(box service.ReviewAskBox) ReviewAskResponse {
	return ReviewAskResponse{Why: box.Why, State: string(box.State)}
}

// reviewAskNotFound answers every report the caller may not ask about with the
// same 404, so nothing about it is revealed.
func reviewAskNotFound(w http.ResponseWriter) {
	writeFieldError(w, http.StatusNotFound, "report", service.FlagNotFoundMessage)
}

// ReviewAskState produces the handler of GET /reports/{id}/review-ask.
//
// @Summary Read the box of your own removed report
// @Description For the author of a removed report: why it was removed (absent when no reason was recorded)
// @Description and whether they can ask a moderator to review it. A report can be asked about 3 times in
// @Description total, one at a time. Any report that is not the caller's own removed report gets 404 with
// @Description nothing revealed. Requires a session verified by email; an unverified caller receives 401.
// @Tags reports
// @Produce json
// @Param id path int true "Report id"
// @Success 200 {object} ReviewAskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/review-ask [get]
func ReviewAskState(svc *service.ReviewAskService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}
		box, err := svc.State(r.Context(), acc.ID, reportID)
		if err != nil {
			if errors.Is(err, service.ErrReportNotFound) {
				reviewAskNotFound(w)
				return
			}
			log.Printf("handlers: ReviewAskState: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, reviewAskWire(box))
	}
}

// ReviewAskCreate produces the handler of POST /reports/{id}/review-ask.
//
// @Summary Ask a moderator to review your own removed report
// @Description For the author of a removed report: asks a moderator to look again, with an optional note of at
// @Description most 200 characters that only moderators see. The note is never sent to the text check. A report
// @Description can be asked about 3 times in total, one at a time: an ask while one is open, or after 3, gets 409.
// @Description A moderator answers by restoring the report or keeping it removed, and the author is told in the
// @Description bell. Any report that is not the caller's own removed report gets 404 with nothing revealed.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags reports
// @Accept json
// @Produce json
// @Param id path int true "Report id"
// @Param body body ReviewAskRequest true "An optional note for the moderators"
// @Success 201 {object} ReviewAskResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/review-ask [post]
func ReviewAskCreate(svc *service.ReviewAskService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxReviewAskBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req ReviewAskRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}
		box, err := svc.Ask(r.Context(), acc.ID, reportID, req.Note)
		if err != nil {
			var ve service.ValidationError
			switch {
			case errors.As(err, &ve):
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
			case errors.Is(err, service.ErrReportNotFound):
				reviewAskNotFound(w)
			case errors.Is(err, service.ErrReviewAskOpen):
				writeFieldError(w, http.StatusConflict, "report", service.ReviewAskOpenMessage)
			case errors.Is(err, service.ErrReviewAskNoneLeft):
				writeFieldError(w, http.StatusConflict, "report", service.ReviewAskNoneLeftMessage)
			default:
				// The note is never logged: only the error, which carries none.
				log.Printf("handlers: ReviewAskCreate: report %d: %v", reportID, err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusCreated, reviewAskWire(box))
	}
}
