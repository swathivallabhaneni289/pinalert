package geocode

// The reverse lookup that names the area of a report for the lock screen
// (Phase 3.3, D-16 and D-19). It exists only so a phone alert can read
// "Flooded road near Central Market" instead of a bare distance.
//
// Rules this file keeps:
//   - It is a method on the one process wide Client and shares its limiters, so
//     the one request a second ceiling of Phase 7 D-02 holds for every caller
//     together (T-07-01). It never builds a client or a limiter of its own.
//   - It takes a limiter token only when one is free right now (Allow, never
//     Wait) and gives up after ReverseTimeout, so it can never queue ahead of a
//     person's address search (Phase 7 D-04 and D-16) and never slows posting.
//   - It keeps only coarse names: a locality, a district, a neighbourhood, a
//     village, a town or a city. A road, a house number or a postcode is never
//     read, let alone returned.
//   - The dispatcher calls it once per report after the report is saved. A
//     failure leaves the name empty and the notification then shows the
//     distance only. There is no cache: one lookup per report.
//   - The base addresses are constants, so no request or report can choose the
//     host, and no error carries the coordinates (T-03.3-13).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// photonReverseURL and nominatimReverseURL are the only addresses a reverse
	// lookup ever calls.
	photonReverseURL    = "https://photon.komoot.io/reverse"
	nominatimReverseURL = "https://nominatim.openstreetmap.org/reverse"

	// ReverseTimeout is the whole time one ReverseArea call may take, both
	// providers together.
	ReverseTimeout = 2 * time.Second

	// ReverseAreaMaxRunes is the longest name ReverseArea returns.
	ReverseAreaMaxRunes = 120

	// reversePhotonShare is the most of ReverseTimeout Photon may use, so a
	// Photon that hangs still leaves time for the Nominatim fallback.
	reversePhotonShare = 1200 * time.Millisecond

	// reverseBodyLimit bounds how much of an answer is read.
	reverseBodyLimit = 1 << 20
)

// ErrNoArea means the providers answered but gave no coarse name for the spot:
// no feature, or only a road, a house number or a postcode.
var ErrNoArea = errors.New("geocode: no coarse area name for the spot")

// errBadReverseCoordinates is returned before any request for a latitude or a
// longitude that is out of range or not a number.
var errBadReverseCoordinates = errors.New("geocode: coordinates out of range")

// photonReverseLayers are the only layers asked of Photon, so a street or a
// house never comes back.
var photonReverseLayers = []string{"locality", "district", "city"}

// nominatimCoarseKeys are the address parts of a Nominatim reverse answer that
// may name an area, in the order they are preferred. road, house_number and
// postcode are deliberately not here.
var nominatimCoarseKeys = []string{"suburb", "neighbourhood", "quarter", "village", "town", "city"}

// nominatimReverseAnswer is the part of a Nominatim reverse answer that is
// read. Every other field is left out.
type nominatimReverseAnswer struct {
	Address map[string]string `json:"address"`
}

// ReverseArea names the area of a spot: a locality, district, neighbourhood,
// village, town or city, at most ReverseAreaMaxRunes long. Photon is asked
// first and Nominatim when Photon has no token free, fails or finds no name.
//
// A provider is asked only when its limiter has a token free right now. When
// neither has, the answer is ErrReaderBusy at once. When the providers answered
// but no coarse name was found, the answer is ErrNoArea. Any error is free of
// the coordinates.
func (c *Client) ReverseArea(ctx context.Context, lat, lon float64) (string, error) {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) ||
		lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return "", errBadReverseCoordinates
	}
	ctx, cancel := context.WithTimeout(ctx, ReverseTimeout)
	defer cancel()

	if c.photonReverse != "" && c.photonLimiter.Allow() {
		photonCtx, photonCancel := context.WithTimeout(ctx, reversePhotonShare)
		name, err := c.reversePhoton(photonCtx, lat, lon)
		photonCancel()
		if err == nil && name != "" {
			return name, nil
		}
		// A Photon failure or an empty answer goes on to Nominatim.
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !c.limiter.Allow() {
		return "", ErrReaderBusy
	}
	name, err := c.reverseNominatim(ctx, lat, lon)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", ErrNoArea
	}
	return name, nil
}

// reversePhoton asks Photon for the coarse layers around the spot. It returns
// an empty name, and no error, when Photon answered without a usable one.
func (c *Client) reversePhoton(ctx context.Context, lat, lon float64) (string, error) {
	params := url.Values{}
	params.Set("lat", formatReverseCoordinate(lat))
	params.Set("lon", formatReverseCoordinate(lon))
	params.Set("lang", photonLang)
	params.Set("limit", "1")
	params["layer"] = photonReverseLayers

	var answer photonResponse
	if err := c.getReverse(ctx, c.photonReverse, params, &answer); err != nil {
		return "", err
	}
	for _, feature := range answer.Features {
		if name := photonReverseName(feature.Properties); name != "" {
			return name, nil
		}
	}
	return "", nil
}

// photonReverseName is the coarse name of a Photon row: its own name when it is
// a locality, district or city sized row, else its locality, district or city,
// in that order. The street, house number and postcode are never read.
func photonReverseName(p photonProperties) string {
	candidates := []string{p.Locality, p.District, p.City}
	if photonAreaTypes[strings.ToLower(strings.TrimSpace(p.Type))] {
		candidates = append([]string{p.Name}, candidates...)
	}
	for _, raw := range candidates {
		if name := finishAreaName(raw); name != "" {
			return name
		}
	}
	return ""
}

// reverseNominatim asks Nominatim for the address of the spot at a coarse zoom
// and keeps only its coarse parts.
func (c *Client) reverseNominatim(ctx context.Context, lat, lon float64) (string, error) {
	params := url.Values{}
	params.Set("lat", formatReverseCoordinate(lat))
	params.Set("lon", formatReverseCoordinate(lon))
	params.Set("format", "jsonv2")
	params.Set("zoom", "14")
	params.Set("addressdetails", "1")
	if c.contactEmail != "" {
		params.Set("email", c.contactEmail)
	}

	var answer nominatimReverseAnswer
	if err := c.getReverse(ctx, c.nominatimReverse, params, &answer); err != nil {
		return "", err
	}
	return finishAreaName(coarseNominatimName(answer.Address)), nil
}

// coarseNominatimName is the first non empty coarse part of a Nominatim
// address: suburb, neighbourhood, quarter, village, town, then city. A road, a
// house number or a postcode is never taken.
func coarseNominatimName(address map[string]string) string {
	for _, key := range nominatimCoarseKeys {
		if part := cleanPart(address[key]); part != "" {
			return part
		}
	}
	return ""
}

// getReverse sends one GET to base with params and decodes the JSON answer into
// out. Every error is free of the request URL, which holds the coordinates.
func (c *Client) getReverse(ctx context.Context, base string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("geocode: building reverse request: %w", scrubURLError(err))
	}
	req.Header.Set("User-Agent", c.userAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("geocode: executing reverse request: %w", scrubURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &UpstreamStatusError{Status: resp.StatusCode}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, reverseBodyLimit)).Decode(out); err != nil {
		return fmt.Errorf("geocode: decoding reverse response: %w", scrubURLError(err))
	}
	return nil
}

// finishAreaName cleans a name and cuts it to ReverseAreaMaxRunes at a word
// end. A name with no letter or digit in it is no name and comes back empty.
func finishAreaName(raw string) string {
	name := cleanPart(raw)
	if utf8.RuneCountInString(name) > ReverseAreaMaxRunes {
		runes := []rune(name)
		cut := runes[:ReverseAreaMaxRunes]
		if !unicode.IsSpace(runes[ReverseAreaMaxRunes]) {
			if i := strings.LastIndexFunc(string(cut), unicode.IsSpace); i > 0 {
				cut = []rune(string(cut)[:i])
			}
		}
		name = strings.TrimRightFunc(string(cut), func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsPunct(r)
		})
	}
	if strings.IndexFunc(name, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) < 0 {
		return ""
	}
	return name
}

// formatReverseCoordinate writes a coordinate as a plain decimal.
func formatReverseCoordinate(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
