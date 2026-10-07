// geo.go holds the D-01 vote distance rule: a vote may only be cast from
// within MaxVoteDistanceKm of the report it is cast on. It mitigates two
// threats from the phase 3 threat model: T-03-01 (the reporter exemption is
// decided elsewhere, from the database's reporter identity, never here) and
// T-03-02 (a NaN or Inf coordinate must fail closed, which the comparison in
// withinVoteRadius guarantees).
//
// The coordinates a client sends cannot be verified by the server. A person
// who spoofs their location defeats this rule and the geohash cell rule
// alike. Plan 03-02's docs state that limit plainly; nothing in this file
// claims the rule resists spoofing.
//
// Like visibility.go this file is pure: no I/O, no context, no store types.

package service

import (
	"errors"
	"math"
	"strconv"
)

// MaxVoteDistanceKm is the one server side vote radius (D-01). Its readers
// are CastVote, TooFarMessage, the docs (plan 03-02), the swagger text (plan
// 03-04) and the page config (plan 03-18). No other 1 km literal belongs in
// Go code.
const MaxVoteDistanceKm = 1.0

// earthRadiusKm matches the radius the NearbyReports SQL uses, so the feed's
// distance_km and this check agree on what a kilometre is.
const earthRadiusKm = 6371.0

// haversineKm returns the great circle distance in kilometres between two
// points given in degrees. It uses the haversine form rather than the law of
// cosines because that form is more accurate at short range. The
// intermediate is clamped to [0, 1] so rounding at the antipodes cannot push
// math.Sqrt or math.Asin into NaN. (report.go owns the name clamp in this
// package, so the clamp is written with math.Min and math.Max.)
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const degToRad = math.Pi / 180
	phi1 := lat1 * degToRad
	phi2 := lat2 * degToRad
	dPhi := (lat2 - lat1) * degToRad
	dLambda := (lon2 - lon1) * degToRad

	sinDPhi := math.Sin(dPhi / 2)
	sinDLambda := math.Sin(dLambda / 2)
	a := sinDPhi*sinDPhi + math.Cos(phi1)*math.Cos(phi2)*sinDLambda*sinDLambda
	a = math.Min(1, math.Max(0, a))
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}

// withinVoteRadius reports whether the voter point is inside the vote radius
// of the report point. The inclusive less than or equal form is what makes it
// fail closed: every comparison with NaN is false, and an Inf argument makes
// the haversine NaN, so a non finite coordinate is always refused. It must
// never be rewritten as a strict greater than test against the radius, which
// would let NaN through.
func withinVoteRadius(rLat, rLon, vLat, vLon float64) bool {
	return haversineKm(rLat, rLon, vLat, vLon) <= MaxVoteDistanceKm
}

// ErrVoterTooFar is returned by CastVote when the voter is farther than
// MaxVoteDistanceKm from the report, or when a coordinate is not finite.
// Plan 03-04 maps it to HTTP 403 with field location and TooFarMessage.
var ErrVoterTooFar = errors.New("service: voter is too far from the report")

// TooFarMessage is the one refusal sentence for all four vote values (UI-SPEC
// Copywriting Contract). It is built from the constant and one number only
// and never includes the computed distance. Plan 03-04 maps ErrVoterTooFar to
// HTTP 403 with field location and this text.
func TooFarMessage() string {
	return "Too far to vote. You need to be within " + strconv.FormatFloat(MaxVoteDistanceKm, 'f', -1, 64) + " km of this report."
}
