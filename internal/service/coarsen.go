// coarsen.go holds the one shared coarsening of a finished Rescue report (plan
// 03.1-01, D-23, D-28). A Rescue report names a person in danger at an exact
// spot, so once it is finished the spot is blurred to the centre of a geohash
// cell. The function is pure and viewer independent: every read path calls it,
// and later phases reuse it, so no path can forget one of the leaking fields.

package service

import "github.com/mmcloughlin/geohash"

// ApproxGeohashPrecision is the geohash precision of the blurred spot. A
// precision 6 cell is 30 bits, 15 for longitude and 15 for latitude, so it is
// 360/2^15 degrees wide and 180/2^15 degrees tall: about 1.22 km wide at the
// equator (about 1.0 km at 35 degrees north) and about 0.61 km tall. A point is
// at most half the cell diagonal from the cell centre, about 0.68 km, which is
// the "about 1 km" of D-23 and stays under 0.8 km across India. The geohash
// cell is the chosen grid, not rounding to 0.01 degree (RESEARCH A1).
const ApproxGeohashPrecision = 6

// GeoPoint is a latitude and longitude pair, used for the viewer's centre.
type GeoPoint struct {
	Lat, Lon float64
}

// CoarsePoint returns the centre of the precision 6 geohash cell that holds the
// point, and that 6 character cell string. It is idempotent: the centre of a
// cell lies inside the same cell.
func CoarsePoint(lat, lon float64) (float64, float64, string) {
	cell := geohash.EncodeWithPrecision(lat, lon, ApproxGeohashPrecision)
	cLat, cLon := geohash.DecodeCenter(cell)
	return cLat, cLon, cell
}

// CoarsenFinishedRescue blurs a finished Rescue report and says whether it did.
// Latitude, longitude, geohash, description and distance all change together
// in this one place, because coarsening only some of them is the leak this
// function exists to prevent (RESEARCH Pitfall 1): the exact geohash, the
// description text or the distance to an exact point would each give the spot
// back. The distance is measured from centre to the coarse point, and it is
// nil when centre is nil, so an exact distance is never left behind.
//
// A report that is not finished, and every category other than Rescue, comes
// back unchanged with false (D-28). Report is passed by value and the pointer
// DistanceKm of the input is never written through.
func CoarsenFinishedRescue(r Report, finished bool, centre *GeoPoint) (Report, bool) {
	if !finished || r.Category != CategoryRescueNeeded {
		return r, false
	}
	lat, lon, cell := CoarsePoint(r.Latitude, r.Longitude)
	r.Latitude = lat
	r.Longitude = lon
	r.Geohash = cell
	r.Description = ""
	r.DistanceKm = nil
	if centre != nil {
		d := haversineKm(centre.Lat, centre.Lon, lat, lon)
		r.DistanceKm = &d
	}
	return r, true
}
