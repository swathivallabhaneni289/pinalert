// geocode.go is the HTTP surface of the address-search feature: it
// validates the q query parameter, calls the geocoding searcher, and shapes
// the response. No rate-limiting logic lives here — the global 1 req/sec
// policy ceiling lives in internal/geocode, and the per-IP anti-abuse
// throttle is applied at route registration in internal/api/router.go
// (plan 07-03).
package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"pinalert/internal/geocode"
)

// GeocodeSearcher is the single-method interface handlers.Geocode depends
// on rather than the concrete *geocode.Client every sibling handler in this
// file would otherwise accept (e.g. *service.VotingService,
// *service.ReportService). This file is the codebase's first dependency
// that talks to a third-party network service rather than to Postgres, and
// the interface is what lets the upstream-failure path required by D-04 be
// unit tested here without a live upstream and without adding exported
// test-only surface to internal/geocode. *geocode.Client satisfies this
// interface unchanged.
type GeocodeSearcher interface {
	Search(ctx context.Context, q string) ([]geocode.Result, error)
}

// minGeocodeQueryRunes and maxGeocodeQueryRunes bound the q parameter,
// counted with utf8.RuneCountInString rather than len: this is an
// India-facing app, and byte counting would let a 2-character Devanagari
// query through the minimum while rejecting a perfectly ordinary
// 67-character Devanagari place name at the maximum.
const (
	minGeocodeQueryRunes = 3
	maxGeocodeQueryRunes = 200
)

// geocodeUnavailableMessage is D-04's copy, verbatim. web/static/js/modal.js
// (plan 07-04) uses the same sentence as its own client-side fallback, so a
// visitor sees one wording whether the failure originated upstream or in
// the browser's own fetch.
const geocodeUnavailableMessage = "Search unavailable, try tapping the map instead."

// geocodeCacheControl asks the browser to cache a successful response.
// Nominatim's usage policy asks callers to cache; this is the response-side
// half of that ask, the client-side same-query cache in plan 07-04 being
// the other half.
const geocodeCacheControl = "private, max-age=300"

// GeocodeResult is a handler-local shape mapped field by field from
// geocode.Result, not a re-marshalled upstream type, for the same two
// reasons CastVoteResponse is handler-local: swag is invoked without
// --parseDependency (see the swag target in the Makefile) so a cross-package
// type would not be documented, and shaping every outbound response
// explicitly is this package's standing rule.
type GeocodeResult struct {
	Name        string  `json:"name" example:"Bengaluru"`
	DisplayName string  `json:"display_name" example:"Bengaluru, Bangalore North, Bengaluru Urban, Karnataka, India"`
	Lat         float64 `json:"lat" example:"12.9767936"`
	Lon         float64 `json:"lon" example:"77.5900820"`
}

// GeocodeResponse is the JSON body of a successful GET /api/geocode
// response.
type GeocodeResponse struct {
	Results []GeocodeResult `json:"results"`
}

// parseGeocodeQuery reads and validates the q query parameter, following
// parseReportID's write-the-error-yourself contract: on an out-of-range
// query it writes the 400 response itself and returns ok=false.  Returning
// false before any outbound call is the point: a rejected query costs
// Nominatim nothing (T-07-05).
func parseGeocodeQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	runeCount := utf8.RuneCountInString(q)
	if runeCount < minGeocodeQueryRunes || runeCount > maxGeocodeQueryRunes {
		writeFieldError(w, http.StatusBadRequest, "q", "Search text must be between 3 and 200 characters.")
		return "", false
	}
	return q, true
}

// Geocode produces the HTTP handler for GET /api/geocode.
//
// @Summary Search for a place or address by free-text query
// @Description Proxies a search query to Nominatim and returns at most five matches. Requires a
// @Description session verified by email through the magic-link flow. A 503 means search is
// @Description temporarily unavailable; report submission remains fully usable regardless.
// @Tags geocode
// @Produce json
// @Param q query string true "Place or address text, 3 to 200 characters"
// @Success 200 {object} GeocodeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /geocode [get]
func Geocode(searcher GeocodeSearcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if searcher == nil {
			// A Deps{} literal that omits the dependency (e.g. a test that
			// doesn't need geocoding) must degrade to the D-04 message
			// instead of panicking.
			log.Printf("handlers: Geocode: %v", "nil searcher")
			writeFieldError(w, http.StatusServiceUnavailable, "q", geocodeUnavailableMessage)
			return
		}

		q, ok := parseGeocodeQuery(w, r)
		if !ok {
			return
		}

		found, err := searcher.Search(r.Context(), q)
		if err != nil {
			log.Printf("handlers: Geocode: %v", err)
			writeFieldError(w, http.StatusServiceUnavailable, "q", geocodeUnavailableMessage)
			return
		}

		// A non-nil empty slice, so an empty result set marshals as [] and
		// not null, which is what drives the browser's "No matches found."
		// path (D-04).
		results := make([]GeocodeResult, 0, len(found))
		for _, res := range found {
			results = append(results, GeocodeResult{
				Name:        res.Name,
				DisplayName: res.DisplayName,
				Lat:         res.Lat,
				Lon:         res.Lon,
			})
		}

		// Set before writeJSON, since writeJSON calls WriteHeader.
		w.Header().Set("Cache-Control", geocodeCacheControl)
		writeJSON(w, http.StatusOK, GeocodeResponse{Results: results})
	}
}
