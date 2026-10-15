// textcheck.go is the one general text check (D-06): it runs the local
// gibberish prefilter, then one bounded call to the text screener, and answers
// with a generic outcome. It is reused unchanged by later comment and nickname
// features. The caller layers its own rules (severity, hold, retry) on top of
// the generic outcome; nothing here knows what the text is for.

package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"pinalert/internal/moderation"
)

// TextScreener checks one piece of text. moderation.Screener, *moderation.Client
// and moderation.PassScreener satisfy it. The screener receives only text,
// never an account, session, network address or coordinates (D-01, T-04-04).
// It is declared here, in the consumer, the way handlers.GeocodeSearcher is.
type TextScreener interface {
	Screen(ctx context.Context, text string) (moderation.Result, error)
}

// TextCheckOutcome is the generic answer of CheckText.
type TextCheckOutcome string

const (
	TextClear      TextCheckOutcome = "clear"
	TextBorderline TextCheckOutcome = "borderline"
	TextRejected   TextCheckOutcome = "rejected"
	TextUnfinished TextCheckOutcome = "unfinished"
)

// TextCheck is what CheckText returns. Reason is set only when Outcome is
// TextRejected. Scores and Model come from the provider answer when one
// arrived. RetryAfter is the provider's wait on an unfinished check and zero
// otherwise.
type TextCheck struct {
	Outcome    TextCheckOutcome
	Reason     RejectReason
	Scores     map[string]float64
	Model      string
	RetryAfter time.Duration
}

// InlineScreenTimeout bounds the one attempt made while the person waits. The
// HTTP server write timeout is 10 s and the screener client timeout is 10 s, so
// an inline retry would race them (RESEARCH Area A landmine 1). Retries happen
// in the background (D-01).
const InlineScreenTimeout = 3 * time.Second

// CheckText screens one piece of free text. The order is the specification:
// the trimmed text goes through IsGibberish first (D-05) and a gibberish text
// is refused with no provider call; a nil screener means pass everything, which
// every caller built without a screener gets; otherwise exactly one Screen call
// runs inside InlineScreenTimeout and never repeats, whatever the error. An
// error answers TextUnfinished with the provider's RetryAfter when it gave one.
//
// The scores go through Decide with a constant for its second argument, because
// Decide never reads it (TestDecideIgnoresSeverity in screening_test.go): the
// caller's self declared importance must not decide whether text is screened,
// so abuse is refused at every level (D-03, T-04-02).
func CheckText(ctx context.Context, s TextScreener, text string) TextCheck {
	trimmed := strings.TrimSpace(text)
	if IsGibberish(trimmed) {
		return TextCheck{Outcome: TextRejected, Reason: RejectGibberish}
	}
	if s == nil {
		return TextCheck{Outcome: TextClear}
	}

	cctx, cancel := context.WithTimeout(ctx, InlineScreenTimeout)
	defer cancel()
	res, err := s.Screen(cctx, trimmed)
	if err != nil {
		var retryAfter time.Duration
		prefix := ""
		var ce *moderation.CallError
		if errors.As(err, &ce) {
			retryAfter = ce.RetryAfter
			if !ce.Transient {
				// A bad key or spent quota is a configuration fault the
				// operator must see.
				prefix = "ERROR "
			}
		}
		log.Printf("%sservice: CheckText: check unfinished: %v", prefix, err)
		return TextCheck{Outcome: TextUnfinished, RetryAfter: retryAfter}
	}

	d := Decide(res.Scores, false)
	switch d.Verdict {
	case VerdictReject:
		return TextCheck{Outcome: TextRejected, Reason: d.Reason, Scores: res.Scores, Model: res.Model}
	case VerdictBorderline:
		return TextCheck{Outcome: TextBorderline, Scores: res.Scores, Model: res.Model}
	default:
		return TextCheck{Outcome: TextClear, Scores: res.Scores, Model: res.Model}
	}
}
