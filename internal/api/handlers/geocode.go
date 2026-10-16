// geocode.go is the HTTP surface of the address-search feature: it
// validates the q query parameter, calls the geocoding searcher, and shapes
// the response. The process wide ceilings on Photon and Nominatim live in
// internal/geocode. The person's own budget is checked here, keyed by the
// verified account like the place search's (quick 261016-sne, D-16): carrier
// NAT, or a host behind a proxy without CLIENT_IP_HEADER, puts many people
// behind one address, so a per IP budget would make them share it.
package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"pinalert/internal/account"
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
	Search(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error)
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
	// MatchedQuery is the shorter text that found these results when the
	// whole text found nothing. It is left out when the text as typed
	// matched, and when nothing matched.
	MatchedQuery string `json:"matched_query,omitempty" example:"Banjara Hills, Hyderabad"`
}

// geocodeAnswer shapes a search answer field by field. Results is a non nil
// slice, so an empty result set marshals as [] and not null, which is what
// drives the browser's "No matches found." path (D-04).
func geocodeAnswer(answer geocode.Answer) GeocodeResponse {
	results := make([]GeocodeResult, 0, len(answer.Results))
	for _, res := range answer.Results {
		results = append(results, GeocodeResult{
			Name:        res.Name,
			DisplayName: res.DisplayName,
			Lat:         res.Lat,
			Lon:         res.Lon,
		})
	}
	return GeocodeResponse{Results: results, MatchedQuery: answer.MatchedQuery}
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
// @Description Searches Photon first, which matches close spellings and half typed words, and
// @Description returns at most five matches, in India only. Requires a session verified by email
// @Description through the magic-link flow. The optional lat and lon (both or neither, the report
// @Description pin) make Photon prefer nearby places. When Photon finds nothing the search asks
// @Description Nominatim with the whole text, preferring places near the pin, then Photon again
// @Description without the first part and then without the second; matched_query then holds the
// @Description shorter text that matched. A first Photon request that takes over 2 seconds is
// @Description sent once more, with 2.5 seconds. When Photon fails, or is too slow both times,
// @Description Nominatim alone runs those tries, in a box around the pin first. At most four upstream requests are
// @Description sent per search. Each verified account has its own budget of 8 searches at once,
// @Description then 1 a second; past it the answer is a 429. A
// @Description 503 means search is temporarily unavailable; report submission remains fully
// @Description usable regardless.
// @Tags geocode
// @Produce json
// @Param q query string true "Place or address text, 3 to 200 characters"
// @Param lat query number false "Report pin latitude, -90 to 90, sent together with lon"
// @Param lon query number false "Report pin longitude, -180 to 180, sent together with lat"
// @Success 200 {object} GeocodeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /geocode [get]
func Geocode(searcher GeocodeSearcher, budget PlaceBudget) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// A request with no verified account is a programming error (a route
		// mounted outside the gated group), the same shape GeocodePlace uses.
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// The budget is keyed by the verified account, never the IP, and
		// under a key of its own, so it never shares the place search's.
		if budget != nil && !budget.Allow(fmt.Sprintf("geocode:%d", acc.ID)) {
			writeFieldError(w, http.StatusTooManyRequests, "q", placeRateLimitedMessage)
			return
		}

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

		// The report pin, both or neither, checked like the place search's
		// map centre. A bad value is refused before any upstream call.
		near, ok := parsePlaceCentre(w, r)
		if !ok {
			return
		}

		answer, err := searcher.Search(r.Context(), q, near)
		if err != nil {
			// The error class only: the error text could carry the search
			// text and, now, the area around the pin.
			log.Printf("handlers: Geocode: search failed (%s)", placeErrorClass(err))
			writeFieldError(w, http.StatusServiceUnavailable, "q", geocodeUnavailableMessage)
			return
		}

		// Set before writeJSON, since writeJSON calls WriteHeader.
		w.Header().Set("Cache-Control", geocodeCacheControl)
		writeJSON(w, http.StatusOK, geocodeAnswer(answer))
	}
}
