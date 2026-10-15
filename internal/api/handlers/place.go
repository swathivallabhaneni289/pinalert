// place.go is the HTTP surface of the reader's place search (HIST-05, D-14):
// the search box on the history views that jumps the map to another area. It
// is deliberately a separate handler from Geocode in geocode.go, which is the
// report form's address search and stays unchanged. The reader lane uses its
// own non waiting path on the shared geocoder limiter, its own cache, its own
// per account budget and its own messages, so a reader browsing another area
// can never queue in front of a reporter filing an emergency.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"pinalert/internal/account"
	"pinalert/internal/geocode"
)

// PlaceSearcher is the single-method interface GeocodePlace depends on, for
// the same reason GeocodeSearcher exists: the busy and failure paths are unit
// tested here without a live upstream. *geocode.Client satisfies it.
type PlaceSearcher interface {
	SearchReader(ctx context.Context, q string, near *geocode.Point) ([]geocode.Result, error)
}

// PlaceBudget is the per account search budget. *ratelimit.PerIP satisfies it
// and is keyed by any string, so the key is built from the verified account
// id and never from the client address (carrier NAT shares one IP across many
// phones, D-16, D-28).
type PlaceBudget interface {
	Allow(key string) bool
}

// placeUnavailableMessage and placeRateLimitedMessage are the UI-SPEC C31 and
// C32 copy, verbatim.
const (
	placeUnavailableMessage = "Search unavailable. Try again in a moment."
	placeRateLimitedMessage = "Too many searches. Wait a few seconds and try again."
)

// parsePlaceCentre reads the optional lat and lon query parameters, the map
// centre used as a non bounded bias. Both or neither: it returns nil for
// neither, and on a bad value it writes the 400 itself and returns ok=false.
func parsePlaceCentre(w http.ResponseWriter, r *http.Request) (*geocode.Point, bool) {
	query := r.URL.Query()
	hasLat, hasLon := query.Has("lat"), query.Has("lon")
	if !hasLat && !hasLon {
		return nil, true
	}
	if !hasLat {
		writeFieldError(w, http.StatusBadRequest, "lat", "Send both lat and lon, or neither.")
		return nil, false
	}
	if !hasLon {
		writeFieldError(w, http.StatusBadRequest, "lon", "Send both lat and lon, or neither.")
		return nil, false
	}

	lat, err := strconv.ParseFloat(query.Get("lat"), 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		writeFieldError(w, http.StatusBadRequest, "lat", "Latitude must be a number between -90 and 90.")
		return nil, false
	}
	lon, err := strconv.ParseFloat(query.Get("lon"), 64)
	if err != nil || math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 {
		writeFieldError(w, http.StatusBadRequest, "lon", "Longitude must be a number between -180 and 180.")
		return nil, false
	}
	return &geocode.Point{Lat: lat, Lon: lon}, true
}

// placeErrorClass names a failed search in a few fixed words for the log: no
// error text, so nothing the reader typed or any coordinate can reach it.
func placeErrorClass(err error) string {
	var status *geocode.UpstreamStatusError
	switch {
	case errors.Is(err, geocode.ErrReaderBusy):
		return "reader lane busy"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.As(err, &status):
		return "upstream status " + strconv.Itoa(status.Status)
	default:
		return "upstream error"
	}
}

// GeocodePlace produces the HTTP handler for GET /api/geocode/place.
//
// @Summary Search for a place on the reader lane
// @Description Place search for the history views. It runs on its own lane of the geocoder: it
// @Description never waits for a token, repeated searches are answered from a cache, and each
// @Description verified account has its own small budget. The optional lat and lon (both or
// @Description neither) bias results toward the map centre without hiding matches elsewhere.
// @Description A 429 or 503 never affects report submission, which has its own search.
// @Tags geocode
// @Produce json
// @Param q query string true "Place text, 3 to 200 characters"
// @Param lat query number false "Map centre latitude, -90 to 90, sent together with lon"
// @Param lon query number false "Map centre longitude, -180 to 180, sent together with lat"
// @Success 200 {object} GeocodeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /geocode/place [get]
func GeocodePlace(searcher PlaceSearcher, budget PlaceBudget) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// A request with no verified account is a programming error (a route
		// mounted outside the gated group), not a client condition, the same
		// shape NearbyReports uses.
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// The budget is keyed by the verified account, never the IP.
		if budget != nil && !budget.Allow(fmt.Sprintf("place:%d", acc.ID)) {
			writeFieldError(w, http.StatusTooManyRequests, "q", placeRateLimitedMessage)
			return
		}

		q, ok := parseGeocodeQuery(w, r)
		if !ok {
			return
		}

		near, ok := parsePlaceCentre(w, r)
		if !ok {
			return
		}

		if searcher == nil {
			log.Printf("handlers: GeocodePlace: %v", "nil searcher")
			writeFieldError(w, http.StatusServiceUnavailable, "q", placeUnavailableMessage)
			return
		}

		found, err := searcher.SearchReader(r.Context(), q, near)
		if err != nil {
			// A fixed message and the error class only. The error text can
			// carry the request URL, and that holds the reader's search
			// text, their map centre and the contact email.
			log.Printf("handlers: GeocodePlace: search failed (%s)", placeErrorClass(err))
			writeFieldError(w, http.StatusServiceUnavailable, "q", placeUnavailableMessage)
			return
		}

		// A non nil empty slice, so an empty result set marshals as [].
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
