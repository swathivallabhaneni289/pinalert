// stars.go holds the pure star display helpers of Phase 9 (D-05). Stars are
// the 0 to 100 reporter score divided by 20, shown to one decimal. The file is
// pure: nothing here reads the clock or performs I/O, and every step is
// integer arithmetic.

package service

import "strconv"

// starsMaxTenths is the top of the star range in tenths: 5.0 stars.
const starsMaxTenths = 50

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
