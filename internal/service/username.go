// username.go holds the pure rules of a username (Phase 7.1, D-07, D-11):
// normalising what was typed, the shape rule, the reserved names and the text
// sent to the screening provider. Nothing here touches the database or HTTP.
// The shape rule equals the database CHECK on accounts.username.

package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// UsernameMinLen and UsernameMaxLen bound a username in characters.
	UsernameMinLen = 3
	UsernameMaxLen = 20
	// UsernameRenameGap is how long a person waits between username changes.
	UsernameRenameGap = 7 * 24 * time.Hour
)

// The username sentences, in the plain words of the UI-SPEC.
const (
	UsernameRuleMessage        = "Use 3 to 20 letters, numbers, dots or underscores."
	UsernameReservedMessage    = "That name is kept for official use."
	UsernameRefusedMessage     = "That username can't be used. Try another."
	UsernameYoursMessage       = "This is your username now."
	UsernameCheckFailedMessage = "Couldn't check that username. Try again."
)

// UsernameTakenMessage is the sentence for a name another account holds.
func UsernameTakenMessage(name string) string { return name + " is taken. Try another." }

// UsernameAvailableMessage is the sentence for a name nobody holds.
func UsernameAvailableMessage(name string) string { return name + " is available." }

// UsernameWaitMessage is the sentence for a change attempted inside the
// rename gap, with the days left (1 reads "1 day").
func UsernameWaitMessage(days int) string {
	unit := "days"
	if days == 1 {
		unit = "day"
	}
	return fmt.Sprintf("You can change your username again in %d %s.", days, unit)
}

// UsernameWaitAfterChangeMessage is the wait sentence for the whole rename gap,
// in days. The Settings page carries it so the Username window can lock on the
// next opening after a change saved in the page, with no reload: a change just
// made starts a full gap. Quick 261108-fku (owner decision 2026-11-08).
func UsernameWaitAfterChangeMessage() string {
	return UsernameWaitMessage(int(UsernameRenameGap / (24 * time.Hour)))
}

// UsernameClass is how ClassifyUsername judges a typed username.
type UsernameClass int

const (
	// UsernameUsable passes the shape rule and is not reserved.
	UsernameUsable UsernameClass = iota
	// UsernameInvalid breaks the shape rule.
	UsernameInvalid
	// UsernameIsReserved has a valid shape but looks official.
	UsernameIsReserved
)

// usernameShape is the database CHECK: lowercase letters, digits, underscore
// and full stop, 3 to 20 of them. Letters are ASCII only, so no hidden,
// markup or look alike Unicode characters can be stored (T-07.1-20).
var usernameShape = regexp.MustCompile(`^[a-z0-9_.]{3,20}$`)

// asciiLower lowers A to Z only; any other rune is kept as typed so a non ASCII
// name still fails the shape rule instead of being folded into something else.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// NormalizeUsername trims spaces, drops one leading @ and folds ASCII capitals.
func NormalizeUsername(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "@")
	return asciiLower(s)
}

func hasASCIILetter(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			return true
		}
	}
	return false
}

func usernameShapeOK(s string) bool {
	return usernameShape.MatchString(s) && hasASCIILetter(s) &&
		!strings.HasPrefix(s, ".") && !strings.HasSuffix(s, ".") && !strings.Contains(s, "..")
}

// ClassifyUsername normalises raw and judges it. The shape check runs first,
// then the reserved check, so a reserved verdict always means a valid shape.
// The returned name is the normalised form whatever the class.
func ClassifyUsername(raw string) (name string, class UsernameClass) {
	name = NormalizeUsername(raw)
	if !usernameShapeOK(name) {
		return name, UsernameInvalid
	}
	if UsernameReserved(name) {
		return name, UsernameIsReserved
	}
	return name, UsernameUsable
}

// These two lists are the D-11 numbers shown at the owner viewing (UI-SPEC
// default 4). Phases 8 and 9 call UsernameReserved, so a change here changes
// every place a public name is checked.
var (
	// reservedExactWords match only a whole name or a whole token between
	// separators, so imdone, modest and helpful stay free.
	reservedExactWords = []string{"imd", "gov", "mod", "admin", "help", "support", "100", "112"}
	// reservedBlockedWords match as the start of the whole name or as a whole
	// token, so police_hyd, hyd.police and firefly are refused.
	reservedBlockedWords = []string{
		"police", "ndma", "ndrf", "sdma", "sdrf", "govt", "government", "official",
		"pinalert", "moderator", "administrator", "emergency", "fire", "ambulance",
	}
	// reservedAnywhereWords match anywhere in the whole name without
	// separators (Phase 7.1 review WR-S03), because the usual shape of an
	// official handle is a place before the word: hydcitypolice, apsdma,
	// vijayawadapolice. They are the long, unambiguous blocked words; fire,
	// emergency and ambulance keep the start or token rule because they sit
	// inside ordinary words. The few false positives (policeman, unofficial)
	// are accepted; the owner can widen or narrow the list at the viewing.
	reservedAnywhereWords = []string{
		"police", "ndma", "ndrf", "sdma", "sdrf", "govt", "government", "official",
		"pinalert", "moderator", "administrator",
	}
)

// maxOneVariants caps the mixed i and l spellings of the digit 1: with more
// ones than this only the all-i and all-l spellings are tried, so the work for
// a 20 character name stays small.
const maxOneVariants = 8

// foldVariants returns the spellings of s to test: the name as typed, and the
// look alike digit folding (0 to o, 3 to e, 4 to a, 5 to s, 7 to t) with each 1
// read as i or as l.
func foldVariants(s string) []string {
	base := []byte(s)
	var ones []int
	for i, c := range base {
		switch c {
		case '0':
			base[i] = 'o'
		case '3':
			base[i] = 'e'
		case '4':
			base[i] = 'a'
		case '5':
			base[i] = 's'
		case '7':
			base[i] = 't'
		case '1':
			ones = append(ones, i)
		}
	}
	out := []string{s}
	if len(ones) == 0 {
		return append(out, string(base))
	}
	if len(ones) > maxOneVariants {
		for _, r := range []byte{'i', 'l'} {
			v := append([]byte(nil), base...)
			for _, i := range ones {
				v[i] = r
			}
			out = append(out, string(v))
		}
		return out
	}
	for mask := 0; mask < 1<<len(ones); mask++ {
		v := append([]byte(nil), base...)
		for bit, i := range ones {
			if mask&(1<<bit) != 0 {
				v[i] = 'l'
			} else {
				v[i] = 'i'
			}
		}
		out = append(out, string(v))
	}
	return out
}

func inList(list []string, w string) bool {
	for _, x := range list {
		if x == w {
			return true
		}
	}
	return false
}

// UsernameReserved reports whether a name looks official. A name is refused
// when, in any digit folding, the whole name without separators or any token
// between separators equals an exact word, or the whole name starts with a
// blocked word, or any token equals a blocked word, or the whole name contains
// one of the long official words anywhere (review WR-S03). So p0lice,
// police_hyd, ndma.india, hydcitypolice, apsdma and adm1n are refused while
// imdone is kept (D-07, T-07.1-16).
// Phases 8 and 9 call it too.
func UsernameReserved(name string) bool {
	name = asciiLower(name)
	for _, v := range foldVariants(name) {
		whole := strings.NewReplacer("_", "", ".", "").Replace(v)
		tokens := strings.FieldsFunc(v, func(r rune) bool { return r == '_' || r == '.' })
		if inList(reservedExactWords, whole) {
			return true
		}
		for _, t := range tokens {
			if inList(reservedExactWords, t) || inList(reservedBlockedWords, t) {
				return true
			}
		}
		for _, w := range reservedBlockedWords {
			if strings.HasPrefix(whole, w) {
				return true
			}
		}
		for _, w := range reservedAnywhereWords {
			if strings.Contains(whole, w) {
				return true
			}
		}
	}
	return false
}

// UsernameScreenText is the text sent to the screening provider for a
// username: underscores and full stops become spaces, because the provider
// reads words. A username is screened with ScreenAbuse, never with the
// gibberish checking text screen, because that prefilter rejects a single
// token such as a name (research Pitfall 1).
func UsernameScreenText(name string) string {
	return strings.NewReplacer("_", " ", ".", " ").Replace(name)
}
