package web

// Quick 261108-fku (owner decision 2026-11-08, review fix WR-02): the Username
// window follows the real end of the rename wait. The server writes the moment
// the wait ends and settings.js builds the wait sentence from the days left, so
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

// fkuJSString returns the string a settings.js constant is set to.
func fkuJSString(t *testing.T, js, name string) string {
	t.Helper()
	m := regexp.MustCompile(`var ` + name + ` = '([^'\n]*)';`).FindStringSubmatch(js)
	if m == nil {
		t.Fatalf("settings.js has no var %s set to a single quoted string", name)
	}
	return m[1]
}

// TestSettingsUsernameWaitWords pins the wait sentence the script builds to
// service.UsernameWaitMessage for every day count from 1 to 7 (1 reads "1 day"),
// and the script's gap in days to service.UsernameRenameGap.
func TestSettingsUsernameWaitWords(t *testing.T) {
	js := p421ReadStatic(t, "js/settings.js")
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
			t.Errorf("settings.js builds %q for %d days, the service says %q", got, days, want)
		}
	}

	gap := regexp.MustCompile(`var WAIT_GAP_DAYS = (\d+);`).FindStringSubmatch(js)
	if gap == nil {
		t.Fatalf("settings.js has no var WAIT_GAP_DAYS set to a number")
	}
	days, err := strconv.Atoi(gap[1])
	if err != nil {
		t.Fatalf("WAIT_GAP_DAYS %q is not a number: %v", gap[1], err)
	}
	if time.Duration(days)*24*time.Hour != service.UsernameRenameGap {
		t.Errorf("settings.js gap is %d days, service.UsernameRenameGap is %s", days, service.UsernameRenameGap)
	}

	// The script rounds the days left up, the way the service does.
	for _, want := range []string{
		"Math.max(1, Math.floor((left + DAY_MS - 1) / DAY_MS))",
		"Date.parse(usernameWin.getAttribute('data-wait-until')",
	} {
		if !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(js) {
			t.Errorf("settings.js lacks %s", want)
		}
	}
}
