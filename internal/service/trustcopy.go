// trustcopy.go is the one home of the server authored trust copy (D-19,
// RESEARCH Pattern 3). Every string here is built only from the constants in
// this file and integers formatted with strconv.Itoa, so no report text can
// ever enter one (T-03-06). The closed character set is ASCII letters,
// digits, space, comma, full stop, apostrophe and plus, plus a hyphen only
// inside the word re-confirming. No string carries a relative time, so a
// feed read and a vote response a moment apart stay equal (Pitfall 8).

package service

import "strconv"

// trustCountCap is the largest number a counter shows as a plain numeral.
// Above it the numeral reads 99+ (A27). The cap lives only on the server, so
// no cap exists in JavaScript.
const trustCountCap = 99

// trustCountText returns the short numeral for a counter: the decimal number
// up to trustCountCap and 99+ above it. The panel sentences always carry the
// real number, even above the cap (V-12).
func trustCountText(n int) string {
	if n > trustCountCap {
		return strconv.Itoa(trustCountCap) + "+"
	}
	return strconv.Itoa(n)
}

// countsCaveat ends every confirmed and disputed panel sentence (A7). It
// tells a reader the numbers count separate places and not raw votes.
const countsCaveat = "Counts separate places, not votes."

// confirmedSentence is the panel line for the confirming place count n (A7,
// RR-1). It has one form for none, one for a single place and one for two or
// more, and it always repeats the real number.
func confirmedSentence(n int) string {
	switch {
	case n <= 0:
		return "No one has confirmed this yet. " + countsCaveat
	case n == 1:
		return "1 person in 1 place confirmed this. " + countsCaveat
	default:
		return strconv.Itoa(n) + " people in " + strconv.Itoa(n) + " different places confirmed this. " + countsCaveat
	}
}

// disputedSentence is the panel line for the disputing place count m (A56,
// RR-1), built on the same shape as confirmedSentence.
func disputedSentence(m int) string {
	switch {
	case m <= 0:
		return "No one nearby has disputed this. " + countsCaveat
	case m == 1:
		return "1 person in 1 place disputed this. " + countsCaveat
	default:
		return strconv.Itoa(m) + " people in " + strconv.Itoa(m) + " different places disputed this. " + countsCaveat
	}
}

// provisionalLeadSentence is the lead line of a Provisional report (D-17, A6,
// RR-2). The number comes from IndependentAgreementThreshold, so the copy
// follows the constant the resolver gates on.
func provisionalLeadSentence() string {
	return "Needs " + strconv.Itoa(IndependentAgreementThreshold) + " confirmations before this counts as confirmed."
}

// expiredLeadSentence is the lead line once a report is at or past its
// expiry (D-17).
const expiredLeadSentence = "This report has expired. It can no longer be confirmed."
