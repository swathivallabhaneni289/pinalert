// place.go is the HTTP surface of the reader's place search (HIST-05, D-14):
// the search box on the history views that jumps the map to another area. It
// is deliberately a separate handler from Geocode in geocode.go, which is the
// report form's address search. Both run the same search in internal/geocode:
// Photon first, then Nominatim's India only, near first, fewer words plan as
// the fallback. The reader lane uses its own non waiting path on both shared
// geocoder limiters, its own Nominatim cache, its own
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
	SearchReader(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error)
	// SearchReaderAreas is SearchReader restricted to areas (neighbourhoods,
	// localities, towns, cities, districts), for kind=area.
	SearchReaderAreas(ctx context.Context, q string, near *geocode.Point) (geocode.Answer, error)
}

// placeKindArea is the one value of the optional kind parameter: the home area
// fields ask for areas only. No kind is the place search as it always was.
const placeKindArea = "area"

// PlaceResponse is the JSON body of a successful GET /api/geocode/place. It is
// GeocodeResponse and one more field, set only for kind=area (plan 34).
type PlaceResponse struct {
	GeocodeResponse
	// NoMatch is true when no row closely matches what was typed and no typed
	// row could be placed, so the page asks the person to add their town. Left
	// out otherwise, and always left out without kind=area.
	NoMatch bool `json:"no_match,omitempty" example:"true"`
	// Unfinished is true when no row closely matches what was typed but a
	// lookup the search needed failed or timed out (plan 35), so Not found
	// cannot be claimed. Never true together with no_match. Left out otherwise,
	// and always left out without kind=area.
	Unfinished bool `json:"unfinished,omitempty" example:"true"`
}

// parsePlaceKind reads the optional kind query parameter. It returns areas=true
// for kind=area and false when kind is absent or empty. Any other value writes
// the 400 itself and returns ok=false.
func parsePlaceKind(w http.ResponseWriter, r *http.Request) (areas, ok bool) {
	switch r.URL.Query().Get("kind") {
	case "":
		return false, true
	case placeKindArea:
		return true, true
	}
	writeFieldError(w, http.StatusBadRequest, "kind", "Kind must be area, or left out.")
	return false, false
}

// PlaceBudget is the per account search budget of both searches, each with
// its own limiter and key prefix (quick 261016-sne). *ratelimit.PerIP
// satisfies it and is keyed by any string, so the key is built from the
// verified account id and never from the client address (carrier NAT shares
// one IP across many phones, D-16, D-28).
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
// centre the search looks around first. Both or neither: it returns nil for
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
// @Description never queues ahead of a report search, repeated searches are answered from a cache, and each
// @Description verified account has its own budget of 8 searches at once, then 1 a second. Results are in India only. It asks
// @Description Photon first, which matches close spellings and half typed words; the optional
// @Description lat and lon (both or neither, the map centre) make Photon prefer nearby places. With lat and lon,
// @Description only places within 50 km of the map centre are returned, and a city or town named as typed; a
// @Description search that finds only farther places returns an empty list, except with kind=area, which is never
// @Description cut by distance; with them a shorter text of one word is not tried. Without them the search covers all
// @Description of India. When
// @Description Photon finds nothing it asks Nominatim with the whole text, preferring places near
// @Description the map centre, then Photon again without the first part and then without the
// @Description second; matched_query then holds the shorter text that matched. A first Photon
// @Description request that takes over 2 seconds is sent once more, with 2.5 seconds. When Photon
// @Description fails, or is too slow both times, Nominatim alone runs those tries, in a box around
// @Description the map centre first. At most four upstream requests are sent per search.
// @Description With kind=area the text is sent without a trailing PIN code, state name or country, and when no area in the
// @Description town the person typed (the second comma separated part) closely matches the first part, the answer starts with
// @Description one extra row marked typed true, the typed text placed in that town; its lookups count inside the same cap of
// @Description four upstream requests. A first part with a digit or a street word never gets that row.
// @Description With kind=area the town may also follow the neighbourhood without a comma (the last one or two words are tried as the
// @Description town), or be named by a six digit PIN code in the text when no town is found; those lookups count inside the same cap.
// @Description no_match is true when no row closely matches what was typed, no typed row could be placed, and every lookup the search needed answered; the page asks for the town.
// @Description unfinished is true instead when a lookup that was needed failed or timed out, so Not found is not claimed; the lookups fall back to Nominatim inside the same cap.
// @Description A 429 or 503 never affects report submission, which has its own search.
// @Tags geocode
// @Produce json
// @Param q query string true "Place text, 3 to 200 characters"
// @Param lat query number false "Map centre latitude, -90 to 90, sent together with lon"
// @Param lon query number false "Map centre longitude, -180 to 180, sent together with lat"
// @Param kind query string false "Send area to get areas only (neighbourhoods, localities, towns, cities, districts), never a street or a building. Left out, every kind of place is returned" Enums(area)
// @Success 200 {object} PlaceResponse
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

		areas, ok := parsePlaceKind(w, r)
		if !ok {
			return
		}

		if searcher == nil {
			log.Printf("handlers: GeocodePlace: %v", "nil searcher")
			writeFieldError(w, http.StatusServiceUnavailable, "q", placeUnavailableMessage)
			return
		}

		search := searcher.SearchReader
		if areas {
			search = searcher.SearchReaderAreas
		}
		answer, err := search(r.Context(), q, near)
		if err != nil {
			// A fixed message and the error class only. The error text can
			// carry the request URL, and that holds the reader's search
			// text, their map centre and the contact email.
			log.Printf("handlers: GeocodePlace: search failed (%s)", placeErrorClass(err))
			writeFieldError(w, http.StatusServiceUnavailable, "q", placeUnavailableMessage)
			return
		}

		// Set before writeJSON, since writeJSON calls WriteHeader.
		w.Header().Set("Cache-Control", geocodeCacheControl)
		writeJSON(w, http.StatusOK, PlaceResponse{
			GeocodeResponse: geocodeAnswer(answer),
			NoMatch:         areas && answer.NoCloseMatch,
			Unfinished:      areas && answer.Unfinished,
		})
	}
}
