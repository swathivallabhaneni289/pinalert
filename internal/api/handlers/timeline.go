// timeline.go is the HTTP surface of the report history, "What happened so far"
// (plan 08-01, PAGE-01, SC2). It decodes nothing, calls service.TimelineService
// and encodes the answer; every rule lives in the service. The route is gated,
// and it is a read polled at the feed's cadence, so it has no rate gate.

package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// TimelineEventResponse is one line of the history. It carries words, a tone, a
// time and a flag, and nothing else: no account, no session, no voter, no place
// and no cell (TRUST-MODEL, what never leaves the server).
type TimelineEventResponse struct {
	// Kind is one of posted, confirmed, disputed, resolved, reopened, faded or yours.
	Kind string `json:"kind" enums:"posted,confirmed,disputed,resolved,reopened,faded,yours" example:"confirmed"`
	// Tone is good, neutral or bad.
	Tone string `json:"tone" enums:"good,neutral,bad" example:"good"`
	// Text is the finished sentence, from closed server copy.
	Text string `json:"text" example:"Confirmed from a second spot."`
	// At is the time of the vote that caused the line, RFC 3339 in UTC.
	At string `json:"at" example:"2026-10-01T12:05:00Z"`
	// Yours is true for a line about the reader's own vote.
	Yours bool `json:"yours" example:"false"`
}

// TimelineResponse is the answer of GET /reports/{id}/timeline.
type TimelineResponse struct {
	Events []TimelineEventResponse `json:"events"`
}

// ReportTimeline produces the handler of GET /reports/{id}/timeline.
//
// @Summary The history of a report, what happened so far
// @Description Returns the history of one report as short sentences, oldest first, built only
// @Description from the report's vote log. Every count is the number of distinct spots the trust
// @Description block counts at that moment, the poster's own votes never count, and the history
// @Description names no voter, no account, no place and no cell. It shows upward steps and state
// @Description changes only (posted, confirmed, disputed, shown to everyone, marked Disputed,
// @Description resolved, reopened, faded) plus the reader's own lines, which carry yours true.
// @Description A report that does not exist, is too old, or is not shown to the reader gets 404.
// @Description Requires a signed in session; otherwise 401.
// @Tags reports
// @Produce json
// @Param id path int true "Report id"
// @Success 200 {object} TimelineResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/timeline [get]
func ReportTimeline(svc *service.TimelineService) http.HandlerFunc {
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

		events, err := svc.Timeline(r.Context(), reportID, acc.ID)
		if err != nil {
			if errors.Is(err, service.ErrReportNotFound) {
				writeFieldError(w, http.StatusNotFound, "report", service.TimelineNotFoundMessage)
				return
			}
			// The report id only: nothing a voter or a reader could be traced by.
			log.Printf("handlers: ReportTimeline: report %d: %v", reportID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		resp := TimelineResponse{Events: make([]TimelineEventResponse, 0, len(events))}
		for _, e := range events {
			resp.Events = append(resp.Events, TimelineEventResponse{
				Kind:  string(e.Kind),
				Tone:  e.Tone,
				Text:  e.Text,
				At:    e.At.UTC().Format(time.RFC3339),
				Yours: e.Yours,
			})
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, resp)
	}
}
