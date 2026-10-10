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
//
// Both coordinates are pointers so an absent key or a JSON null can be told
// apart from a real 0.
type CastVoteRequest struct {
	// Latitude must be between -90 and 90. Required, except on the reporter's own resolve and reopen.
	Latitude *float64 `json:"latitude" example:"13.0827"`
	// Longitude must be between -180 and 180. Required, except on the reporter's own resolve and reopen.
	Longitude *float64 `json:"longitude" example:"80.2707"`
}

// CastVoteResponse is the resolver's own answer and its own explanation,
// serialised verbatim from service.CastVoteResult — the client renders them
// and never recomputes them (D-04, TRUST-02). The enum tags are the
// published form of the two closed sets visibility.go declares, so 02-06's
// display-copy mapping has a documented contract to key off. Trust is the
// same trust object GET /reports carries for this report (D-21): distinct
// place counts and server written sentences, never a raw vote total, a
// voter's place or an account id.
type CastVoteResponse struct {
	Visibility string `json:"visibility" enums:"hidden,provisional,live,retracted" example:"live"`
	Reason     string `json:"reason" enums:"resolved,critical_bypasses_gates,disputed,awaiting_second_independent_confirmation,confirmed" example:"confirmed"`
	// Trust is the server built trust object for this report, the same value
	// the feed returns (D-21). It is present on every successful vote.
	Trust TrustResponse `json:"trust"`
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
// @Description A voter more than 1 km from the report (service.MaxVoteDistanceKm) gets 403 whose
// @Description error field is location, on all four votes, with one fixed sentence that never
// @Description carries the distance. The reporter's own resolve and reopen are exempt.
// @Description The response also carries the same trust object GET /reports returns for this report.
// @Description A location is required on every vote except the reporter's own resolve and reopen.
// @Description A body with no latitude and no longitude is 400 with error field latitude, and a body
// @Description with only one of the two is 400 naming the missing one.
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

		// Both keys absent leaves the decision to the service, which excuses
		// only the reporter's own resolve or reopen, judged from the database
		// reporter id. Exactly one key absent is a 400 naming the missing key
		// here, because no vote of any kind can use half a location.
		in := service.CastVoteInput{
			ReportID:  reportID,
			AccountID: acc.ID,
			Kind:      kind,
			Value:     value,
		}
		switch {
		case req.Latitude == nil && req.Longitude == nil:
			in.LocationMissing = true
		case req.Latitude == nil:
			writeFieldError(w, http.StatusBadRequest, "latitude", "latitude is required.")
			return
		case req.Longitude == nil:
			writeFieldError(w, http.StatusBadRequest, "longitude", "longitude is required.")
			return
		default:
			in.Latitude = *req.Latitude
			in.Longitude = *req.Longitude
		}

		res, err := svc.CastVote(r.Context(), in)
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
			case errors.Is(err, service.ErrVoterTooFar):
				// 403 rather than 400: the request is well formed and the
				// caller is authenticated, and the server refuses it because
				// the voter stands farther than service.MaxVoteDistanceKm
				// from the report (D-01). The field is location so the
				// browser can key its one silent retry on it (D-02), and the
				// message is the fixed sentence with no computed distance.
				writeFieldError(w, http.StatusForbidden, "location", service.TooFarMessage())
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
			Trust:      trustToResponse(res.Trust),
		})
	}
}
