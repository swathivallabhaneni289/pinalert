package geocode

// Photon (https://photon.komoot.io, by komoot) is the first provider for both
// searches since quick task 261016-pho. It is built on OpenStreetMap data like
// Nominatim, needs no key, and, unlike Nominatim, matches close spellings and
// half typed words and allows search as you type. Nominatim stays as the
// fallback for when Photon fails, times out or finds nothing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// photonSearchURL is the only Photon URL this package calls. Like
// nominatimSearchURL it is a constant, so the proxy can never be pointed
// anywhere a caller chooses (T-07-01).
const photonSearchURL = "https://photon.komoot.io/api/"

// Photon tuning.
const (
	// photonLimiterEvery and photonLimiterBurst cap Photon at two requests a
	// second across the whole process. Photon's public server asks only for
	// a reasonable number of requests and is meant for search as you type;
	// one person typing with the browser's 600 ms pause never waits, and many
	// people typing at once cannot flood it.
	photonLimiterEvery = 500 * time.Millisecond
	photonLimiterBurst = 2
	// photonLimiterWait bounds how long a report search waits for a Photon
	// token. When it runs out the search goes to Nominatim instead.
	photonLimiterWait = 500 * time.Millisecond
	// defaultPhotonTimeout bounds one Photon request. A slow Photon must not
	// eat the time the Nominatim fallback needs inside the report form's
	// 8 second limit.
	defaultPhotonTimeout = 2 * time.Second
	// photonFetchLimit asks for more than are shown, so the India filter and
	// the ranking still leave five.
	photonFetchLimit = "10"
	// photonLang asks for English names where OpenStreetMap has them, so the
	// labels read like the Nominatim ones. Text typed in another script still
	// matches.
	photonLang = "en"
	// indiaBBox is minimum longitude, minimum latitude, maximum longitude,
	// maximum latitude around India. It also covers parts of Pakistan, Nepal,
	// Bangladesh, Sri Lanka and their neighbours, so every answer is also
	// filtered on its country code.
	indiaBBox = "68.0,6.4,97.5,37.1"
	// indiaCountryCode is the country code an answer must carry.
	indiaCountryCode = "IN"
	// maxResults is how many matches a search returns.
	maxResults = 5
)

// Which provider gave an answer. Kept inside this package for the live check
// and the tests; the handlers map answers field by field, so it never reaches
// the JSON.
const (
	providerPhoton    = "photon"
	providerNominatim = "nominatim"
)

// photonResponse is the part of Photon's GeoJSON answer this package reads.
// Every other field (osm_id, extent, postcode and the rest) is left out, so it
// can never be forwarded to a caller (T-07-04).
type photonResponse struct {
	Features []photonFeature `json:"features"`
}

type photonFeature struct {
	Geometry struct {
		// GeoJSON order: longitude first, then latitude.
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties photonProperties `json:"properties"`
}

type photonProperties struct {
	Name        string `json:"name"`
	HouseNumber string `json:"housenumber"`
	Street      string `json:"street"`
	Locality    string `json:"locality"`
	District    string `json:"district"`
	City        string `json:"city"`
	State       string `json:"state"`
	CountryCode string `json:"countrycode"`
	OSMKey      string `json:"osm_key"`
}

// placeOSMKeys are the OpenStreetMap keys that make an answer a place rather
// than a point of interest: towns, areas and districts, roads, named
// neighbourhoods and houses. Shops, amenities, offices and the rest are
// points of interest.
var placeOSMKeys = map[string]bool{
	"place":    true,
	"boundary": true,
	"highway":  true,
	"landuse":  true,
	"building": true,
}

// cleanPart trims a label part and collapses its spaces. A part that is empty
// or the text null is dropped.
func cleanPart(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if strings.EqualFold(s, "null") {
		return ""
	}
	return s
}

// photonLabel builds the full label from the name, house number, street,
// locality, district, city and state, in that order. Missing parts are
// skipped and a part already in the label (in any case) is not repeated, so
// there is never a "null", an empty comma or "Hyderabad, Hyderabad".
func photonLabel(p photonProperties) string {
	var parts []string
	for _, raw := range []string{p.Name, p.HouseNumber, p.Street, p.Locality, p.District, p.City, p.State} {
		part := cleanPart(raw)
		if part == "" {
			continue
		}
		repeated := false
		for _, have := range parts {
			if strings.EqualFold(have, part) {
				repeated = true
				break
			}
		}
		if !repeated {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ", ")
}

// normalizeSpelling lower cases text, turns punctuation into spaces and
// collapses spaces, so "Road No. 12" and "road no 12" compare equal.
func normalizeSpelling(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return ' '
		}
		return unicode.ToLower(r)
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// allowedEdits is how many letters may differ for a text of n letters to
// still count as a close spelling: none under five letters, one up to eight,
// two beyond. Short texts need an exact name, or every three letter word
// would be close to dozens of places.
func allowedEdits(n int) int {
	switch {
	case n < 5:
		return 0
	case n <= 8:
		return 1
	default:
		return 2
	}
}

// editDistance is the Levenshtein distance between a and b, counted in
// letters (runes), so Telugu and Devanagari text is measured correctly.
func editDistance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// closeSpelling reports whether name is the typed text q spelled the same or
// within allowedEdits letters of it, ignoring case, punctuation and spacing.
func closeSpelling(q, name string) bool {
	nq, nn := normalizeSpelling(q), normalizeSpelling(name)
	if nq == "" || nn == "" {
		return false
	}
	allowed := allowedEdits(utf8.RuneCountInString(nq))
	rq, rn := []rune(nq), []rune(nn)
	if diff := len(rq) - len(rn); diff > allowed || -diff > allowed {
		return false
	}
	return editDistance(rq, rn) <= allowed
}

// photonCandidate is one usable Photon answer before ranking.
type photonCandidate struct {
	result Result
	place  bool
	name   string
}

// rankPhoton puts places whose name is a close spelling of the typed text
// first, then everything else, each group in Photon's own order, and keeps
// maxResults. Photon scores an object that carries the typed text exactly,
// typo included, above the place the person meant: live on 2026-10-16
// "Hyderbad" gave a shop named "Hyderbad" before Hyderabad. A point of
// interest whose name matches is never lifted, so an exact hospital name
// keeps the place Photon gave it.
//
// A row whose label reads the same (in any case) as one already kept is
// dropped: a city and its district, or one road split into several
// OpenStreetMap ways, would otherwise fill the list with rows nobody can
// tell apart.
func rankPhoton(q string, candidates []photonCandidate) []Result {
	results := make([]Result, 0, maxResults)
	seen := make(map[string]bool, len(candidates))
	for _, lifted := range []bool{true, false} {
		for _, c := range candidates {
			if (c.place && closeSpelling(q, c.name)) != lifted {
				continue
			}
			label := strings.ToLower(c.result.DisplayName)
			if seen[label] {
				continue
			}
			seen[label] = true
			results = append(results, c.result)
		}
	}
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	return results
}

// fetchPhoton sends one search to Photon and returns the usable Indian
// answers, ranked. It never touches a limiter; the caller has taken a token.
// The request has its own time limit, so a slow Photon ends in an error the
// caller can fall back from.
func (c *Client) fetchPhoton(ctx context.Context, q string, near *Point) ([]Result, error) {
	ctx, cancel := context.WithTimeout(ctx, c.photonTimeout)
	defer cancel()

	params := url.Values{}
	params.Set("q", q)
	params.Set("limit", photonFetchLimit)
	params.Set("lang", photonLang)
	params.Set("bbox", indiaBBox)
	if near != nil {
		params.Set("lat", strconv.FormatFloat(near.Lat, 'f', 6, 64))
		params.Set("lon", strconv.FormatFloat(near.Lon, 'f', 6, 64))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.photonURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("geocode: building photon request: %w", scrubURLError(err))
	}
	req.Header.Set("User-Agent", c.userAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocode: executing photon request: %w", scrubURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &UpstreamStatusError{Status: resp.StatusCode}
	}

	var body photonResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("geocode: decoding photon response: %w", err)
	}

	candidates := make([]photonCandidate, 0, len(body.Features))
	for _, f := range body.Features {
		p := f.Properties
		if !strings.EqualFold(strings.TrimSpace(p.CountryCode), indiaCountryCode) {
			continue
		}
		if len(f.Geometry.Coordinates) < 2 {
			continue
		}
		lon, lat := f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
		if math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
			// Skip, don't clamp: a bad row must never become a wrong pin.
			continue
		}
		label := photonLabel(p)
		if label == "" {
			continue
		}
		name := cleanPart(p.Name)
		candidates = append(candidates, photonCandidate{
			result: Result{Name: name, DisplayName: label, Lat: lat, Lon: lon},
			place:  placeOSMKeys[strings.TrimSpace(p.OSMKey)],
			name:   name,
		})
	}
	return rankPhoton(q, candidates), nil
}

// waitForPhotonToken is the report search's turn at Photon's limiter for its
// Photon try i. The first try waits at most photonLimiterWait, since
// Nominatim is right behind it. A shorter text's try (i > 0) waits up to
// limiterWaitTimeout like a Nominatim try: by then Photon is the best chance
// left, and at one Photon request a second its next token is most of a second
// away.
func (c *Client) waitForPhotonToken(ctx context.Context, i int) error {
	wait := photonLimiterWait
	if i > 0 {
		wait = limiterWaitTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if err := c.photonLimiter.Wait(waitCtx); err != nil {
		return fmt.Errorf("geocode: photon limiter wait: %w", err)
	}
	return nil
}

// takeReaderPhotonToken is the place search's turn at Photon's limiter for
// its Photon try i, by the reader rule (takeReaderTokenFrom): the first try
// takes a token only if one is free right now, a later one waits without
// reserving and gives up when a report search is waiting, so a reader never
// queues ahead of a report search (D-16).
func (c *Client) takeReaderPhotonToken(ctx context.Context, i int) error {
	return takeReaderTokenFrom(ctx, c.photonLimiter, i)
}

// photonOutcome is what asking Photon came to.
type photonOutcome struct {
	answer Answer
	found  bool // Photon answered with at least one match
	sent   int  // requests actually sent to Photon (0 or 1)
	failed bool // Photon errored, timed out or had no token: never cached
}

// askPhoton is one Photon try for text: its cache, its limiter and one
// request. An error is returned only when the search itself must end: the
// reader found no free token, or the caller went away. Any other Photon
// failure is reported as failed, and the search goes on to Nominatim.
func (c *Client) askPhoton(ctx context.Context, text string, near *Point, take func(ctx context.Context) error) (photonOutcome, error) {
	key := cacheKey(text, near)
	if cached, ok := c.photon.get(key); ok {
		// A cached empty answer means Photon found nothing here before.
		return photonOutcome{answer: cached, found: len(cached.Results) > 0}, nil
	}
	if err := take(ctx); err != nil {
		if errors.Is(err, ErrReaderBusy) || ctx.Err() != nil {
			return photonOutcome{}, err
		}
		// The report search waited its share for a token: treat Photon as
		// unavailable rather than make the person wait longer.
		return photonOutcome{failed: true}, nil
	}
	results, err := c.fetchPhoton(ctx, text, near)
	if err != nil {
		if ctx.Err() != nil {
			// The browser gave up or the server is shutting down: no
			// fallback request for an answer nobody will read.
			return photonOutcome{}, err
		}
		return photonOutcome{sent: 1, failed: true}, nil
	}
	answer := Answer{Results: results, provider: providerPhoton}
	c.photon.put(key, answer)
	return photonOutcome{answer: answer, found: len(results) > 0, sent: 1}, nil
}
