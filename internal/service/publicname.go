// publicname.go checks a Name or a username before it becomes public (Phase 9,
// plan 09-04, research gap G1, 07.1 D-16). A Name used to be shown to its owner
// only; the moment "Let others follow me" is on, other people read it, so it
// goes through the reserved word check (a Name must not look like an agency)
// and the abuse screen. Both fail closed: an abuse check that cannot finish
// refuses with a try again line and never lets the Name through.

package service

import (
	"context"
	"strings"
)

// The four sentences of the public name checks, word for word from the UI-SPEC
// (P9-C18) and the plan.
const (
	// PublicNameRefusedMessage refuses "Let others follow me" for a Name that
	// failed the reserved word check or the abuse screen.
	PublicNameRefusedMessage = "Your name can't be shown on a public profile. Change it with Edit profile."
	// PublicUsernameRefusedMessage refuses it for a missing, borderline or
	// rejected username.
	PublicUsernameRefusedMessage = "Your username can't be shown on a public profile. Change it with Edit profile."
	// PublicNameUnfinishedMessage answers a check that could not finish.
	PublicNameUnfinishedMessage = "Couldn't check your name just now. Try again."
	// PublicNameEditRefusedMessage refuses a Name edit while the profile is on.
	PublicNameEditRefusedMessage = "That name can't be shown on your public profile. Try another."
)

// NameReserved reports whether a Name looks official. The Name is lowered to
// ASCII, every rune that is not an ASCII letter or digit becomes an underscore,
// repeats collapse and the ends are trimmed, and the joined form goes to
// UsernameReserved, which splits it into tokens itself and also tests the whole
// string without separators. So "N.D.M.A" (ndma), "NDMA Control Room" and
// "IMD Hyderabad" are reserved while "Ravi Kumar" and "Imdad Ali" are not.
//
// A Name in another script is not compared with the reserved lists, because
// the lists are ASCII: every such rune folds to a separator, so a Name made only
// of them is never reserved and the abuse screen is its only check. That limit
// is recorded for docs/TRUST-MODEL.md in plan 09-27.
func NameReserved(name string) bool {
	var b strings.Builder
	lastUnderscore := true // so leading separators are dropped
	for _, r := range asciiLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	folded := strings.TrimRight(b.String(), "_")
	if folded == "" {
		return false
	}
	return UsernameReserved(folded)
}

// PublicNameResult is how ScreenPublicName judged a Name.
type PublicNameResult int

const (
	// PublicNameClear: the Name may be shown.
	PublicNameClear PublicNameResult = iota
	// PublicNameRefused: reserved, borderline or rejected.
	PublicNameRefused
	// PublicNameUnfinished: the abuse check could not finish. It fails closed.
	PublicNameUnfinished
)

// ScreenPublicName judges a Name that is about to become public. The reserved
// word check runs first and refuses without calling the screener. Then one
// ScreenAbuse call runs (never the gibberish prefilter, which refuses a one
// word Name): clear passes, borderline and rejected refuse, and an unfinished
// check, an error or the inline timeout is PublicNameUnfinished. A nil
// screener passes a Name that is not reserved, as every other caller built
// without a screener does.
func ScreenPublicName(ctx context.Context, screener TextScreener, name string) PublicNameResult {
	if NameReserved(name) {
		return PublicNameRefused
	}
	switch ScreenAbuse(ctx, screener, name).Outcome {
	case TextClear:
		return PublicNameClear
	case TextUnfinished:
		return PublicNameUnfinished
	default:
		return PublicNameRefused
	}
}
