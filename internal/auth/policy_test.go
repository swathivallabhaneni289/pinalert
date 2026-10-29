package auth

import (
	"bufio"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCheckPassword(t *testing.T) {
	const email = "Ravi.Kumar@example.com"
	cases := []struct {
		name     string
		password string
		email    string
		want     error
	}{
		{"seven characters", "qzxwvbn", email, ErrPasswordTooShort},
		{"eight characters", "qzxwvbnm", email, nil},
		{"empty", "", email, ErrPasswordTooShort},
		{"128 characters", strings.Repeat("ab", 64), email, nil},
		{"129 characters", strings.Repeat("ab", 64) + "c", email, ErrPasswordTooLong},
		{"eight devanagari letters", "कखगघङचछज", email, nil},
		{"seven devanagari letters", "कखगघङचछ", email, ErrPasswordTooShort},
		{"common password", "password", email, ErrPasswordCommon},
		{"common with capital and digit", "Password1", email, ErrPasswordCommon},
		{"common digits", "12345678", email, ErrPasswordCommon},
		{"common upper case", "QWERTYUIOP", email, ErrPasswordCommon},
		{"one character repeated", "aaaaaaaa", email, ErrPasswordCommon},
		{"one digit repeated", "77777777777", email, ErrPasswordCommon},
		{"capital and symbol alone do not help", "Password!", email, nil},
		{"the full address", "ravi.kumar@example.com", email, ErrPasswordIsEmail},
		{"the full address in other case", "RAVI.KUMAR@EXAMPLE.COM", email, ErrPasswordIsEmail},
		{"the local part", "RAVI.kumar", email, ErrPasswordIsEmail},
		{"local part with padding is a different word", "ravi.kumar1982", email, nil},
		{"fullwidth letters fold to ASCII", "ｐａｓｓｗｏｒｄ", email, ErrPasswordCommon},
		{"no email given", "tr0ub4dor&3", "", nil},
		{"fullwidth too long is counted after folding", strings.Repeat("ａｂ", 64) + "ｃ", email, ErrPasswordTooLong},
		{"a few words in a row", "blue tiffin carrier", email, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckPassword(tc.password, tc.email)
			if !errors.Is(got, tc.want) || (tc.want == nil && got != nil) {
				t.Fatalf("CheckPassword(%q) = %v, want %v", tc.password, got, tc.want)
			}
		})
	}

	if PasswordMinRunes != 8 || PasswordMaxRunes != 128 {
		t.Fatalf("limits %d..%d, want 8..128", PasswordMinRunes, PasswordMaxRunes)
	}
}

func TestNormalizePassword(t *testing.T) {
	// U+FB01 (fi ligature) folds to "fi" under NFKC.
	if got := NormalizePassword("ﬁsh"); got != "fish" {
		t.Fatalf("NormalizePassword = %q, want fish", got)
	}
	// Normalising twice changes nothing.
	once := NormalizePassword("école")
	if NormalizePassword(once) != once {
		t.Fatal("NormalizePassword is not idempotent")
	}
}

func TestCommonPasswordList(t *testing.T) {
	f, err := os.Open("common_passwords.txt")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			if !strings.HasPrefix(line, "#") {
				t.Errorf("first line must be a # comment naming the source, got %q", line)
			}
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len([]rune(line)) < 8 {
			t.Errorf("entry %q is shorter than 8 characters", line)
		}
		if line != strings.ToLower(line) {
			t.Errorf("entry %q is not lowercase", line)
		}
		if line != strings.TrimSpace(line) {
			t.Errorf("entry %q has surrounding space", line)
		}
		if seen[line] {
			t.Errorf("entry %q is duplicated", line)
		}
		seen[line] = true
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) < 300 {
		t.Fatalf("list has %d entries, want at least 300", len(seen))
	}
	for _, must := range []string{"password", "12345678", "qwertyuiop", "iloveyou", "11111111",
		"password1", "sunshine", "1q2w3e4r", "princess", "baseball"} {
		if !seen[must] {
			t.Errorf("list is missing %q", must)
		}
	}
	// The loaded map matches the file.
	if len(commonPasswords) != len(seen) {
		t.Errorf("loaded %d entries, file has %d", len(commonPasswords), len(seen))
	}
}
