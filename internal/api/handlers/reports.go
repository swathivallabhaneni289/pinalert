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
	"strconv"

	"pinalert/internal/service"
	"pinalert/internal/session"
)

// maxSubmitBodyBytes caps the POST body so an unbounded request can't be
// used as a cheap denial-of-service vector.
const maxSubmitBodyBytes = 64 * 1024

// Nearby-query radius defaults and bounds (FOUND-03): a single request must
// never be able to demand an arbitrarily wide Haversine evaluation.
const (
	defaultRadiusKm = 10.0
	minRadiusKm     = 0.1
	maxRadiusKm     = 50.0
)

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
		DistanceKm:            r.DistanceKm,
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

// NearbyReports handles GET /api/reports: parse lat/lon/radius_km from the
// query string, run the indexed bbox+Haversine query via svc, and return
// unexpired reports nearest-first. This is the one endpoint that serves
// both the map and the list (01-RESEARCH.md Pattern 3) — there is no
// separate map-only or list-only route.
func NearbyReports(svc *service.ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		lat, ok := parseCoordinate(w, q, "lat", -90, 90)
		if !ok {
			return
		}
		lon, ok := parseCoordinate(w, q, "lon", -180, 180)
		if !ok {
			return
		}

		radiusKm := defaultRadiusKm
		if raw := q.Get("radius_km"); raw != "" {
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				writeFieldError(w, http.StatusBadRequest, "radius_km", "radius_km must be a number.")
				return
			}
			radiusKm = parsed
		}
		if radiusKm < minRadiusKm || radiusKm > maxRadiusKm {
			writeFieldError(w, http.StatusBadRequest, "radius_km",
				"radius_km must be between 0.1 and 50.")
			return
		}

		reports, err := svc.Nearby(r.Context(), service.NearbyQuery{
			Latitude:  lat,
			Longitude: lon,
			RadiusKm:  radiusKm,
		})
		if err != nil {
			log.Printf("handlers: NearbyReports: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		responses := make([]reportResponse, 0, len(reports))
		for _, rep := range reports {
			responses = append(responses, reportToResponse(rep))
		}
		writeJSON(w, http.StatusOK, map[string]any{"reports": responses})
	}
}

// parseCoordinate reads and range-checks a required numeric query
// parameter, writing the 400 response itself and returning ok=false on any
// failure so the caller can just early-return.
func parseCoordinate(w http.ResponseWriter, q map[string][]string, name string, min, max float64) (value float64, ok bool) {
	raw := ""
	if vals, present := q[name]; present && len(vals) > 0 {
		raw = vals[0]
	}
	if raw == "" {
		writeFieldError(w, http.StatusBadRequest, name, name+" is required.")
		return 0, false
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		writeFieldError(w, http.StatusBadRequest, name, name+" must be a number.")
		return 0, false
	}
	if parsed < min || parsed > max {
		writeFieldError(w, http.StatusBadRequest, name, name+" is out of range.")
		return 0, false
	}
	return parsed, true
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
