// Package handlers implements the HTTP handlers for Pinalert's JSON API.
// Handlers decode/encode only — all validation, expiry computation, and
// geo-math live in internal/service; all SQL lives behind the sqlc-generated
// Querier. This file also declares the public JSON report shape explicitly,
// field by field, so a column added to the store later cannot silently
// start appearing in a response (see 01-RESEARCH.md's Security Domain:
// report read paths must never return session_id).
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"pinalert/internal/service"
	"pinalert/internal/session"
)

// maxSubmitBodyBytes caps the POST body so an unbounded request can't be
// used as a cheap denial-of-service vector.
const maxSubmitBodyBytes = 64 * 1024

// submitReportRequest is the raw JSON shape a client posts. Every field
// that the server itself computes (id, geohash, created_at, expires_at) is
// deliberately absent here — decode with DisallowUnknownFields so an
// attempt to set one of them is a loud 400, not a silently dropped field.
type submitReportRequest struct {
	Category              string  `json:"category"`
	Severity              string  `json:"severity"`
	Description           string  `json:"description"`
	Latitude              float64 `json:"latitude"`
	Longitude             float64 `json:"longitude"`
	ShelterCapacityStatus *string `json:"shelter_capacity_status,omitempty"`
	ShelterHeadcount      *int    `json:"shelter_headcount,omitempty"`
}

// reportResponse is the public JSON shape of a report. Declared field by
// field rather than reusing any store row type — see package doc comment.
// It never includes session_id.
type reportResponse struct {
	ID                    int64    `json:"id"`
	Category              string   `json:"category"`
	Severity              string   `json:"severity"`
	Description           string   `json:"description"`
	Latitude              float64  `json:"latitude"`
	Longitude             float64  `json:"longitude"`
	Geohash               string   `json:"geohash"`
	ShelterCapacityStatus *string  `json:"shelter_capacity_status,omitempty"`
	ShelterHeadcount      *int     `json:"shelter_headcount,omitempty"`
	CreatedAt             string   `json:"created_at"`
	ExpiresAt             string   `json:"expires_at"`
	DistanceKm            *float64 `json:"distance_km,omitempty"`
}

func reportToResponse(r service.Report) reportResponse {
	var capacityStatus *string
	if r.ShelterCapacityStatus != nil {
		v := string(*r.ShelterCapacityStatus)
		capacityStatus = &v
	}
	return reportResponse{
		ID:                    r.ID,
		Category:              string(r.Category),
		Severity:              string(r.Severity),
		Description:           r.Description,
		Latitude:              r.Latitude,
		Longitude:             r.Longitude,
		Geohash:               r.Geohash,
		ShelterCapacityStatus: capacityStatus,
		ShelterHeadcount:      r.ShelterHeadcount,
		CreatedAt:             r.CreatedAt.Format(rfc3339Milli),
		ExpiresAt:             r.ExpiresAt.Format(rfc3339Milli),
	}
}

const rfc3339Milli = "2006-01-02T15:04:05.000Z07:00"

type fieldErrorBody struct {
	Error struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"error"`
}

// SubmitReport handles POST /api/reports: decode, validate (via svc), and
// persist under the caller's session id. A ValidationError maps to 400 with
// the offending field named; any other error maps to a generic 500 with the
// detail logged server-side only, so database errors never reach a client.
func SubmitReport(svc *service.ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := session.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxSubmitBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req submitReportRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		var capacityStatus *service.CapacityStatus
		if req.ShelterCapacityStatus != nil {
			cs := service.CapacityStatus(*req.ShelterCapacityStatus)
			capacityStatus = &cs
		}

		in := service.SubmitInput{
			Category:              service.Category(req.Category),
			Severity:              service.Severity(req.Severity),
			Description:           req.Description,
			Latitude:              req.Latitude,
			Longitude:             req.Longitude,
			ShelterCapacityStatus: capacityStatus,
			ShelterHeadcount:      req.ShelterHeadcount,
		}

		report, err := svc.Submit(r.Context(), sessionID, in)
		if err != nil {
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: SubmitReport: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"report": reportToResponse(report)})
	}
}

func writeFieldError(w http.ResponseWriter, status int, field, message string) {
	var body fieldErrorBody
	body.Error.Field = field
	body.Error.Message = message
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("handlers: writing JSON response: %v", err)
	}
}
