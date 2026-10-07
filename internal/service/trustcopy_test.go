// trustcopy_test.go declares package service (the internal test package)
// because it reaches the unexported trustCountText and the sentence builders,
// and it reuses trust_test.go's acctPtr. assertPlainCopy is reused by plans
// 03-08 and 03-10, so its signature must stay exact.

package service

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strconv"
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

// p05StringFields collects every copy string field of v, walking nested
// structs. Fields of a named string type, such as CurrencyBand and FadeStage,
// are enum slugs that go on the wire as identifiers, not authored copy, so
// they are skipped (plan 03-08).
func p05StringFields(v reflect.Value, path string, out map[string]string) {
	switch v.Kind() {
	case reflect.String:
		if v.Type() == reflect.TypeOf("") {
			out[path] = v.String()
		}
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

// TestTrustCopyCurrencyIsPlainText is the V-32 extension for the still current
// copy (A50, D-19, T-03-07): the exact strings, the builder grid and every
// currency string BuildTrust can produce stay inside the closed set.
func TestTrustCopyCurrencyIsPlainText(t *testing.T) {
	t.Run("exact copy", func(t *testing.T) {
		exact := []struct{ got, want string }{
			{currencyLabelUpToDate, "Up to date"},
			{currencyLabelGettingOld, "Getting old"},
			{currencyLabelNeedsReconfirming, "Needs re-confirming"},
			{currencyLabelTooEarly, "Too early to tell"},
			{upToDateSentence(2, "2 hours"), "2 different places confirmed this in the last 2 hours."},
			{gettingOldSentence("6 hours"), "Confirmed earlier, but fewer than 2 places confirmed it in the last 6 hours."},
			{needsReconfirmingSentence, "Confirmed earlier, but it has gone quiet. People nearby can confirm it again."},
			{disputeRaisedBarSentence(3, "2 hours"), "Someone nearby disputed this, so it needs 3 places to confirm it in the last 2 hours."},
			{tooEarlySentence(), "Fewer than 2 places have confirmed this so far."},
			{currencyScoreSentence(82), "Score 82 out of 100, based on how recently different places confirmed this."},
		}
		for _, e := range exact {
			if e.got != e.want {
				t.Errorf("copy = %q, want %q", e.got, e.want)
			}
		}
		threshold := " " + strconv.Itoa(IndependentAgreementThreshold) + " "
		for _, s := range []string{gettingOldSentence("2 hours"), tooEarlySentence(), upToDateSentence(IndependentAgreementThreshold, "2 hours")} {
			if !strings.Contains(s, threshold) && !strings.HasPrefix(s, strings.TrimSpace(threshold)+" ") {
				t.Errorf("copy %q does not carry the threshold", s)
			}
		}
	})

	t.Run("builder grid", func(t *testing.T) {
		var phrases []string
		for _, m := range []int{0, 1, 2, 45, 59, 60, 61, 90, 120, 360, 1440} {
			p := windowPhrase(m)
			assertPlainCopy(t, p)
			phrases = append(phrases, p)
		}
		for _, p := range phrases {
			for _, r := range []int{2, 3, 10, 99, 100, 12345} {
				assertPlainCopy(t, upToDateSentence(r, p))
			}
			assertPlainCopy(t, gettingOldSentence(p))
			for _, k := range []int{3, 4, 10, 100} {
				assertPlainCopy(t, disputeRaisedBarSentence(k, p))
			}
		}
		for _, s := range []int{0, 1, 12, 49, 50, 62, 74, 75, 93, 99, 100} {
			assertPlainCopy(t, currencyScoreSentence(s))
		}
		assertPlainCopy(t, tooEarlySentence())
		assertPlainCopy(t, needsReconfirmingSentence)
		for _, b := range CurrencyBands {
			assertPlainCopy(t, currencyLabel(b))
		}
	})

	t.Run("BuildTrust grid", func(t *testing.T) {
		metas := []ReportMeta{p08Low, p08Medium, p08Critical, p08RescueLow}
		for _, vis := range Visibilities {
			for _, meta := range metas {
				life := ExpiryDuration(meta.Severity)
				window := life / CurrencyWindowDivisor
				expires := p08Created.Add(life)
				inLife := p08Created.Add(3*window + time.Minute)

				at := func(back time.Duration) time.Time { return inLife.Add(-back) }
				confirm := func(account int64, cell string, when time.Time) sqlcgen.CurrentVotesForReportsRow {
					return p05VoteAt(1, account, VoteKindContent, VoteConfirm, cell, when)
				}
				dispute := func(account int64, cell string, when time.Time) sqlcgen.CurrentVotesForReportsRow {
					return p05VoteAt(1, account, VoteKindContent, VoteDispute, cell, when)
				}
				twoAt := func(back time.Duration) []sqlcgen.CurrentVotesForReportsRow {
					return []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", at(back)), confirm(102, "g2", at(back))}
				}
				rowSets := map[string][]sqlcgen.CurrentVotesForReportsRow{
					"none":                nil,
					"one confirm":         {confirm(101, "g1", at(0))},
					"two fresh":           twoAt(0),
					"two 1.5 windows old": twoAt(window * 3 / 2),
					"two 3 windows old":   twoAt(3 * window),
					"three and a dispute": {
						confirm(101, "g1", at(0)), confirm(102, "g2", at(time.Minute)),
						confirm(103, "g3", at(2*time.Minute)), dispute(201, "g4", at(time.Minute)),
					},
					"two and a dispute": append(twoAt(0), dispute(201, "g4", at(time.Minute))),
				}
				for name, rows := range rowSets {
					for _, now := range []time.Time{inLife, expires} {
						in := TrustInput{
							Rows: rows, ReportID: 1, Meta: meta, Visibility: vis,
							CreatedAt: p08Created, ExpiresAt: expires, Now: now,
						}
						cur := BuildTrust(in).Currency
						label := fmt.Sprintf("%s/%v/%s/%v", vis, meta, name, now.Equal(expires))
						for _, s := range []string{cur.Label, cur.Sentence, cur.ScoreSentence} {
							assertPlainCopy(t, s)
							for _, banned := range []string{"fades", "first report", "/100"} {
								if strings.Contains(s, banned) {
									t.Errorf("%s: copy %q contains %q", label, s, banned)
								}
							}
						}
						if (cur.Label == "") != (cur.Sentence == "") {
							t.Errorf("%s: Label %q and Sentence %q disagree", label, cur.Label, cur.Sentence)
						}
						if (cur.ScoreSentence == "") != (cur.Score == nil) {
							t.Errorf("%s: ScoreSentence %q and Score %v disagree", label, cur.ScoreSentence, cur.Score)
						}
					}
				}
			}
		}
	})
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

// TestTrustCopyReporterIsPlainText is the V-32 extension for the reporter
// copy (D-15, D-19, A50, T-03-06, T-03-07). It covers the exact copy, the
// builder grid, the composition grid and the trust object grid.
func TestTrustCopyReporterIsPlainText(t *testing.T) {
	minJudged := strconv.Itoa(ReliabilityMinJudged)
	const (
		confirmedOf = " earlier reports were confirmed by people nearby."
		needs       = " A rating needs 5 reports with a clear result."
	)

	t.Run("exact copy", func(t *testing.T) {
		for tag, want := range map[ReporterTag]string{
			ReporterReliable:     "Reliable reporter",
			ReporterMixed:        "Mixed record",
			ReporterUnreliable:   "Unreliable reporter",
			ReporterNew:          "New reporter",
			ReporterNone:         "",
			ReporterTag("bogus"): "",
		} {
			if got := reporterLabel(tag); got != want {
				t.Errorf("reporterLabel(%q) = %q, want %q", tag, got, want)
			}
		}
		cases := []struct {
			tag  ReporterTag
			j, c int
			want string
		}{
			{ReporterReliable, 5, 5, "5 of 5" + confirmedOf},
			{ReporterMixed, 5, 4, "4 of 5" + confirmedOf},
			{ReporterUnreliable, 5, 0, "0 of 5" + confirmedOf},
			{ReporterNew, 0, 0, "None of their reports has had a clear result yet." + needs},
			{ReporterNew, 1, 1, "Only 1 of their reports has had a clear result so far." + needs},
			{ReporterNew, 2, 0, "Only 2 of their reports have had a clear result so far." + needs},
			{ReporterNew, 4, 3, "Only 4 of their reports have had a clear result so far." + needs},
			{ReporterNone, 5, 5, ""},
			{ReporterTag("bogus"), 5, 5, ""},
		}
		for _, c := range cases {
			if got := reporterSentence(c.tag, c.j, c.c); got != c.want {
				t.Errorf("reporterSentence(%q, %d, %d) = %q, want %q", c.tag, c.j, c.c, got, c.want)
			}
			if c.tag == ReporterNew && !strings.Contains(c.want, "A rating needs "+minJudged+" reports with a clear result.") {
				t.Errorf("the New reporter minimum in %q is not %s", c.want, minJudged)
			}
		}
		if got, want := reporterScoreSentence(86), "Score 86 out of 100, based on how often their reports with a clear result were confirmed."; got != want {
			t.Errorf("reporterScoreSentence(86) = %q, want %q", got, want)
		}
	})

	t.Run("builder grid", func(t *testing.T) {
		check := func(s string) {
			t.Helper()
			assertPlainCopy(t, s)
			for _, banned := range []string{"first report", "/100", "fades"} {
				if strings.Contains(s, banned) {
					t.Errorf("copy %q holds %q", s, banned)
				}
			}
		}
		tags := append(append([]ReporterTag(nil), ReporterTags...), ReporterTag("bogus"))
		for _, tag := range tags {
			check(reporterLabel(tag))
			for _, j := range []int{0, 1, 2, 4, 5, 30, 99, 100, 12345} {
				for _, c := range []int{0, 1, j} {
					if c > j {
						continue
					}
					check(reporterSentence(tag, j, c))
				}
			}
		}
		for _, score := range []int{0, 1, 14, 25, 27, 32, 34, 35, 71, 74, 75, 86, 97, 99, 100} {
			check(reporterScoreSentence(score))
		}
	})

	t.Run("composition grid", func(t *testing.T) {
		check := func(label string, signals map[int64]ReporterSignal) {
			t.Helper()
			for id, s := range signals {
				assertPlainCopy(t, s.Label)
				assertPlainCopy(t, s.Sentence)
				assertPlainCopy(t, s.ScoreSentence)
				if s.Tag == ReporterNone || s.Label == "" {
					t.Errorf("%s account %d: tag %q label %q, a composed signal is never none", label, id, s.Tag, s.Label)
				}
				if (s.ScoreSentence == "") != (s.Score == nil) {
					t.Errorf("%s account %d: score sentence %q with score %v", label, id, s.ScoreSentence, p10Deref(s.Score))
				}
				if (s.Score == nil) != (s.Tag == ReporterNew) {
					t.Errorf("%s account %d: score %v with tag %q", label, id, p10Deref(s.Score), s.Tag)
				}
			}
		}
		ids, h, v := p10GroupFixture()
		check("fixture", ReporterSignals(ids, h, v, p10Now))
		rng := rand.New(rand.NewPCG(10, 3))
		for i := 0; i < 300; i++ {
			ids, h, v := p10RandomHistory(rng)
			check(fmt.Sprintf("random %d", i), ReporterSignals(ids, h, v, p10Now))
		}
	})

	t.Run("trust object grid", func(t *testing.T) {
		ids, h, v := p10GroupFixture()
		signals := ReporterSignals(ids, h, v, p10Now)
		stray := ReporterSignal{Tag: ReporterNone, Label: "stray", Sentence: "stray"}
		inputs := map[string]*ReporterSignal{"nil": nil, "stray none": &stray}
		for _, id := range []int64{101, 102, 103, 104, 105, 106} {
			s := signals[id]
			inputs[fmt.Sprintf("account %d", id)] = &s
		}
		for _, vis := range Visibilities {
			for name, in := range inputs {
				got := BuildTrust(p10TrustInput(vis, in)).Reporter
				assertPlainCopy(t, got.Label)
				assertPlainCopy(t, got.Sentence)
				assertPlainCopy(t, got.ScoreSentence)
				if (got.Label == "") != (got.Tag == ReporterNone) {
					t.Errorf("%s/%s: label %q with tag %q", vis, name, got.Label, got.Tag)
				}
				if (got.ScoreSentence == "") != (got.Score == nil) {
					t.Errorf("%s/%s: score sentence %q with score %v", vis, name, got.ScoreSentence, p10Deref(got.Score))
				}
			}
		}
	})
}
