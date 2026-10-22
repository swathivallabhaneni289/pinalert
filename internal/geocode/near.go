package geocode

// The near only rule of plan 04-32 (04-UAT test 11). On 2026-10-23, near Benz
// Circle in Vijayawada, the owner searched for a well known mall that the map
// data does not hold, and the answer led with malls of similar names in Delhi,
// Bhopal and Mumbai. When the search knows where the person is (their
// position, the report pin or the area the map shows), it now answers only
// with places within nearRadiusKm of that point, and a search that finds only
// far rows has found nothing, so the page can say so plainly.

import (
	"context"
	"strings"
)

// textsFor is searchTexts for this search. With a position (and outside the
// home area search), a shorter text of one word is not tried: one word left
// over from a longer name matches places that merely start with it. Live on
// 2026-10-23 near Benz Circle, "PVP Square Mall" (not in the map data) went on
// to "Mall" and was answered with villages named Mallapalle and Mallavaram,
// where the page should say plainly that nothing near matches. Shorter texts
// of two or more words, such as an address without its flat number, are still
// tried, and a search with no position tries every shorter text, as before.
func textsFor(ctx context.Context, q string, near *Point) []string {
	texts := searchTexts(q)
	if near == nil || areasOnly(ctx) {
		return texts
	}
	kept := []string{texts[0]}
	for _, text := range texts[1:] {
		if len(strings.Fields(text)) >= 2 {
			kept = append(kept, text)
		}
	}
	return kept
}

// answerable reports whether a row at lat, lon named name may be answered for
// the text q searched around near.
//
//   - With no position (near is nil) every row is answerable: the search stays
//     India wide, as it was.
//   - The home area search (areas only) is left as it was: a home area is
//     often far from where the person is standing, and that search has its own
//     typed row and Not found rules.
//   - A row within nearRadiusKm of near is answerable.
//   - A far row is answerable only when it is a city or a town whose name is a
//     close spelling of what was typed, so "Hyderabad" typed in Vijayawada
//     still moves the map to Hyderabad. Jumping to another area by name is
//     what the map's place search is for (HIST-05), and a city named as typed
//     is never a look alike of something near.
func answerable(ctx context.Context, q string, near *Point, lat, lon float64, cityLevel bool, name string) bool {
	if near == nil || areasOnly(ctx) {
		return true
	}
	if distanceKm(near.Lat, near.Lon, lat, lon) <= nearRadiusKm {
		return true
	}
	return cityLevel && closeSpelling(q, name)
}

// keepNear returns the rows of results that answerable allows, in their order.
// It never changes the slice it is given.
func keepNear(ctx context.Context, q string, near *Point, results []Result) []Result {
	if near == nil || areasOnly(ctx) {
		return results
	}
	kept := make([]Result, 0, len(results))
	for _, r := range results {
		if answerable(ctx, q, near, r.Lat, r.Lon, r.cityLevel, r.Name) {
			kept = append(kept, r)
		}
	}
	return kept
}
