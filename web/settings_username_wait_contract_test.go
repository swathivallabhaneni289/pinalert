package web

// Quick 261108-fku (owner decision 2026-11-08, review fix WR-02): the Username
// window follows the real end of the rename wait. The server writes the moment
// the wait ends and profile-edit.js builds the wait sentence from the days left, so
// the words of that sentence live in two places. This test pins the script's
// words and its gap to the service's, by reading the shipped script from the
// embedded tree and asking the service for the same sentence.

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"pinalert/internal/service"
)

// fkuJSString returns the string a profile-edit.js constant is set to.
func fkuJSString(t *testing.T, js, name string) string {
	t.Helper()
	m := regexp.MustCompile(`var ` + name + ` = '([^'\n]*)';`).FindStringSubmatch(js)
	if m == nil {
		t.Fatalf("profile-edit.js has no var %s set to a single quoted string", name)
	}
	return m[1]
}

// TestSettingsUsernameWaitWords pins the wait sentence the script builds to
// service.UsernameWaitMessage for every day count from 1 to 7 (1 reads "1 day"),
// and the script's gap in days to service.UsernameRenameGap.
func TestSettingsUsernameWaitWords(t *testing.T) {
	js := p421ReadStatic(t, "js/profile-edit.js")
	prefix := fkuJSString(t, js, "WAIT_PREFIX")
	one := fkuJSString(t, js, "WAIT_ONE_DAY")
	many := fkuJSString(t, js, "WAIT_MANY_DAYS")
	for days := 1; days <= 7; days++ {
		tail := many
		if days == 1 {
			tail = one
		}
		got := prefix + strconv.Itoa(days) + tail
		if want := service.UsernameWaitMessage(days); got != want {
			t.Errorf("profile-edit.js builds %q for %d days, the service says %q", got, days, want)
		}
	}

	gap := regexp.MustCompile(`var WAIT_GAP_DAYS = (\d+);`).FindStringSubmatch(js)
	if gap == nil {
		t.Fatalf("profile-edit.js has no var WAIT_GAP_DAYS set to a number")
	}
	days, err := strconv.Atoi(gap[1])
	if err != nil {
		t.Fatalf("WAIT_GAP_DAYS %q is not a number: %v", gap[1], err)
	}
	if time.Duration(days)*24*time.Hour != service.UsernameRenameGap {
		t.Errorf("profile-edit.js gap is %d days, service.UsernameRenameGap is %s", days, service.UsernameRenameGap)
	}

	// The script rounds the days left up, the way the service does.
	for _, want := range []string{
		"Math.max(1, Math.floor((left + DAY_MS - 1) / DAY_MS))",
		"Date.parse(usernameWin.getAttribute('data-wait-until')",
	} {
		if !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(js) {
			t.Errorf("profile-edit.js lacks %s", want)
		}
	}
}

// TestUsernameFieldReadOnlyLook pins the owner finding of 2026-11-09 (07.1 test
// 5): the locked Username box looks locked. While its input is read only the box
// takes the surface fill and the quiet border, keeps the body text colour (muted
// text is kept for --color-bg), and shows no text cursor.
func TestUsernameFieldReadOnlyLook(t *testing.T) {
	css := stripCSSComments(p421ReadStatic(t, "css/chrome.css"))
	rule := regexp.MustCompile(`(?s)\.username-field:has\(\.username-field__input\[readonly\]\)\s*\{([^}]*)\}`).FindStringSubmatch(css)
	if rule == nil {
		t.Fatalf("chrome.css has no rule for the box of a read only Username input")
	}
	p13MustContain(t, "read only box", rule[1],
		"border-color: var(--color-border);", "background: var(--color-surface);", "cursor: default;")
	p13MustNotContain(t, "read only box", rule[1], "color-text-muted", "!important", "opacity")
	p13MustContain(t, "chrome.css", css, ".username-field__input[readonly] {")
}
