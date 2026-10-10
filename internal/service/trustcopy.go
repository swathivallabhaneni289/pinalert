// trustcopy.go is the one home of the server authored trust copy (D-15, D-19,
// A50, A85, RESEARCH Pattern 3). Every string here is built only from the constants in
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

// countsCaveat ends the confirmed panel sentence only, once (A7 as amended by
// A85). It tells a reader the numbers count places and not raw votes.
const countsCaveat = "Counts places, not votes."

// confirmedSentence is the panel line for the confirming place count n (A7,
// RR-1, A85). It has one form for none, one for a single place and one for two
// or more, it always repeats the real number, and it ends with the caveat.
func confirmedSentence(n int) string {
	switch {
	case n <= 0:
		return "No one has confirmed this yet. " + countsCaveat
	case n == 1:
		return "1 place confirmed this. " + countsCaveat
	default:
		return strconv.Itoa(n) + " different places confirmed this. " + countsCaveat
	}
}

// disputedSentence is the panel line for the disputing place count m (A56,
// RR-1, A85), built on the same shape as confirmedSentence. The disputed line
// is a count of places too and carries no caveat, because the confirmed line
// above it does (A56 as amended by A85).
func disputedSentence(m int) string {
	switch {
	case m <= 0:
		return "No one nearby has disputed this."
	case m == 1:
		return "1 place disputed this."
	default:
		return strconv.Itoa(m) + " different places disputed this."
	}
}

// provisionalLeadSentence is the lead line of a Provisional report (D-17, A6,
// RR-2). The number comes from IndependentAgreementThreshold, so the copy
// follows the constant the resolver gates on.
func provisionalLeadSentence() string {
	return "Needs " + strconv.Itoa(IndependentAgreementThreshold) + " confirmations to count as confirmed."
}

// expiredLeadSentence is the lead line once a report is at or past its
// expiry (D-17).
const expiredLeadSentence = "Expired, so it can no longer be confirmed."

// The still current words (D-18). Too early to tell is shown only on critical
// and rescue needed reports with fewer than 2 confirming places, which never
// show the Unconfirmed label.
const (
	currencyLabelUpToDate          = "Up to date"
	currencyLabelGettingOld        = "Getting old"
	currencyLabelNeedsReconfirming = "Needs re-confirming"
	currencyLabelTooEarly          = "Too early to tell"
)

// currencyLabel returns the word for a scored band, and an empty string for
// every other band.
func currencyLabel(b CurrencyBand) string {
	switch b {
	case CurrencyUpToDate:
		return currencyLabelUpToDate
	case CurrencyGettingOld:
		return currencyLabelGettingOld
	case CurrencyNeedsReconfirming:
		return currencyLabelNeedsReconfirming
	default:
		return ""
	}
}

// windowPhrase turns a window length in minutes into words: 1 hour or N hours
// when the minutes are a positive whole number of hours, otherwise 1 minute
// or N minutes.
func windowPhrase(minutes int) string {
	if minutes > 0 && minutes%60 == 0 {
		if minutes == 60 {
			return "1 hour"
		}
		return strconv.Itoa(minutes/60) + " hours"
	}
	if minutes == 1 {
		return "1 minute"
	}
	return strconv.Itoa(minutes) + " minutes"
}

// upToDateSentence is the panel line of an Up to date report (D-19).
func upToDateSentence(recent int, window string) string {
	return strconv.Itoa(recent) + " places confirmed this in the last " + window + "."
}

// gettingOldSentence is the panel line of a Getting old report with no
// standing dispute (D-19).
func gettingOldSentence(window string) string {
	return "Fewer than " + strconv.Itoa(IndependentAgreementThreshold) + " places confirmed it in the last " + window + "."
}

// needsReconfirmingSentence is the panel line of a Needs re-confirming report
// with no standing dispute (D-19).
const needsReconfirmingSentence = "Gone quiet. People nearby can confirm it again."

// disputeRaisedBarSentence is the panel line when a standing dispute place
// raised the bar to k confirming places (D-19).
func disputeRaisedBarSentence(k int, window string) string {
	return "Disputed nearby, so it needs " + strconv.Itoa(k) + " places in the last " + window + "."
}

// tooEarlySentence is the panel line of a critical or rescue needed report
// with too few confirming places (D-19).
func tooEarlySentence() string {
	return "Fewer than " + strconv.Itoa(IndependentAgreementThreshold) + " places have confirmed this."
}

// currencyScoreSentence is the popup score line (A50).
func currencyScoreSentence(score int) string {
	return "Score " + strconv.Itoa(score) + " out of 100, based on recent confirmations."
}

// The reporter reliability words (D-15). None has no word, because a report
// with no rated account shows nothing (A47).
const (
	reporterLabelReliable   = "Reliable reporter"
	reporterLabelMixed      = "Mixed record"
	reporterLabelUnreliable = "Unreliable reporter"
	reporterLabelNew        = "New reporter"
)

// reporterLabel returns the word for a tag, and an empty string for none and
// for any value outside the closed tag set.
func reporterLabel(t ReporterTag) string {
	switch t {
	case ReporterReliable:
		return reporterLabelReliable
	case ReporterMixed:
		return reporterLabelMixed
	case ReporterUnreliable:
		return reporterLabelUnreliable
	case ReporterNew:
		return reporterLabelNew
	default:
		return ""
	}
}

// reporterSentence is the panel line for a reporter (D-19). A rated reporter
// reads how many of their judged reports were corroborated. A New reporter
// reads the judged minimum first, then how many reports have had a clear
// result, as a number or none (A85). Neither form calls any report a first
// report. None and any invalid tag give an empty string.
func reporterSentence(t ReporterTag, judged, corroborated int) string {
	switch t {
	case ReporterReliable, ReporterMixed, ReporterUnreliable:
		return strconv.Itoa(corroborated) + " of " + strconv.Itoa(judged) + " earlier reports were confirmed by people nearby."
	case ReporterNew:
		count := "none"
		if judged >= 1 {
			count = strconv.Itoa(judged)
		}
		return "A rating needs " + strconv.Itoa(ReliabilityMinJudged) + " reports with a clear result. They have " + count + " so far."
	default:
		return ""
	}
}

// reporterScoreSentence is the popup score line for a rated reporter (A50).
func reporterScoreSentence(score int) string {
	return "Score " + strconv.Itoa(score) + " out of 100, based on how often their reports were confirmed."
}
