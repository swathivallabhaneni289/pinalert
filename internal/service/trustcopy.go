// trustcopy.go is the one home of the server authored trust copy (D-15, D-19,
// A50, RESEARCH Pattern 3). Every string here is built only from the constants in
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
	return strconv.Itoa(recent) + " different places confirmed this in the last " + window + "."
}

// gettingOldSentence is the panel line of a Getting old report with no
// standing dispute (D-19).
func gettingOldSentence(window string) string {
	return "Confirmed earlier, but fewer than " + strconv.Itoa(IndependentAgreementThreshold) + " places confirmed it in the last " + window + "."
}

// needsReconfirmingSentence is the panel line of a Needs re-confirming report
// with no standing dispute (D-19).
const needsReconfirmingSentence = "Confirmed earlier, but it has gone quiet. People nearby can confirm it again."

// disputeRaisedBarSentence is the panel line when a standing dispute place
// raised the bar to k confirming places (D-19).
func disputeRaisedBarSentence(k int, window string) string {
	return "Someone nearby disputed this, so it needs " + strconv.Itoa(k) + " places to confirm it in the last " + window + "."
}

// tooEarlySentence is the panel line of a critical or rescue needed report
// with too few confirming places (D-19).
func tooEarlySentence() string {
	return "Fewer than " + strconv.Itoa(IndependentAgreementThreshold) + " places have confirmed this so far."
}

// currencyScoreSentence is the popup score line (A50).
func currencyScoreSentence(score int) string {
	return "Score " + strconv.Itoa(score) + " out of 100, based on how recently different places confirmed this."
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
// reads how many reports have had a clear result so far, in one of three
// forms, followed by the judged minimum. Neither form calls any report a
// first report. None and any invalid tag give an empty string.
func reporterSentence(t ReporterTag, judged, corroborated int) string {
	switch t {
	case ReporterReliable, ReporterMixed, ReporterUnreliable:
		return strconv.Itoa(corroborated) + " of " + strconv.Itoa(judged) + " earlier reports were confirmed by people nearby."
	case ReporterNew:
		var lead string
		switch {
		case judged >= 2:
			lead = "Only " + strconv.Itoa(judged) + " of their reports have had a clear result so far."
		case judged == 1:
			lead = "Only 1 of their reports has had a clear result so far."
		default:
			lead = "None of their reports has had a clear result yet."
		}
		return lead + " A rating needs " + strconv.Itoa(ReliabilityMinJudged) + " reports with a clear result."
	default:
		return ""
	}
}

// reporterScoreSentence is the popup score line for a rated reporter (A50).
func reporterScoreSentence(score int) string {
	return "Score " + strconv.Itoa(score) + " out of 100, based on how often their reports with a clear result were confirmed."
}
