// trustcopy.go is the one home of the server authored trust copy (D-15, D-19,
// A50, A85, A86, A88, RESEARCH Pattern 3). Every string here is built only
// from the constants in this file and integers formatted with strconv.Itoa,
// so no report text can ever enter one (T-03-06). The closed character set is
// ASCII letters, digits, space, comma, full stop, apostrophe and plus, plus a
// hyphen only inside the word re-confirming. No string carries a relative
// time, so a feed read and a vote response a moment apart stay equal
// (Pitfall 8). People confirm and dispute, never places (A88): a count of
// places is worded as people at different spots, and the word place appears
// in no sentence.

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

// confirmedSentence is the panel line for the confirming place count n (A7,
// RR-1, A88). It has one form for none, one for a single person and one for
// two or more, and it always repeats the real number. The count is of distinct
// places, a lower bound on people (D-04), and the sentence says people at
// different spots, so several people at one spot count once without a caveat.
func confirmedSentence(n int) string {
	switch {
	case n <= 0:
		return "No one has confirmed this yet."
	case n == 1:
		return "1 person confirmed this."
	default:
		return "Confirmed by " + strconv.Itoa(n) + " people at different spots."
	}
}

// disputedSentence is the panel line for the disputing place count m (A56,
// RR-1, A88), built on the same shape as confirmedSentence. The one person
// form says nearby.
func disputedSentence(m int) string {
	switch {
	case m <= 0:
		return "No one has disputed this."
	case m == 1:
		return "1 person nearby disputed this."
	default:
		return strconv.Itoa(m) + " people at different spots disputed this."
	}
}

// provisionalLeadSentence is the lead line of a Provisional report that
// nobody has confirmed yet (D-17, A6, RR-2, A88). The number comes from
// IndependentAgreementThreshold, so the copy follows the constant the resolver
// gates on. The Phase 4 glossary reuses it as the threshold sentence.
func provisionalLeadSentence() string {
	return "Needs " + strconv.Itoa(IndependentAgreementThreshold) + " people at different spots to confirm it."
}

// provisionalLeadSentenceFor is the lead line of a Provisional report that
// has the given count of confirming places (A88). It says how many more
// people the report needs: the threshold form while nobody has confirmed, one
// more person after one place, and a plural form for a larger threshold. The
// number needed is clamped to the range 1 to the threshold, so the line stays
// plain text even for a Provisional report that already shows the threshold,
// which the resolver never produces.
func provisionalLeadSentenceFor(confirmed int) string {
	need := IndependentAgreementThreshold - confirmed
	switch {
	case need >= IndependentAgreementThreshold:
		return provisionalLeadSentence()
	case need <= 1:
		return "Needs 1 more person at a different spot to confirm it."
	default:
		return "Needs " + strconv.Itoa(need) + " more people at different spots to confirm it."
	}
}

// expiredLeadSentence is the lead line once a report is at or past its
// expiry (D-17).
const expiredLeadSentence = "Expired, so it can no longer be confirmed."

// The still current words (D-18). The three scored words name their subject,
// the report, so a reader knows what is up to date or getting old (A86). Too
// early to tell is shown only on critical and rescue needed reports with fewer
// than 2 confirming places, which never show the Unconfirmed label.
const (
	currencyLabelUpToDate          = "Report up to date"
	currencyLabelGettingOld        = "Report getting old"
	currencyLabelNeedsReconfirming = "Report needs re-confirming"
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

// confirmationsPhrase is a count of confirmations with its noun in the right
// number: 1 confirmation, 2 confirmations (A88).
func confirmationsPhrase(n int) string {
	if n == 1 {
		return "1 confirmation"
	}
	return strconv.Itoa(n) + " confirmations"
}

// upToDateSentence is the panel line of a Report up to date report (D-19,
// A88). recent is the confirming count inside the window.
func upToDateSentence(recent int, window string) string {
	return confirmationsPhrase(recent) + " in the last " + window + "."
}

// gettingOldSentence is the panel line of a Report getting old report with no
// standing dispute (D-19, A88).
func gettingOldSentence(window string) string {
	return "Fewer than " + confirmationsPhrase(IndependentAgreementThreshold) + " in the last " + window + "."
}

// needsReconfirmingSentence is the panel line of a Report needs re-confirming
// report with no standing dispute (D-19).
const needsReconfirmingSentence = "Gone quiet. People nearby can confirm it again."

// disputeRaisedBarSentence is the panel line when a standing dispute raised
// the bar to k confirmations (D-19, A88).
func disputeRaisedBarSentence(k int, window string) string {
	return "Disputed nearby, so it needs " + confirmationsPhrase(k) + " in the last " + window + "."
}

// tooEarlySentence is the panel line of a critical or rescue needed report
// with too few confirming people (D-19, A88). The threshold is 2 or more, so
// the plural is fixed (TestPanelWordingPluralConstants).
func tooEarlySentence() string {
	return "Fewer than " + strconv.Itoa(IndependentAgreementThreshold) + " people have confirmed this yet."
}

// currencyScoreSentence is the popup score line (A50, A88).
func currencyScoreSentence(score int) string {
	return "Score " + strconv.Itoa(score) + " out of 100."
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

// reporterSentence is the panel line for a reporter (D-19, A88). A rated
// reporter reads how many of their last judged reports were confirmed, with
// the verb and the noun in the right number. A New reporter reads that they
// are not rated yet and the judged minimum, and the count so far is not in the
// sentence. Neither form calls any report a first report. None and any invalid
// tag give an empty string.
func reporterSentence(t ReporterTag, judged, corroborated int) string {
	switch t {
	case ReporterReliable, ReporterMixed, ReporterUnreliable:
		reports, verb := "reports", "were"
		if judged == 1 {
			reports = "report"
		}
		if corroborated == 1 {
			verb = "was"
		}
		return strconv.Itoa(corroborated) + " of their last " + strconv.Itoa(judged) + " " + reports + " " + verb + " confirmed."
	case ReporterNew:
		return "Not rated yet. It needs " + strconv.Itoa(ReliabilityMinJudged) + " reports with a clear result."
	default:
		return ""
	}
}

// reporterScoreSentence is the popup score line for a rated reporter (A50,
// A88).
func reporterScoreSentence(score int) string {
	return "Score " + strconv.Itoa(score) + " out of 100."
}
