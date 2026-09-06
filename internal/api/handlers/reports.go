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

// SubmitReportRequest is the raw JSON shape a client posts to POST
// /api/reports. Every field that the server itself computes (id, geohash,
// created_at, expires_at) is deliberately absent here — decode with
// DisallowUnknownFields so an attempt to set one of them is a loud 400, not
// a silently dropped field. Exported and hand-declared field by field (never
// aliasing a store row) so swag can document it by name and so a column
// added to the store later cannot silently change this contract.
type SubmitReportRequest struct {
	// Category is one of the nine canonical report categories (internal/service.Categories).
	Category string `json:"category" enums:"flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other" example:"flood"`
	// Severity affects triage ordering only; self-declaring critical never bypasses the provisional-visibility gate (internal/service.Severities).
	Severity string `json:"severity" enums:"low,medium,critical" example:"medium"`
	// Description is a free-text account of the report, 10-1000 characters.
	Description string `json:"description" example:"Water rising fast near the market bridge."`
	// Latitude must be between -90 and 90.
	Latitude float64 `json:"latitude" example:"13.0827"`
	// Longitude must be between -180 and 180.
	Longitude float64 `json:"longitude" example:"80.2707"`
	// ShelterCapacityStatus is required when category is shelter_open and forbidden for every other category (internal/service.CapacityStatuses).
	ShelterCapacityStatus *string `json:"shelter_capacity_status,omitempty" enums:"available,limited,full,closed" example:"available"`
	// ShelterHeadcount is optional and only valid alongside category shelter_open.
	ShelterHeadcount *int `json:"shelter_headcount,omitempty" example:"42"`
}

// ReportResponse is the public JSON shape of a report, returned by both
// POST /api/reports and GET /api/reports. Declared field by field rather
// than reusing any store row type — see package doc comment. It never
// includes session_id (threat register T-01-02).
type ReportResponse struct {
	ID          int64   `json:"id" example:"42"`
	Category    string  `json:"category" enums:"flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other" example:"flood"`
	Severity    string  `json:"severity" enums:"low,medium,critical" example:"critical"`
	Description string  `json:"description" example:"Water rising fast near the market bridge."`
	Latitude    float64 `json:"latitude" example:"13.0827"`
	Longitude   float64 `json:"longitude" example:"80.2707"`
	Geohash     string  `json:"geohash" example:"tdr1qgzp"`
	// ShelterCapacityStatus is present only for shelter_open reports.
	ShelterCapacityStatus *string `json:"shelter_capacity_status,omitempty" enums:"available,limited,full,closed" example:"available"`
	ShelterHeadcount      *int    `json:"shelter_headcount,omitempty" example:"42"`
	CreatedAt             string  `json:"created_at" example:"2026-09-05T14:03:00.000Z"`
	ExpiresAt             string  `json:"expires_at" example:"2026-09-06T14:03:00.000Z"`
	// DistanceKm is present only in GET /api/reports responses (nil on the report just submitted).
	DistanceKm *float64 `json:"distance_km,omitempty" example:"1.42"`
}

// SubmitReportResponse wraps the single report created by a successful
// POST /api/reports.
type SubmitReportResponse struct {
	Report ReportResponse `json:"report"`
}

// ReportListResponse wraps the unexpired, nearest-first reports returned by
// GET /api/reports.
type ReportListResponse struct {
	Reports []ReportResponse `json:"reports"`
}

func reportToResponse(r service.Report) ReportResponse {
	var capacityStatus *string
	if r.ShelterCapacityStatus != nil {
		v := string(*r.ShelterCapacityStatus)
		capacityStatus = &v
	}
	return ReportResponse{
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

// ErrorDetail names the single input field that failed validation (or
// "body" for a malformed request) and a user-safe message.
type ErrorDetail struct {
	Field   string `json:"field" example:"category"`
	Message string `json:"message" example:"Choose a category to continue."`
}

// ErrorResponse is the JSON shape of every 400 and 500 response this API
// returns. Exported so swag can document it by name.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// SubmitReport handles POST /api/reports: decode, validate (via svc), and
// persist under the caller's session id. A ValidationError maps to 400 with
// the offending field named; any other error maps to a generic 500 with the
// detail logged server-side only, so database errors never reach a client.
//
// @Summary      Submit a new emergency report
// @Description  Creates a location-tagged report under the caller's anonymous session. The
// @Description  anonymous session cookie is issued automatically on the first request; no signup
// @Description  or authentication is required. id, geohash, created_at and expires_at are computed
// @Description  server-side and can never be set by the client.
// @Tags         reports
// @Accept       json
// @Produce      json
// @Param        body  body      SubmitReportRequest  true  "Report to submit"
// @Success      201   {object}  SubmitReportResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /reports [post]
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

		var req SubmitReportRequest
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

		writeJSON(w, http.StatusCreated, SubmitReportResponse{Report: reportToResponse(report)})
	}
}

// NearbyReports handles GET /api/reports: parse lat/lon/radius_km from the
// query string, run the indexed bbox+Haversine query via svc, and return
// unexpired reports nearest-first. This is the one endpoint that serves
// both the map and the list (01-RESEARCH.md Pattern 3) — there is no
// separate map-only or list-only route.
//
// @Summary      List unexpired reports near a point
// @Description  Runs the indexed bounding-box prefilter followed by exact Haversine distance and
// @Description  returns unexpired reports within radius_km, nearest first. This single endpoint
// @Description  serves both the map and the list views — there is no separate map-only route.
// @Tags         reports
// @Produce      json
// @Param        lat        query     number  true   "Latitude of the query center (required, -90 to 90)"
// @Param        lon        query     number  true   "Longitude of the query center (required, -180 to 180)"
// @Param        radius_km  query     number  false  "Search radius in kilometers (defaults to 10, bounded to 0.1-50)" minimum(0.1) maximum(50) default(10)
// @Success      200  {object}  ReportListResponse
// @Failure      400  {object}  ErrorResponse
// @Router       /reports [get]
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

		responses := make([]ReportResponse, 0, len(reports))
		for _, rep := range reports {
			responses = append(responses, reportToResponse(rep))
		}
		writeJSON(w, http.StatusOK, ReportListResponse{Reports: responses})
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
	writeJSON(w, status, ErrorResponse{Error: ErrorDetail{Field: field, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("handlers: writing JSON response: %v", err)
	}
}
