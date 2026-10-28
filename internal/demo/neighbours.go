package demo

import "math"

// meanEarthRadiusM is the sphere radius the destination point formula uses, in
// metres. It is the same 6371 km the services use for every distance.
const meanEarthRadiusM = 6371000.0

// offsetPoint returns the point metres away from (lat, lon) along bearingDeg,
// where 0 is north and 90 is east, on a sphere (the standard destination point
// formula). At the distances the Demo uses (under a kilometre) the error against
// the true ellipsoid is far below the width of a precision 7 cell.
func offsetPoint(lat, lon, metres, bearingDeg float64) (float64, float64) {
	const rad = math.Pi / 180
	phi1 := lat * rad
	lam1 := lon * rad
	theta := bearingDeg * rad
	delta := metres / meanEarthRadiusM

	sinPhi2 := math.Sin(phi1)*math.Cos(delta) + math.Cos(phi1)*math.Sin(delta)*math.Cos(theta)
	phi2 := math.Asin(sinPhi2)
	lam2 := lam1 + math.Atan2(
		math.Sin(theta)*math.Sin(delta)*math.Cos(phi1),
		math.Cos(delta)-math.Sin(phi1)*sinPhi2,
	)
	return phi2 / rad, lam2 / rad
}
