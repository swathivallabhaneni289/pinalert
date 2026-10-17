package geocode

// The typed row of the home area search (Phase 3.4 plan 32).
//
// OpenStreetMap lacks many Indian neighbourhoods as areas. The owner typed
// "Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008" and nothing came back
// that was her neighbourhood: OpenStreetMap holds only a street called
// Gayatri Nagar Main there. When no area in the typed town closely matches the
// first part of what was typed, the areas answer starts with one extra row, the
// typed text, placed in the typed town. The person keeps their own words.
//
// The position comes from, in order: an any place match in the same town whose
// name is a close spelling of the first part (or a street or place that starts
// with one), else the town itself from an areas search for the second part,
// else no typed row. The stored home cell is about 5 km wide, so a street or
// landmark point in the right town is as good as the neighbourhood's own point.
//
// A street address is never offered as a home area: a first part with a digit
// (a house or plot number) or a street word gets no typed row.
//
// Cost: the lookups send nothing the per search cap of maxUpstreamPerSearch has
// no room for. The cap is counted over the whole search, the areas search
// included (see withUpstreamCount), so the worst case stays four upstream
// requests. They go through askPhoton, so a repeat is answered from the cache,
// and take their Photon token by the reader rule, never ahead of a report
// search. A busy limiter or a failed lookup leaves the plain areas answer as it
// is; it is never an error for a search that already has results.

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// typedTownRadiusKm is how far from the typed town's point a place may lie and
// still count as in that town when its label does not name the town (Banjara
// Hills reads "Banjara Hills, Telangana", with no city).
const typedTownRadiusKm = 25.0

// homeAreaNameMaxRunes is the longest name the server stores as a home area
// (service.HomeAreaNameMaxRunes). This package cannot import service, so a test
// of the value lives in the handlers package.
const homeAreaNameMaxRunes = 120

// minTypedPartRunes is the shortest first part or town a typed row is made
// from, the same minimum the handlers apply to what people type.
const minTypedPartRunes = 3

// upstreamCountKey carries a search's request counter in its context.
type upstreamCountKey struct{}

type upstreamCount struct{ n atomic.Int32 }

// withUpstreamCount starts counting the upstream requests sent under ctx.
func withUpstreamCount(ctx context.Context) context.Context {
	return context.WithValue(ctx, upstreamCountKey{}, &upstreamCount{})
}

// noteUpstream counts one upstream request, when ctx is counting.
func noteUpstream(ctx context.Context) {
	if counter, ok := ctx.Value(upstreamCountKey{}).(*upstreamCount); ok {
		counter.n.Add(1)
	}
}

// upstreamUsed is how many upstream requests were sent under ctx so far.
func upstreamUsed(ctx context.Context) int {
	if counter, ok := ctx.Value(upstreamCountKey{}).(*upstreamCount); ok {
		return int(counter.n.Load())
	}
	return 0
}

// errTypedBudget says the per search cap leaves no room for another lookup.
var errTypedBudget = errors.New("geocode: no upstream request left in the search cap")

// indianStates are the names dropped from the end of what was typed. Delhi,
// Chandigarh and Puducherry are left out: each is also the name of the city
// people mean.
var indianStates = map[string]bool{
	"andhra pradesh": true, "arunachal pradesh": true, "assam": true, "bihar": true,
	"chhattisgarh": true, "goa": true, "gujarat": true, "haryana": true,
	"himachal pradesh": true, "jharkhand": true, "karnataka": true, "kerala": true,
	"madhya pradesh": true, "maharashtra": true, "manipur": true, "meghalaya": true,
	"mizoram": true, "nagaland": true, "odisha": true, "orissa": true, "punjab": true,
	"rajasthan": true, "sikkim": true, "tamil nadu": true, "telangana": true,
	"tripura": true, "uttar pradesh": true, "uttarakhand": true, "west bengal": true,
	"jammu and kashmir": true, "ladakh": true, "lakshadweep": true,
	"andaman and nicobar islands": true, "dadra and nagar haveli and daman and diu": true,
}

// streetWords mark a first part as a street or a building, not a neighbourhood.
var streetWords = map[string]bool{
	"road": true, "rd": true, "street": true, "lane": true, "marg": true, "gali": true,
	"avenue": true, "highway": true, "hwy": true, "flat": true, "apartment": true,
	"apartments": true, "building": true, "plot": true, "house": true, "floor": true,
}

// pinSuffix is a six digit PIN code at the end of a part, alone or after a space
// or a dash.
var pinSuffix = regexp.MustCompile(`(?:^|[\s-])[1-9][0-9]{5}$`)

// typedHome is what a home area search text comes to.
type typedHome struct {
	// search is the text the areas search is sent: the typed text without a
	// trailing PIN code, state name or country, in comma separated parts.
	search string
	// ok says a typed row may be offered.
	ok bool
	// name is the saved name of the typed row: the first two parts.
	name string
	// first is the part before the first comma, town the second part.
	first, town string
}

// typedParts splits text into trimmed, space collapsed, non empty parts.
func typedParts(text string) []string {
	var parts []string
	for _, part := range strings.Split(text, ",") {
		if part = strings.Join(strings.Fields(part), " "); part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// stripTrailing drops a trailing country, PIN code and state name, in any
// order, as a pasted address writes them ("..., Andhra Pradesh 520008, India").
// A bare PIN or state is kept when it is all that was typed, and a state is
// kept when dropping it would leave one part: then it is the town.
func stripTrailing(parts []string) []string {
	for len(parts) > 0 {
		last := parts[len(parts)-1]
		key := normalizeSpelling(last)
		switch {
		case len(parts) > 1 && key == "india":
			parts = parts[:len(parts)-1]
			continue
		case pinSuffix.MatchString(last):
			rest := strings.Trim(pinSuffix.ReplaceAllString(last, ""), " -")
			if rest != "" {
				parts[len(parts)-1] = rest
				continue
			}
			if len(parts) > 1 {
				parts = parts[:len(parts)-1]
				continue
			}
		case len(parts) > 2 && indianStates[key]:
			parts = parts[:len(parts)-1]
			continue
		}
		break
	}
	return parts
}

// parseTypedHome cleans the text of a home area search and says whether a typed
// row may be made from it.
func parseTypedHome(q string) typedHome {
	parts := stripTrailing(typedParts(q))
	home := typedHome{search: strings.Join(parts, ", ")}
	if len(parts) < 2 {
		return home
	}
	first, town := parts[0], parts[1]
	if utf8.RuneCountInString(first) < minTypedPartRunes || utf8.RuneCountInString(town) < minTypedPartRunes {
		return home
	}
	if looksLikeStreetAddress(first) || strings.IndexFunc(town, unicode.IsDigit) >= 0 {
		return home
	}
	name := first + ", " + town
	if utf8.RuneCountInString(name) > homeAreaNameMaxRunes {
		return home
	}
	home.ok, home.name, home.first, home.town = true, name, first, town
	return home
}

// looksLikeStreetAddress says a first part is a house, a plot, a flat or a
// street, which is never offered as a home area.
func looksLikeStreetAddress(first string) bool {
	if strings.IndexFunc(first, unicode.IsDigit) >= 0 {
		return true
	}
	for _, word := range strings.Fields(normalizeSpelling(first)) {
		if streetWords[word] {
			return true
		}
	}
	return false
}

// leadingNameMatches says name is a close spelling of first, or starts with
// words that are (Gayatri Nagar Main for Gayathri Nagar). A name that merely
// contains the words further in does not match.
func leadingNameMatches(first, name string) bool {
	if closeSpelling(first, name) {
		return true
	}
	want := len(strings.Fields(normalizeSpelling(first)))
	have := strings.Fields(normalizeSpelling(name))
	if want == 0 || len(have) <= want {
		return false
	}
	return closeSpelling(first, strings.Join(have[:want], " "))
}

// distanceKm is the great circle distance between two points.
func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

// typedLookup finds the position of a typed row, spending only what is left of
// the search's request cap.
type typedLookup struct {
	c       *Client
	ctx     context.Context
	near    *Point
	home    typedHome
	areas   []Result // the areas answer already in hand
	town    *Result  // the town's own row, once looked for
	townTry bool
}

// withTypedRow returns answer with the typed row first, or answer itself when
// the typed text needs none or no position is found.
func (c *Client) withTypedRow(ctx context.Context, home typedHome, near *Point, answer Answer) Answer {
	lk := &typedLookup{c: c, ctx: ctx, near: near, home: home, areas: answer.Results}
	if lk.areaInTown() {
		return answer
	}
	lat, lon, ok := lk.position()
	if !ok {
		return answer
	}
	row := Result{Name: home.name, DisplayName: home.town, Lat: lat, Lon: lon, Typed: true, Town: home.town}
	results := append([]Result{row}, answer.Results...)
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	answer.Results = results
	return answer
}

// photon asks Photon for text through askPhoton: its cache first, then a token
// by the reader rule, and only while the search's request cap has room. It
// returns nothing on any failure. anyPlace lifts the areas only restriction.
func (lk *typedLookup) photon(text string, anyPlace bool) []Result {
	if lk.ctx.Err() != nil {
		return nil
	}
	ctx := lk.ctx
	if anyPlace {
		ctx = context.WithValue(ctx, areasOnlyKey{}, false)
	}
	take := func(ctx context.Context) error {
		if upstreamUsed(ctx) >= maxUpstreamPerSearch {
			return errTypedBudget
		}
		return lk.c.takeReaderPhotonToken(ctx, 1)
	}
	out, err := lk.c.askPhoton(ctx, text, lk.near, take, lk.c.photonTimeout)
	if err != nil || out.failed {
		return nil
	}
	return out.answer.Results
}

// labelNamesTown says one comma separated part of the row's label is a close
// spelling of the typed town.
func (lk *typedLookup) labelNamesTown(r Result) bool {
	for _, part := range strings.Split(r.DisplayName, ",") {
		if closeSpelling(lk.home.town, part) {
			return true
		}
	}
	return false
}

// townRow is the town's own row: a row of the areas answer, else the first row
// of an areas search for the second part, whose name is a close spelling of it.
func (lk *typedLookup) townRow() *Result {
	if lk.townTry {
		return lk.town
	}
	lk.townTry = true
	find := func(rows []Result) *Result {
		for i := range rows {
			if closeSpelling(lk.home.town, rows[i].Name) {
				return &rows[i]
			}
		}
		return nil
	}
	if lk.town = find(lk.areas); lk.town == nil {
		lk.town = find(lk.photon(lk.home.town, false))
	}
	return lk.town
}

// inTown says a row lies in the typed town: its label names the town, or its
// point is within typedTownRadiusKm of the town's point.
func (lk *typedLookup) inTown(r Result) bool {
	if lk.labelNamesTown(r) {
		return true
	}
	town := lk.townRow()
	return town != nil && distanceKm(r.Lat, r.Lon, town.Lat, town.Lon) <= typedTownRadiusKm
}

// areaInTown says the areas answer already holds an area in the typed town
// whose name is a close spelling of the first part, so no typed row is needed.
// The same name in another town does not count.
func (lk *typedLookup) areaInTown() bool {
	for _, r := range lk.areas {
		if closeSpelling(lk.home.first, r.Name) && lk.inTown(r) {
			return true
		}
	}
	return false
}

// position is the typed row's point: a match in the same town from an any place
// search, else the town's own point.
func (lk *typedLookup) position() (lat, lon float64, ok bool) {
	var elsewhere []Result
	for _, r := range lk.photon(lk.home.name, true) {
		if !leadingNameMatches(lk.home.first, r.Name) {
			continue
		}
		if lk.labelNamesTown(r) {
			return r.Lat, r.Lon, true
		}
		elsewhere = append(elsewhere, r)
	}
	town := lk.townRow()
	if town == nil {
		return 0, 0, false
	}
	// A match whose label does not name the town still counts when it lies
	// near the town's point.
	for _, r := range elsewhere {
		if distanceKm(r.Lat, r.Lon, town.Lat, town.Lon) <= typedTownRadiusKm {
			return r.Lat, r.Lon, true
		}
	}
	return town.Lat, town.Lon, true
}
