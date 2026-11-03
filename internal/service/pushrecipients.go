// pushrecipients.go is the inverse lookup of phone alerts (plan 03.3-09): from a
// report to the accounts whose home area is near it. 3.2's Reliable alert goes
// the other way, from an account's own bell poll to nearby reports; a push has
// to go from the report, with the app closed.
//
// The lookup selects accounts by their stored home cell, so the server never
// needs anyone's position, and distance is measured from the middle of the home
// cell (3.2 Q5). D-22's eight neighbour cells are superseded by the distance
// each person chose in Settings (1, 2, 5 or 10 km).

package service

import (
	"math"
	"sort"

	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// PushCandidateMaxRadiusKm is the largest distance a person can choose in
	// Settings; every candidate cell is searched out to it.
	PushCandidateMaxRadiusKm = 10
	// homeCellHalfDiagonalKm is the reach from the middle of a precision 5 home
	// cell (about 4.9 km across) to its corner, rounded up. A home whose centre
	// is within the chosen distance of the report can have its cell centre this
	// much beyond the distance.
	homeCellHalfDiagonalKm = 3.6
	// candidateLatticeStep is the step, in degrees, of the sampling lattice. A
	// precision 5 cell is about 0.044 degrees across, so a step under half of
	// that touches every cell the box overlaps.
	candidateLatticeStep = 0.02
	// candidateCellsMax bounds the list. It is never reached at real latitudes
	// (the 10 km list holds about 60 cells); it only keeps a point near a pole,
	// where the longitude range opens to the whole circle, from building an
	// enormous array.
	candidateCellsMax = 400
)

// candidateCells lists the precision 5 cells that can hold the home of someone
// within radiusKm of the point, plus the cell half diagonal. The report's own
// cell comes first and the rest follow by the distance from the report point to
// each cell's middle, ties by the cell string, so PushHomeCandidates (which
// orders by the position of the home cell in this list before its row cap)
// keeps the nearest accounts when an area holds more than the cap (review note
// N1). Cells across the date line are not wrapped; the product serves India.
func candidateCells(lat, lon, radiusKm float64) []string {
	latMin, latMax, lonMin, lonMax := BoundingBox(lat, lon, radiusKm+homeCellHalfDiagonalKm)
	seen := make(map[string]struct{})
	for la := latMin; la <= latMax+candidateLatticeStep; la += candidateLatticeStep {
		for lo := lonMin; lo <= lonMax+candidateLatticeStep; lo += candidateLatticeStep {
			seen[geohash.EncodeWithPrecision(clamp(la, -90, 90), clamp(lo, -180, 180), 5)] = struct{}{}
		}
	}
	own := geohash.EncodeWithPrecision(lat, lon, 5)
	delete(seen, own)

	type distCell struct {
		cell string
		km   float64
	}
	rest := make([]distCell, 0, len(seen))
	for c := range seen {
		clat, clon := geohash.DecodeCenter(c)
		rest = append(rest, distCell{cell: c, km: haversineKm(lat, lon, clat, clon)})
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].km != rest[j].km {
			return rest[i].km < rest[j].km
		}
		return rest[i].cell < rest[j].cell
	})

	out := make([]string, 0, len(rest)+1)
	out = append(out, own)
	for _, r := range rest {
		if len(out) >= candidateCellsMax {
			break
		}
		out = append(out, r.cell)
	}
	return out
}

// PushRecipient is one account chosen to be told about a report, with the
// distance from the middle of its home cell to the report. It holds no position.
type PushRecipient struct {
	AccountID  int64
	DistanceKm float64
}

// filterRecipients applies the rules the candidate read leaves to Go: the home
// cell must decode, the distance from its middle to the report point must be at
// most the account's own chosen distance, the account must not be the author,
// and want (when not nil) must accept the row. want is the caller's own row
// filter (Critical passes nil, the Reliable bell refresh passes the not serious
// test); no stored switch is read (owner decision 2026-11-03, quick 261103-jal,
// D2). The result is nearest first, ties by account
// id, and holds at most PushRecipientsPerReport accounts.
func filterRecipients(rows []sqlcgen.PushHomeCandidatesRow, lat, lon float64, author *int64, want func(sqlcgen.PushHomeCandidatesRow) bool) []PushRecipient {
	var out []PushRecipient
	for _, r := range rows {
		if author != nil && r.AccountID == *author {
			continue
		}
		if want != nil && !want(r) {
			continue
		}
		alat, alon, ok := HomeAreaAnchor(r.HomeAreaCell)
		if !ok {
			continue
		}
		d := haversineKm(lat, lon, alat, alon)
		if math.IsNaN(d) || d > float64(r.AlertRadiusKm) {
			continue
		}
		out = append(out, PushRecipient{AccountID: r.AccountID, DistanceKm: d})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DistanceKm != out[j].DistanceKm {
			return out[i].DistanceKm < out[j].DistanceKm
		}
		return out[i].AccountID < out[j].AccountID
	})
	if len(out) > PushRecipientsPerReport {
		out = out[:PushRecipientsPerReport]
	}
	return out
}
