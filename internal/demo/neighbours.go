package demo

import (
	"math"
	"time"
)

// The pretend neighbours (D-01, D-05). After the visitor posts a practice
// report, a short script of neighbours confirms it one by one so the visitor
// watches "confirmed by N" climb. The script is a pure function of elapsed time:
// PlanNeighbours says who votes when, and World.ApplyDue casts whatever has fallen
// due, through the real CastVote, before each read. There is no goroutine and no
// timer, so the script tests with a fake clock.

// NeighbourInterval is the gap between one neighbour's vote and the next.
const NeighbourInterval = 4 * time.Second

// neighbourMetres and neighbourBearings place the neighbours: 300, 450, 600 and
// 750 metres from the report on bearings 0, 90, 180 and 270 degrees. Each is
// farther than a precision 7 cell (about 150 m) from the report and from the
// others, so each lands in its own cell, and all are inside the 1 km vote radius.
var (
	neighbourMetres   = []float64{300, 450, 600, 750}
	neighbourBearings = []float64{0, 90, 180, 270}
)

// PlannedVote is one confirmation a pretend neighbour will cast.
type PlannedVote struct {
	ReportID  int64
	AccountID int64
	Lat, Lon  float64
	DueAt     time.Time
}

// PlanNeighbours returns the confirmations planned for a report at (lat, lon)
// posted at postedAt: neighbour k (1 to 4) is the synthetic account
// NeighbourAccountBase plus k, stands at its own spot and votes at postedAt plus
// k times NeighbourInterval. It is pure.
func PlanNeighbours(reportID int64, lat, lon float64, postedAt time.Time) []PlannedVote {
	plan := make([]PlannedVote, 0, len(neighbourMetres))
	for i := range neighbourMetres {
		k := i + 1
		nLat, nLon := offsetPoint(lat, lon, neighbourMetres[i], neighbourBearings[i])
		plan = append(plan, PlannedVote{
			ReportID:  reportID,
			AccountID: NeighbourAccountBase + int64(k),
			Lat:       nLat,
			Lon:       nLon,
			DueAt:     postedAt.Add(time.Duration(k) * NeighbourInterval),
		})
	}
	return plan
}

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
