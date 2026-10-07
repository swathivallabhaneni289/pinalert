// trustcopy_test.go declares package service (the internal test package)
// because it reaches the unexported trustCountText and the sentence builders,
// and it reuses trust_test.go's acctPtr. assertPlainCopy is reused by plans
// 03-08 and 03-10, so its signature must stay exact.

package service

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// assertPlainCopy fails the test when s leaves the closed character set of
// server authored copy (T-03-07, V-32): ASCII letters, digits, space and the
// marks , . ' + - with the hyphen allowed only inside re-confirming, and no
// leading, trailing or doubled space. An empty string passes, because
// LeadSentence is legitimately empty.
func assertPlainCopy(t *testing.T, s string) {
	t.Helper()
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == ' ' || r == ',' || r == '.' || r == '\'' || r == '+' || r == '-'
		if !ok {
			t.Errorf("copy %q holds the character %q (U+%04X), outside the closed set", s, r, r)
		}
	}
	if strings.Contains(strings.ReplaceAll(s, "re-confirming", ""), "-") {
		t.Errorf("copy %q holds a hyphen outside re-confirming", s)
	}
	if strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		t.Errorf("copy %q has a leading or trailing space", s)
	}
	if strings.Contains(s, "  ") {
		t.Errorf("copy %q has a double space", s)
	}
}

// p05DistinctRows returns n rows on report 1, each from its own account in
// its own cell, with the given value.
func p05DistinctRows(n int, value VoteValue, firstAccount int64) []sqlcgen.CurrentVotesForReportsRow {
	rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, p05VoteAt(1, firstAccount+int64(i), VoteKindContent, value, fmt.Sprintf("p05cell%s%d", value, i), p05Now))
	}
	return rows
}

// p05StringFields collects every string field of v, walking nested structs.
func p05StringFields(v reflect.Value, path string, out map[string]string) {
	switch v.Kind() {
	case reflect.String:
		out[path] = v.String()
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			p05StringFields(v.Field(i), path+"."+v.Type().Field(i).Name, out)
		}
	case reflect.Pointer:
		if !v.IsNil() {
			p05StringFields(v.Elem(), path, out)
		}
	}
}

// TestTrustCountLabel is V-12: the capped numerals are always present, and
// the panel sentence carries the real number above the cap.
func TestTrustCountLabel(t *testing.T) {
	for n, want := range map[int]string{0: "0", 1: "1", 2: "2", 99: "99", 100: "99+", 101: "99+", 1000: "99+"} {
		if got := trustCountText(n); got != want {
			t.Errorf("trustCountText(%d) = %q, want %q", n, got, want)
		}
	}

	expires := p05Now.Add(time.Hour)
	t.Run("no rows shows both counters", func(t *testing.T) {
		got := BuildTrust(p05Input(nil, nil, VisibilityProvisional, expires))
		if got.ConfirmedText != "0" || got.DisputedText != "0" {
			t.Errorf("texts = %q and %q, want 0 and 0", got.ConfirmedText, got.DisputedText)
		}
	})
	t.Run("zero confirms and one dispute", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{p05VoteAt(1, 20, VoteKindContent, VoteDispute, "cellX", p05Now)}
		got := BuildTrust(p05Input(rows, nil, VisibilityProvisional, expires))
		if got.ConfirmedText != "0" || got.DisputedText != "1" {
			t.Errorf("texts = %q and %q, want 0 and 1", got.ConfirmedText, got.DisputedText)
		}
	})
	t.Run("150 confirming places cap the numeral but not the sentence", func(t *testing.T) {
		got := BuildTrust(p05Input(p05DistinctRows(150, VoteConfirm, 100), nil, VisibilityLive, expires))
		if got.ConfirmedPlaces != 150 || got.ConfirmedText != "99+" {
			t.Errorf("ConfirmedPlaces = %d, ConfirmedText = %q", got.ConfirmedPlaces, got.ConfirmedText)
		}
		want := "150 people in 150 different places confirmed this. Counts separate places, not votes."
		if got.ConfirmedSentence != want {
			t.Errorf("ConfirmedSentence = %q, want %q", got.ConfirmedSentence, want)
		}
	})
}

// TestTrustCounterSentences pins the confirmed and disputed sentences
// exactly (RR-1, A7, A56).
func TestTrustCounterSentences(t *testing.T) {
	const tail = " Counts separate places, not votes."
	confirmed := map[int]string{
		0:   "No one has confirmed this yet." + tail,
		1:   "1 person in 1 place confirmed this." + tail,
		2:   "2 people in 2 different places confirmed this." + tail,
		100: "100 people in 100 different places confirmed this." + tail,
	}
	disputed := map[int]string{
		0:   "No one nearby has disputed this." + tail,
		1:   "1 person in 1 place disputed this." + tail,
		2:   "2 people in 2 different places disputed this." + tail,
		100: "100 people in 100 different places disputed this." + tail,
	}
	for n, want := range confirmed {
		if got := confirmedSentence(n); got != want {
			t.Errorf("confirmedSentence(%d) = %q, want %q", n, got, want)
		}
	}
	for n, want := range disputed {
		if got := disputedSentence(n); got != want {
			t.Errorf("disputedSentence(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestTrustCopyIsPlainText is V-32: every string the trust code can produce
// stays inside the closed character set.
func TestTrustCopyIsPlainText(t *testing.T) {
	const tail = "Counts separate places, not votes."
	for _, n := range []int{0, 1, 2, 3, 10, 99, 100, 101, 999, 12345} {
		assertPlainCopy(t, trustCountText(n))
		c, d := confirmedSentence(n), disputedSentence(n)
		assertPlainCopy(t, c)
		assertPlainCopy(t, d)
		if !strings.HasSuffix(c, tail) || !strings.HasSuffix(d, tail) {
			t.Errorf("a counter sentence for %d lost its caveat: %q, %q", n, c, d)
		}
	}
	assertPlainCopy(t, provisionalLeadSentence())
	assertPlainCopy(t, expiredLeadSentence)
	assertPlainCopy(t, TooFarMessage())

	rowSets := map[string][]sqlcgen.CurrentVotesForReportsRow{
		"none":        nil,
		"one confirm": p05DistinctRows(1, VoteConfirm, 100),
		"two confirms and one dispute": append(p05DistinctRows(2, VoteConfirm, 100),
			p05DistinctRows(1, VoteDispute, 300)...),
		"150 confirming and 120 disputing": append(p05DistinctRows(150, VoteConfirm, 1000),
			p05DistinctRows(120, VoteDispute, 3000)...),
	}
	for _, vis := range Visibilities {
		for _, expires := range []time.Time{p05Now.Add(time.Hour), p05Now.Add(-time.Hour)} {
			for name, rows := range rowSets {
				fields := map[string]string{}
				p05StringFields(reflect.ValueOf(BuildTrust(p05Input(rows, nil, vis, expires))), "Trust", fields)
				if len(fields) == 0 {
					t.Fatal("no string fields found on Trust")
				}
				for path, s := range fields {
					t.Run(fmt.Sprintf("%s/%s/%v/%s", vis, name, expires.After(p05Now), path), func(t *testing.T) {
						assertPlainCopy(t, s)
					})
				}
			}
		}
	}
}
