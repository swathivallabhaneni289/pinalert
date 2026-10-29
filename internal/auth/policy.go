package auth

import (
	_ "embed"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// The password rule (D-13, IDENT-06): length first, a blocklist, and no rules
// about capitals, digits or symbols, which NIST SP 800-63B-4 advises against.
//
// The minimum is 8 because the owner chose an easy sign up (D-13). NIST SP
// 800-63B-4 asks for 15 when a password is the only factor; that stricter
// option is recorded in the docs plan and is a one constant change here. Because
// the minimum is 8, the embedded list has to cover the common passwords of 8 to
// 14 characters, which is where a short minimum is weakest. The throttle in the
// database and the device bucket offset the rest (threat T-07.1-15, accepted).
const (
	PasswordMinRunes = 8
	PasswordMaxRunes = 128
)

// The typed refusals of CheckPassword. The API turns each into one calm line.
var (
	ErrPasswordTooShort = errors.New("auth: password is shorter than the minimum")
	ErrPasswordTooLong  = errors.New("auth: password is longer than the maximum")
	ErrPasswordCommon   = errors.New("auth: password is too common")
	ErrPasswordIsEmail  = errors.New("auth: password is the email address")
)

//go:embed common_passwords.txt
var commonPasswordsRaw string

// commonPasswords is the lookup set, built once from the embedded list. Lines
// that are blank or start with # are skipped. Entries are lowercase.
var commonPasswords = loadCommonPasswords(commonPasswordsRaw)

func loadCommonPasswords(raw string) map[string]struct{} {
	set := make(map[string]struct{}, 512)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[line] = struct{}{}
	}
	return set
}

// NormalizePassword applies Unicode NFKC, so the same password typed on two
// keyboards (composed or decomposed letters, full width forms) hashes the same.
// Callers normalise before CheckPassword, Hash and Verify.
func NormalizePassword(raw string) string {
	return norm.NFKC.String(raw)
}

// CheckPassword returns nil when password is acceptable, or one of
// ErrPasswordTooShort, ErrPasswordTooLong, ErrPasswordIsEmail or
// ErrPasswordCommon. Length counts runes after NFKC, so eight Devanagari
// letters pass. The password equal to the address, or to its local part, is
// refused case insensitively. One character repeated ("aaaaaaaa") counts as
// common. The common list is matched on the lowercase form, so "Password1" is
// caught; a capital and a symbol do not make a common password acceptable, and
// they are not required of a rare one.
func CheckPassword(password, email string) error {
	p := NormalizePassword(password)
	n := utf8.RuneCountInString(p)
	if n < PasswordMinRunes {
		return ErrPasswordTooShort
	}
	if n > PasswordMaxRunes {
		return ErrPasswordTooLong
	}

	lower := strings.ToLower(p)
	if e := strings.ToLower(strings.TrimSpace(NormalizePassword(email))); e != "" {
		if lower == e {
			return ErrPasswordIsEmail
		}
		if at := strings.LastIndex(e, "@"); at > 0 && lower == e[:at] {
			return ErrPasswordIsEmail
		}
	}

	if oneRuneRepeated(lower) {
		return ErrPasswordCommon
	}
	if _, bad := commonPasswords[lower]; bad {
		return ErrPasswordCommon
	}
	return nil
}

func oneRuneRepeated(s string) bool {
	first, size := utf8.DecodeRuneInString(s)
	if first == utf8.RuneError {
		return false
	}
	for _, r := range s[size:] {
		if r != first {
			return false
		}
	}
	return true
}
