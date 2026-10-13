// trustfacts.go builds the facts list of the trust object (A89, A90, plan
// 03-31): the ordered rows the tick mark card shows on the map popup, the feed
// rows and the Activity rows. The server authors every word, so the browser
// holds none. A fact is one title, one detail line and an optional score
// line, with a tone that picks the row's glyph.
//
// The strings follow the closed character set of trustcopy.go: ASCII letters,
// digits, space, comma, full stop and apostrophe, and never the word place.
// They are built only from constants and integers. The still current and the
// reporter rows carry the sentences of the existing signals (the Gone quiet
// row drops the words its title already says), so nothing is recomputed and
// the two cannot drift. The file is pure: it reads a built
// Trust and the visibility and clock BuildTrust already holds.

package service

import (
	"strconv"
	"time"
)

// FactKind names what a fact is about. A kind is left out when it does not
// apply, and the order below is the order of the list.
type FactKind string

const (
	FactStatus        FactKind = "status"
	FactConfirmations FactKind = "confirmations"
	FactDisputes      FactKind = "disputes"
	FactCurrency      FactKind = "currency"
	FactReporter      FactKind = "reporter"
)

// FactKinds is the canonical list of kinds, in list order.
var FactKinds = []FactKind{FactStatus, FactConfirmations, FactDisputes, FactCurrency, FactReporter}

// FactTone picks the glyph of a row. Each tone has its own shape, so colour is
// never the only cue (A93).
type FactTone string

const (
	ToneGood    FactTone = "good"
	ToneNeutral FactTone = "neutral"
	ToneCaution FactTone = "caution"
	ToneBad     FactTone = "bad"
)

// FactTones is the canonical list of tones.
var FactTones = []FactTone{ToneGood, ToneNeutral, ToneCaution, ToneBad}

// TrustFact is one row of the facts list. Detail and ScoreDetail may be
// empty, and the row then shows only its title. ScoreDetail is shown in the
// popup only (A50, D-09).
type TrustFact struct {
	Kind        FactKind
	Tone        FactTone
	Title       string
	Detail      string
	ScoreDetail string
}

// The fixed words of the facts. The count dependent ones are built below.
const (
	factTitleExpired        = "Expired"
	factDetailExpired       = "It can no longer be confirmed."
	factTitleNotConfirmed   = "Not confirmed yet"
	factTitleNoDisputes     = "No disputes"
	factDetailNoDisputes    = "No one nearby has disputed this."
	factDetailSomeoneWrong  = "Someone nearby says this is wrong."
	factDetailDifferentSpot = "At different spots nearby."
	factTitleStillHappening = "Still happening"
	factTitleGettingOld     = "Getting old"
	factTitleGoneQuiet      = "Gone quiet"
	factDetailGoneQuiet     = "People nearby can confirm it again."
	factTitleNeedsMore      = "Needs more confirmations"
	factTitleTooEarly       = "Too early to tell"
)

// peoplePhrase is a count of people with its noun in the right number: 1
// person, 2 people.
func peoplePhrase(n int) string {
	if n == 1 {
		return "1 person"
	}
	return strconv.Itoa(n) + " people"
}

// needsDetail is the line that says how many more people a report needs. It
// is the threshold form while nobody has confirmed, and the more form after
// that. The number needed is clamped to 1 or more, so the line stays plain
// even for a count at the threshold, which the caller never asks for.
func needsDetail(confirmed int) string {
	need := IndependentAgreementThreshold - confirmed
	switch {
	case confirmed <= 0:
		return "Needs " + strconv.Itoa(IndependentAgreementThreshold) + " people at different spots."
	case need <= 1:
		return "Needs 1 more person at a different spot."
	default:
		return "Needs " + strconv.Itoa(need) + " more people at different spots."
	}
}

// confirmationsFact is the row for the confirming count n. The count is of
// distinct spots, a lower bound on people (D-04), and the row says people at
// different spots. needsShown is false for a report nobody can still act on.
func confirmationsFact(n int, needsShown bool) TrustFact {
	f := TrustFact{Kind: FactConfirmations}
	switch {
	case n <= 0:
		f.Tone, f.Title = ToneNeutral, factTitleNotConfirmed
	case n < IndependentAgreementThreshold:
		f.Tone, f.Title = ToneNeutral, "Confirmed by "+peoplePhrase(n)
	default:
		f.Tone, f.Title, f.Detail = ToneGood, "Confirmed by "+peoplePhrase(n), factDetailDifferentSpot
		return f
	}
	if needsShown {
		f.Detail = needsDetail(n)
	}
	return f
}

// disputesFact is the row for the disputing count m, on the shape of the
// confirmations row.
func disputesFact(m int) TrustFact {
	f := TrustFact{Kind: FactDisputes}
	switch {
	case m <= 0:
		f.Tone, f.Title, f.Detail = ToneGood, factTitleNoDisputes, factDetailNoDisputes
	case m == 1:
		f.Tone, f.Title, f.Detail = ToneBad, "Disputed by "+peoplePhrase(m), factDetailSomeoneWrong
	default:
		f.Tone, f.Title, f.Detail = ToneBad, "Disputed by "+peoplePhrase(m), factDetailDifferentSpot
	}
	return f
}

// raisedBar reports whether a standing dispute raised the bar of the signal.
// It is read from the signal itself: a getting old or needs re-confirming
// band carries the plain sentence of its band unless a dispute raised the bar,
// so any other sentence is the raised bar sentence (D-19).
func raisedBar(c CurrencySignal) bool {
	switch c.Band {
	case CurrencyGettingOld:
		return c.Sentence != gettingOldSentence(windowPhrase(c.WindowMinutes))
	case CurrencyNeedsReconfirming:
		return c.Sentence != needsReconfirmingSentence
	default:
		return false
	}
}

// currencyFact is the still current row. It exists exactly when the signal
// shows a word, and it carries the signal's own sentence and score sentence.
func currencyFact(c CurrencySignal) (TrustFact, bool) {
	if c.Label == "" {
		return TrustFact{}, false
	}
	f := TrustFact{Kind: FactCurrency, Detail: c.Sentence, ScoreDetail: c.ScoreSentence}
	switch {
	case c.Band == CurrencyNotYetCorroborated:
		f.Tone, f.Title = ToneNeutral, factTitleTooEarly
	case c.Band == CurrencyUpToDate:
		f.Tone, f.Title = ToneGood, factTitleStillHappening
	case raisedBar(c):
		f.Tone, f.Title = ToneCaution, factTitleNeedsMore
	case c.Band == CurrencyGettingOld:
		f.Tone, f.Title = ToneCaution, factTitleGettingOld
	case c.Band == CurrencyNeedsReconfirming:
		// The signal's sentence starts with the words Gone quiet, which the
		// row title already says.
		f.Tone, f.Title, f.Detail = ToneBad, factTitleGoneQuiet, factDetailGoneQuiet
	default:
		return TrustFact{}, false
	}
	return f, true
}

// reporterFact is the reporter row. It is left out for the none signal and
// for any tag outside the closed set.
func reporterFact(r ReporterSignal) (TrustFact, bool) {
	var tone FactTone
	switch r.Tag {
	case ReporterReliable:
		tone = ToneGood
	case ReporterMixed:
		tone = ToneCaution
	case ReporterUnreliable:
		tone = ToneBad
	case ReporterNew:
		tone = ToneNeutral
	default:
		return TrustFact{}, false
	}
	return TrustFact{
		Kind: FactReporter, Tone: tone, Title: reporterLabel(r.Tag),
		Detail: r.Sentence, ScoreDetail: r.ScoreSentence,
	}, true
}

// buildTrustFacts is the one builder of the list. Retracted wins over expired,
// the same precedence as the lead sentence, so a resolved report reads as
// resolved and not as expired. The list always holds the two counter rows.
func buildTrustFacts(t Trust, vis Visibility, expiresAt, now time.Time) []TrustFact {
	past := !expiresAt.After(now)
	quiet := vis == VisibilityHidden || vis == VisibilityRetracted || past

	facts := make([]TrustFact, 0, 5)
	if past && vis != VisibilityRetracted {
		facts = append(facts, TrustFact{Kind: FactStatus, Tone: ToneNeutral, Title: factTitleExpired, Detail: factDetailExpired})
	}
	facts = append(facts, confirmationsFact(t.ConfirmedPlaces, !quiet), disputesFact(t.DisputedPlaces))
	if f, ok := currencyFact(t.Currency); ok {
		facts = append(facts, f)
	}
	if f, ok := reporterFact(t.Reporter); ok {
		facts = append(facts, f)
	}
	return facts
}
