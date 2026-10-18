package geocode

// The areas only mode of the reader lane (Phase 3.4 plan 28), used by the home
// area fields: the person types a few letters and is offered neighbourhoods,
// localities, towns, cities and districts, never a street or a building. The
// stored home cell is about 5 km wide, so a finer place is no use there.
//
// The mode travels in the context, so it reaches every upstream request of a
// search without a change to the signatures the search order is built from: the
// first Photon try, its timeout retry, the shorter Photon tries, and every
// Nominatim request. It also keys both caches (see cacheKeyFor).
//
// Parameters checked live on 2026-10-17:
//   - Photon: layer=locality&layer=district&layer=city (the parameter repeated).
//     The county layer was left out: it only added administrative districts that
//     the city layer already gives ("Hyderabad District" beside Hyderabad).
//   - Nominatim: featureType=settlement, which returned suburbs and villages for
//     Indiranagar and nothing for a street.

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"
)

type areasOnlyKey struct{}

// withAreasOnly marks ctx as an areas only search.
func withAreasOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, areasOnlyKey{}, true)
}

// areasOnly reports whether ctx carries the areas only mode.
func areasOnly(ctx context.Context) bool {
	on, _ := ctx.Value(areasOnlyKey{}).(bool)
	return on
}

// cacheKeyFor is cacheKey for the search mode ctx carries. The normal key is
// unchanged; the areas only key adds a suffix, so an answer for one mode is
// never served for the other.
func cacheKeyFor(ctx context.Context, q string, near *Point) string {
	key := cacheKey(q, near)
	if areasOnly(ctx) {
		return key + "|areas"
	}
	return key
}

// photonAreaLayers are the Photon layers an areas only search asks for.
var photonAreaLayers = []string{"locality", "district", "city"}

// nominatimAreaFeatureType is the featureType an areas only search sends.
const nominatimAreaFeatureType = "settlement"

// photonAreaTypes are Photon's own row sizes that count as an area.
var photonAreaTypes = map[string]bool{
	"locality": true,
	"district": true,
	"city":     true,
}

// photonNotAreaValues are place values Photon files under the area layers that
// are one dwelling or one plot.
var photonNotAreaValues = map[string]bool{
	"house":             true,
	"plot":              true,
	"farm":              true,
	"isolated_dwelling": true,
	"allotments":        true,
	"square":            true,
	"city_block":        true,
}

// photonIsArea says whether a Photon row is an area a person lives in. The row
// must be of an area size (locality, district or city) and drawn as a place or
// an administrative boundary. A residential land use area also counts, because
// that is how OpenStreetMap draws some real neighbourhoods (Kothrud in Pune);
// industrial and commercial land use (a port, a depot, a power plant) does not.
func photonIsArea(p photonProperties) bool {
	if !photonAreaTypes[strings.ToLower(strings.TrimSpace(p.Type))] {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(p.OSMKey))
	value := strings.ToLower(strings.TrimSpace(p.OSMValue))
	if photonNotAreaValues[value] {
		return false
	}
	switch key {
	case "place", "boundary":
		return true
	case "landuse":
		return value == "residential"
	}
	return false
}

// photonArea is the name of the area a Photon row lies in: the row's own name
// when it is an area (photonIsArea), else its locality, district or city, in
// that order. Empty when Photon sent none of them. The first time flow uses it
// to prefill the home area from a street or a building picked on step 3.
func photonArea(p photonProperties) string {
	if photonIsArea(p) {
		return cleanPart(p.Name)
	}
	for _, raw := range []string{p.Locality, p.District, p.City} {
		if part := cleanPart(raw); part != "" {
			return part
		}
	}
	return ""
}

// nominatimArea is the area a Nominatim row lies in, but only when the row is
// an area itself: its own name, or the first part of its display name when it
// has no name. Nominatim sends the address parts only when asked
// (addressdetails=1), and asking would change every outbound request, so a
// street or building from the Nominatim fallback has no area and the field
// stays empty. Photon answers first, so this is the rare case.
func nominatimArea(rec nominatimRecord) string {
	if !nominatimIsArea(rec) {
		return ""
	}
	if name := cleanPart(rec.Name); name != "" {
		return name
	}
	first, _, _ := strings.Cut(rec.DisplayName, ",")
	return cleanPart(first)
}

// nominatimAreaTypes are the addresstype values of a Nominatim row that is an
// area. A street is "road", a building "building" or "house".
var nominatimAreaTypes = map[string]bool{
	"suburb":        true,
	"neighbourhood": true,
	"quarter":       true,
	"city_district": true,
	"borough":       true,
	"district":      true,
	"locality":      true,
	"hamlet":        true,
	"village":       true,
	"town":          true,
	"city":          true,
	"municipality":  true,
}

// nominatimIsArea says whether a Nominatim row is an area: drawn as a place or
// an administrative boundary, and of an area kind.
func nominatimIsArea(rec nominatimRecord) bool {
	category := strings.ToLower(strings.TrimSpace(rec.Category))
	if category != "place" && category != "boundary" {
		return false
	}
	return nominatimAreaTypes[strings.ToLower(strings.TrimSpace(rec.AddressType))]
}

// photonIsCityLevel says a Photon row is a city or a town, drawn as a place or
// as the administrative boundary of a city. A village, a suburb or a locality
// is not (plan 34): a town word typed without a comma is placed only by a row
// of this kind, so a hamlet that happens to share a name never places it.
func photonIsCityLevel(p photonProperties) bool {
	key := strings.ToLower(strings.TrimSpace(p.OSMKey))
	value := strings.ToLower(strings.TrimSpace(p.OSMValue))
	switch key {
	case "place":
		return value == "city" || value == "town"
	case "boundary":
		return strings.ToLower(strings.TrimSpace(p.Type)) == "city"
	}
	return false
}

// photonRegion is the town a PIN code row names, for a row drawn as a postcode
// and empty for every other row (see regionName).
func photonRegion(p photonProperties) string {
	if !strings.EqualFold(strings.TrimSpace(p.OSMKey), "place") || !strings.EqualFold(strings.TrimSpace(p.OSMValue), "postcode") {
		return ""
	}
	return regionName(p.City, p.County)
}

// regionName is the town a postcode row names: its city when it has one, else
// its county with a trailing "(Urban)" or "District" dropped (live, 520008 is
// in the county "Vijayawada (Urban)"). The district part of a postcode row is a
// locality and is never used. Empty when no name of at least three letters
// without a digit is left.
func regionName(city, county string) string {
	name := cleanPart(city)
	if name == "" {
		name = cleanPart(county)
	}
	if i := strings.Index(name, "("); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	if strings.HasSuffix(strings.ToLower(name), " district") {
		name = strings.TrimSpace(name[:len(name)-len(" district")])
	}
	if utf8.RuneCountInString(name) < minTypedPartRunes || strings.IndexFunc(name, unicode.IsDigit) >= 0 {
		return ""
	}
	return name
}
