// stars.go holds the pure star display helpers of Phase 9 (D-05). Stars are
// the 0 to 100 reporter score divided by 20, shown to one decimal. The file is
// pure: nothing here reads the clock or performs I/O, and every step is
// integer arithmetic.

package service

import "strconv"

// starsMaxTenths is the top of the star range in tenths: 5.0 stars.
const starsMaxTenths = 50

// MinRatersForStars is how many counted raters a reporter needs before tapped
// ratings count at all (D-02). Raters are counted by distinct voter cell (D-21).
// Below it the points are the report outcome score alone.
const MinRatersForStars = 5

// RatingPointsPerStar converts a tapped star into the 0 to 100 point scale: one
// star is 20 points, so a five star average is 100 and a one star average is 20.
const RatingPointsPerStar = 20

// CombinePoints returns the reporter's points from the outcome score and the
// tapped ratings (D-02, D-05, research 1.4). With fewer than MinRatersForStars
// counted raters it returns the outcome unchanged (review C1). From five raters
// on, the points are half the outcome and half the tapped average, rounded half
// up, in integers only:
//
//	num = outcome * raters + RatingPointsPerStar * tapSum
//	den = 2 * raters
//	points = (2 * num + den) / (2 * den)
//
// and clamped to 0 to 100. The tag is cut from these points by
// reporterTagForScore, the one place the Reliable line lives. Five raters in five
// different places can move the points by up to 40 either way (review B4).
func CombinePoints(outcome, raters, tapSum int) int {
	if raters < MinRatersForStars {
		return outcome
	}
	num := outcome*raters + RatingPointsPerStar*tapSum
	den := 2 * raters
	points := (2*num + den) / (2 * den)
	if points < 0 {
		return 0
	}
	if points > 100 {
		return 100
	}
	return points
}

// TappedHalfStars returns the tapped average rounded to the nearest half star,
// in halves, for example 9 for 4.5 stars (review W5). It is
// (4 * tapSum + raters) / (2 * raters) in integers. The shown tapped part is
// never an exact average, which blurs one person's exact stars when there are
// few raters; it is not a guarantee, because the exact total, the outcome part
// and the rater count still let a careful reader estimate the average closely.
// It returns 0 when there are no raters.
func TappedHalfStars(raters, tapSum int) int {
	if raters <= 0 {
		return 0
	}
	return (4*tapSum + raters) / (2 * raters)
}

// HalfStarsText returns halves as text with one decimal, "4.5" for 9 and "4.0"
// for 8, built from integers.
func HalfStarsText(halves int) string {
	if halves < 0 {
		halves = 0
	}
	frac := "0"
	if halves%2 == 1 {
		frac = "5"
	}
	return strconv.Itoa(halves/2) + "." + frac
}

// StarsTenths returns the stars of a 0 to 100 score in tenths of a star:
// points divided by 2 with integer division, clamped to 0 to 50. It floors and
// never rounds.
func StarsTenths(points int) int {
	if points < 0 {
		return 0
	}
	t := points / 2
	if t > starsMaxTenths {
		return starsMaxTenths
	}
	return t
}

// StarsText returns the stars of a 0 to 100 score as text with one decimal,
// for example "3.5". It floors, because score 69 is 3.45 stars and must never
// read 3.5 beside the Mixed record tag. It is built from the tenths with
// integer arithmetic; formatting a float with one decimal rounds half up and
// would print 69 as 3.5, so that is a bug here (research Pitfall 1).
func StarsText(points int) string {
	t := StarsTenths(points)
	return strconv.Itoa(t/10) + "." + strconv.Itoa(t%10)
}
