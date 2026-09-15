// votes.go is the HTTP surface of the confirm/dispute/resolve/reopen
// mechanic. All four endpoints (POST /reports/{id}/confirm, /dispute,
// /resolve, /reopen) are produced by ONE factory, CastVote, so the JSON
// contract and the error mapping exist exactly once. Every trust decision —
// self-vote authorization, expiry rejection, the independence predicate, and
// visibility resolution — is made in internal/service, never in this file.
// This file decodes the request, calls the service, and encodes the
// response, following reports.go's package-level "handlers decode/encode
// only" rule.

package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// maxVoteBodyBytes caps the vote POST body. A vote body is two floats, so
// this cap is far tighter than reports.go's 64 KiB submit cap — same
// denial-of-service rationale as maxSubmitBodyBytes, sized to the payload.
const maxVoteBodyBytes = 4 * 1024

// CastVoteRequest is the one request shape all four vote endpoints accept.
// The client sends the raw coordinates captured by the browser's
// once-per-session geolocation prompt (D-17) and nothing else — the voter's
// cell, the vote's validity, and the resulting visibility are all computed
// server-side. Because the decoder that reads this struct rejects unknown
// JSON keys (see CastVote below), a body that tries to supply a precomputed
// cell or a visibility is a loud 400 rather than a silently dropped field —
// the same protection SubmitReportRequest already gives an attempt to set
// expires_at/created_at.
type CastVoteRequest struct {
	// Latitude must be between -90 and 90.
	Latitude float64 `json:"latitude" example:"13.0827"`
	// Longitude must be between -180 and 180.
	Longitude float64 `json:"longitude" example:"80.2707"`
}

// CastVoteResponse is the resolver's own answer and its own explanation,
// serialised verbatim from service.CastVoteResult — the client renders them
// and never recomputes them (D-04, TRUST-02). The enum tags are the
// published form of the two closed sets visibility.go declares, so 02-06's
// display-copy mapping has a documented contract to key off. It deliberately
// carries no vote counts: the weighted "confirmed by N nearby" number is
// Phase 3 / TRUST-05.
type CastVoteResponse struct {
	Visibility string `json:"visibility" enums:"hidden,provisional,live,retracted" example:"live"`
	Reason     string `json:"reason" enums:"resolved,critical_bypasses_gates,disputed,awaiting_second_independent_confirmation,confirmed" example:"confirmed"`
}

// parseReportID reads the {id} path parameter, parses it, and on an empty,
// unparseable, or non-positive value writes the 400 response itself and
// returns ok=false — mirroring parseCoordinate's write-the-error-yourself
// contract in reports.go. A non-positive id is rejected here rather than
// left to reach the store: reports.id is a BIGSERIAL starting at 1, so 0 and
// negatives are malformed input, not missing rows.
func parseReportID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeFieldError(w, http.StatusBadRequest, "id", "Report id must be a number.")
		return 0, false
	}
	return id, true
}

// CastVote produces the HTTP handler for one of the four vote endpoints,
// closing over the kind/value pair the route registration supplies. All
// four routes share this single factory so the JSON contract and error
// mapping exist exactly once.
//
// @Summary Cast a confirm, dispute, resolve, or reopen vote on a report
// @Description Casts one of four votes against a report: confirm/dispute are content votes (is
// @Description this report still true), resolve/reopen are resolution votes (is this resolved).
// @Description Requires a session verified by email through the magic-link flow; an unverified
// @Description caller receives 401. The reporter receives 403 on confirm/dispute but may resolve
// @Description or reopen their own report instantly. The response carries the freshly recomputed
// @Description visibility and reason so the client can render the outcome without guessing.
// @Tags reports
// @Accept json
// @Produce json
// @Param id path int true "Report id"
// @Param body body CastVoteRequest true "Voter's current coordinates"
// @Success 200 {object} CastVoteResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/confirm [post]
// @Router /reports/{id}/dispute [post]
// @Router /reports/{id}/resolve [post]
// @Router /reports/{id}/reopen [post]
func CastVote(svc *service.VotingService, kind service.VoteKind, value service.VoteValue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			// Reaching this handler at all means the gate already resolved
			// a verified account, so a missing one is a programming error
			// (a route mounted outside the gated group), not a client
			// condition — the same shape SubmitReport uses for a missing
			// session id.
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxVoteBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req CastVoteRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		res, err := svc.CastVote(r.Context(), service.CastVoteInput{
			ReportID:  reportID,
			AccountID: acc.ID,
			Kind:      kind,
			Value:     value,
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
		})
		if err != nil {
			// The errors.Is sentinel branches are ordered ahead of the
			// errors.As validation branch below, and deliberately so: the
			// sentinels are distinct error values errors.As on a
			// ValidationError would not match anyway, and ordering them
			// first keeps the security-relevant refusals visibly at the
			// top of the map rather than buried under input handling.
			switch {
			case errors.Is(err, service.ErrCannotVoteOwnReport):
				// 403 rather than 400: the request is well-formed, the
				// caller is authenticated, and the server is refusing on
				// authorisation grounds (D-03).
				writeFieldError(w, http.StatusForbidden, "account", "You can't vote on your own report.")
				return
			case errors.Is(err, service.ErrReportNotFound):
				writeFieldError(w, http.StatusNotFound, "report", "Report not found.")
				return
			case errors.Is(err, service.ErrReportExpired):
				// 409 Conflict because the request conflicts with the
				// report's current state; 410 Gone was rejected because the
				// report still exists and is still retrievable through its
				// owner's Activity history; 400 was rejected because
				// nothing about the request itself is malformed.
				writeFieldError(w, http.StatusConflict, "report", "This report has expired.")
				return
			}
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: CastVote: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// 200, not 201: a vote is not a newly created addressable resource
		// from the client's point of view, and D-02 means the same caller
		// re-POSTing changes their standing vote rather than creating a
		// second one.
		writeJSON(w, http.StatusOK, CastVoteResponse{
			Visibility: string(res.Visibility),
			Reason:     string(res.Reason),
		})
	}
}
