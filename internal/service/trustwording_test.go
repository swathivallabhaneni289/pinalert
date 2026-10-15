// trustwording_test.go pins the plain people wording of the panel (A88,
// plan 03-30): people confirm and never places, the caveat is gone, every
// sentence that is filled stays filled, and singular and plural read right
// wherever a number can be 1. It is package service because it reaches the
// sentence builders and the grid helpers of the other trust tests. Every
// helper starts with p30 so it cannot collide with a name from another plan.
// No database is needed.

package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// p30Banned are the words no panel sentence may hold, compared in lower
// case. "place" covers place and places (A88). "counts" and "vote" cover the
// retired caveat Counts places, not votes.
var p30Banned = []string{"place", "counts", "vote"}

// p30Check fails when s names a place, carries the caveat or leaves the
// closed character set.
func p30Check(t *testing.T, label, s string) {
	t.Helper()
	assertPlainCopy(t, s)
	lower := strings.ToLower(s)
	for _, b := range p30Banned {
		if strings.Contains(lower, b) {
			t.Errorf("%s: sentence %q holds %q", label, s, b)
		}
	}
}

// p30Sentences is every panel string of one trust object, by field name.
func p30Sentences(tr Trust) map[string]string {
	return map[string]string{
		"lead":           tr.LeadSentence,
		"confirmed":      tr.ConfirmedSentence,
		"disputed":       tr.DisputedSentence,
		"currency label": tr.Currency.Label,
		"currency":       tr.Currency.Sentence,
		"currency score": tr.Currency.ScoreSentence,
		"reporter label": tr.Reporter.Label,
		"reporter":       tr.Reporter.Sentence,
		"reporter score": tr.Reporter.ScoreSentence,
	}
}

// p30Reporters returns the reporter signals of the shared fixture by a short
// name, plus none (nil). The fixture holds every rated tag and two New
// reporters (plan 03-10).
func p30Reporters() map[string]*ReporterSignal {
	ids, h, v := p10GroupFixture()
	signals := ReporterSignals(ids, h, v, p10Now)
	out := map[string]*ReporterSignal{"none": nil}
	for _, id := range ids {
		s := signals[id]
		out[fmt.Sprintf("%s %d", s.Tag, id)] = &s
	}
	return out
}

// p30Case is one state of the grid with the trust object built for it and
// the facts the fill rules need.
type p30Case struct {
	label    string
	in       TrustInput
	trust    Trust
	places   int
	critical bool
}

// p30Grid builds the trust object for every visibility, report kind, vote
// pattern, clock and reporter signal, so every panel state is visited. The
// states include combinations the resolver never produces (Provisional with
// two or more confirming places), because the copy must stay plain there too.
func p30Grid() []p30Case {
	var out []p30Case
	metas := map[string]ReportMeta{"low": p08Low, "medium": p08Medium, "critical": p08Critical, "rescue low": p08RescueLow}
	reporters := p30Reporters()
	for _, vis := range Visibilities {
		// A withheld report never reaches BuildTrust (R-01).
		if vis == VisibilityWithheld {
			continue
		}
		for mname, meta := range metas {
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
				"none":                 nil,
				"one confirm":          {confirm(101, "g1", at(0))},
				"one confirm one disp": {confirm(101, "g1", at(0)), dispute(201, "g4", at(0))},
				"two fresh":            twoAt(0),
				"two 1.5 windows old":  twoAt(window * 3 / 2),
				"two 3 windows old":    twoAt(3 * window),
				"three and a dispute": {
					confirm(101, "g1", at(0)), confirm(102, "g2", at(time.Minute)),
					confirm(103, "g3", at(2*time.Minute)), dispute(201, "g4", at(time.Minute)),
				},
				"two and a dispute": append(twoAt(0), dispute(201, "g4", at(time.Minute))),
				"two disputes":      {dispute(201, "g4", at(0)), dispute(202, "g5", at(0))},
				"150 and 120":       append(p05DistinctRows(150, VoteConfirm, 1000), p05DistinctRows(120, VoteDispute, 3000)...),
			}
			for rname, rows := range rowSets {
				cells := map[string]bool{}
				for _, r := range rows {
					if r.Kind == string(VoteKindContent) && r.Value == string(VoteConfirm) {
						cells[r.GeohashCell] = true
					}
				}
				for nname, now := range map[string]time.Time{"in life": inLife, "at expiry": expires, "past expiry": expires.Add(time.Hour)} {
					for repName, rep := range reporters {
						in := TrustInput{
							Rows: rows, ReportID: 1, Meta: meta, Visibility: vis,
							CreatedAt: p08Created, ExpiresAt: expires, Now: now, Reporter: rep,
						}
						out = append(out, p30Case{
							label:    strings.Join([]string{string(vis), mname, rname, nname, repName}, "/"),
							in:       in,
							trust:    BuildTrust(in),
							places:   len(cells),
							critical: meta.criticalBypass(),
						})
					}
				}
			}
		}
	}
	return out
}

// TestPanelWordingNamesNoPlace is the A88 rule: no panel sentence names a
// place or carries the old caveat, in any state, and every one stays in the
// closed character set. It walks the whole state grid and then each builder
// over a wide range of numbers.
func TestPanelWordingNamesNoPlace(t *testing.T) {
	t.Run("state grid", func(t *testing.T) {
		cases := p30Grid()
		if len(cases) < 1000 {
			t.Fatalf("the grid has %d states, want at least 1000", len(cases))
		}
		for _, c := range cases {
			for field, s := range p30Sentences(c.trust) {
				p30Check(t, c.label+" "+field, s)
			}
		}
	})

	t.Run("builders over a wide range", func(t *testing.T) {
		windows := []string{"1 minute", "45 minutes", "1 hour", "2 hours", "6 hours", "24 hours"}
		// The counter builders take any count, so they also see a negative one.
		for n := -1; n <= 130; n++ {
			p30Check(t, fmt.Sprintf("confirmed %d", n), confirmedSentence(n))
			p30Check(t, fmt.Sprintf("disputed %d", n), disputedSentence(n))
		}
		for n := 0; n <= 130; n++ {
			for _, w := range windows {
				p30Check(t, fmt.Sprintf("up to date %d %s", n, w), upToDateSentence(n, w))
				p30Check(t, fmt.Sprintf("raised bar %d %s", n, w), disputeRaisedBarSentence(n, w))
			}
		}
		for _, w := range windows {
			p30Check(t, "getting old "+w, gettingOldSentence(w))
		}
		p30Check(t, "gone quiet", needsReconfirmingSentence)
		p30Check(t, "too early", tooEarlySentence())
		p30Check(t, "threshold lead", provisionalLeadSentence())
		p30Check(t, "expired lead", expiredLeadSentence)
		for s := 0; s <= 100; s++ {
			p30Check(t, fmt.Sprintf("currency score %d", s), currencyScoreSentence(s))
			p30Check(t, fmt.Sprintf("reporter score %d", s), reporterScoreSentence(s))
		}
		tags := append(append([]ReporterTag(nil), ReporterTags...), ReporterTag("bogus"))
		for _, tag := range tags {
			p30Check(t, "reporter label "+string(tag), reporterLabel(tag))
			for j := 0; j <= 40; j++ {
				for c := 0; c <= j; c++ {
					p30Check(t, fmt.Sprintf("reporter %s %d of %d", tag, c, j), reporterSentence(tag, j, c))
				}
			}
		}
		for _, b := range CurrencyBands {
			p30Check(t, "currency label "+string(b), currencyLabel(b))
		}
		p30Check(t, "too early label", currencyLabelTooEarly)
	})
}

// TestPanelWordingStaysFilled is the A88 guard that the words only change:
// every sentence that was filled before plan 03-30 is still filled, and every
// one that was empty is still empty. The rules below are the documented ones
// (docs/TRUST-MODEL.md, the trust object table), written here from the inputs
// and not read back from the code under test, so the test passes on the base
// commit and after the wording change alike.
func TestPanelWordingStaysFilled(t *testing.T) {
	for _, c := range p30Grid() {
		tr, in := c.trust, c.in
		expired := !in.ExpiresAt.After(in.Now)

		if tr.ConfirmedSentence == "" || tr.DisputedSentence == "" {
			t.Errorf("%s: a counter sentence is empty: confirmed %q, disputed %q", c.label, tr.ConfirmedSentence, tr.DisputedSentence)
		}

		wantLead := in.Visibility != VisibilityRetracted && (expired || in.Visibility == VisibilityProvisional)
		if (tr.LeadSentence != "") != wantLead {
			t.Errorf("%s: lead %q, want it filled %v", c.label, tr.LeadSentence, wantLead)
		}

		shown := in.Visibility == VisibilityLive && !expired
		wantLabel := shown && (c.places >= IndependentAgreementThreshold || c.critical)
		if (tr.Currency.Label != "") != wantLabel {
			t.Errorf("%s: currency label %q, want it filled %v", c.label, tr.Currency.Label, wantLabel)
		}
		if (tr.Currency.Sentence != "") != (tr.Currency.Label != "") {
			t.Errorf("%s: currency label %q and sentence %q disagree", c.label, tr.Currency.Label, tr.Currency.Sentence)
		}
		scored := (in.Visibility == VisibilityLive || in.Visibility == VisibilityProvisional) && !expired &&
			c.places >= IndependentAgreementThreshold
		if (tr.Currency.Score != nil) != scored {
			t.Errorf("%s: currency score %v, want a score %v", c.label, tr.Currency.Score, scored)
		}
		if (tr.Currency.ScoreSentence != "") != (tr.Currency.Score != nil) {
			t.Errorf("%s: currency score sentence %q with score %v", c.label, tr.Currency.ScoreSentence, tr.Currency.Score)
		}

		rated := tr.Reporter.Tag != ReporterNone
		if (tr.Reporter.Label != "") != rated || (tr.Reporter.Sentence != "") != rated {
			t.Errorf("%s: reporter tag %q with label %q and sentence %q", c.label, tr.Reporter.Tag, tr.Reporter.Label, tr.Reporter.Sentence)
		}
		if (tr.Reporter.ScoreSentence != "") != (tr.Reporter.Score != nil) {
			t.Errorf("%s: reporter score sentence %q with score %v", c.label, tr.Reporter.ScoreSentence, tr.Reporter.Score)
		}
	}

	t.Run("builders", func(t *testing.T) {
		for n := -1; n <= 130; n++ {
			if confirmedSentence(n) == "" || disputedSentence(n) == "" {
				t.Errorf("a counter sentence is empty for %d", n)
			}
		}
		for _, tag := range []ReporterTag{ReporterReliable, ReporterMixed, ReporterUnreliable, ReporterNew} {
			for _, j := range []int{0, 1, 2, 5, 30} {
				if reporterSentence(tag, j, 0) == "" {
					t.Errorf("the %s sentence is empty for %d judged", tag, j)
				}
			}
		}
		for _, tag := range []ReporterTag{ReporterNone, ReporterTag("bogus")} {
			if got := reporterSentence(tag, 5, 5); got != "" {
				t.Errorf("the %s sentence is %q, want empty", tag, got)
			}
		}
	})
}

// TestPanelWordingEveryState pins the whole panel copy of every state the
// owner can see, as one table: the lead, the two counter sentences and the
// still current line, with the popup score line (A88).
func TestPanelWordingEveryState(t *testing.T) {
	expiresLow := p08Created.Add(ExpiryDuration(SeverityLow))
	expiresCritical := p08Created.Add(ExpiryDuration(SeverityCritical))
	confirm := func(account int64, cell string, at time.Time) sqlcgen.CurrentVotesForReportsRow {
		return p05VoteAt(1, account, VoteKindContent, VoteConfirm, cell, at)
	}
	dispute := func(account int64, cell string, at time.Time) sqlcgen.CurrentVotesForReportsRow {
		return p05VoteAt(1, account, VoteKindContent, VoteDispute, cell, at)
	}
	h := func(d time.Duration) time.Time { return p08Created.Add(d) }

	type want struct{ lead, confirmed, disputed, label, currency, score string }
	states := []struct {
		name    string
		vis     Visibility
		meta    ReportMeta
		rows    []sqlcgen.CurrentVotesForReportsRow
		expires time.Time
		now     time.Time
		want    want
	}{
		{
			name: "provisional, nobody has confirmed", vis: VisibilityProvisional, meta: p08Low,
			expires: expiresLow, now: h(30 * time.Minute),
			want: want{
				lead: "Needs 2 people at different spots to confirm it.", confirmed: "No one has confirmed this yet.",
				disputed: "No one has disputed this.",
			},
		},
		{
			name: "provisional, one person confirmed", vis: VisibilityProvisional, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(10*time.Minute))},
			expires: expiresLow, now: h(30 * time.Minute),
			want: want{
				lead: "Needs 1 more person at a different spot to confirm it.", confirmed: "1 person confirmed this.",
				disputed: "No one has disputed this.",
			},
		},
		{
			name: "provisional, one confirmed and one disputed", vis: VisibilityProvisional, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(10*time.Minute)), dispute(201, "g2", h(11*time.Minute))},
			expires: expiresLow, now: h(30 * time.Minute),
			want: want{
				lead: "Needs 1 more person at a different spot to confirm it.", confirmed: "1 person confirmed this.",
				disputed: "1 person nearby disputed this.",
			},
		},
		{
			name: "live, report up to date", vis: VisibilityLive, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour)), confirm(102, "g2", h(time.Hour))},
			expires: expiresLow, now: h(time.Hour),
			want: want{
				confirmed: "Confirmed by 2 people at different spots.", disputed: "No one has disputed this.",
				label: "Report up to date", currency: "2 confirmations in the last 2 hours.", score: "Score 100 out of 100.",
			},
		},
		{
			name: "live, report getting old", vis: VisibilityLive, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(0)), confirm(102, "g2", h(0))},
			expires: expiresLow, now: h(2*time.Hour + 10*time.Minute),
			want: want{
				confirmed: "Confirmed by 2 people at different spots.", disputed: "No one has disputed this.",
				label: "Report getting old", currency: "Fewer than 2 confirmations in the last 2 hours.", score: "Score 72 out of 100.",
			},
		},
		{
			name: "live, report needs re-confirming", vis: VisibilityLive, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(0)), confirm(102, "g2", h(0))},
			expires: expiresLow, now: h(4*time.Hour + 30*time.Minute),
			want: want{
				confirmed: "Confirmed by 2 people at different spots.", disputed: "No one has disputed this.",
				label: "Report needs re-confirming", currency: "Gone quiet. People nearby can confirm it again.", score: "Score 43 out of 100.",
			},
		},
		{
			name: "live, a dispute raised the bar", vis: VisibilityLive, meta: p08Low,
			rows: []sqlcgen.CurrentVotesForReportsRow{
				confirm(101, "g1", h(0)), confirm(102, "g2", h(0)), dispute(201, "g3", h(time.Hour)),
			},
			expires: expiresLow, now: h(time.Hour),
			want: want{
				confirmed: "Confirmed by 2 people at different spots.", disputed: "1 person nearby disputed this.",
				label: "Report needs re-confirming", currency: "Disputed nearby, so it needs 3 confirmations in the last 2 hours.",
				score: "Score 0 out of 100.",
			},
		},
		{
			name: "live critical, one person confirmed, too early to tell", vis: VisibilityLive, meta: p08Critical,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour))},
			expires: expiresCritical, now: h(time.Hour),
			want: want{
				confirmed: "1 person confirmed this.", disputed: "No one has disputed this.",
				label: "Too early to tell", currency: "Fewer than 2 people have confirmed this yet.",
			},
		},
		{
			name: "live rescue needed, nobody confirmed, too early to tell", vis: VisibilityLive, meta: p08RescueLow,
			expires: expiresLow, now: h(time.Hour),
			want: want{
				confirmed: "No one has confirmed this yet.", disputed: "No one has disputed this.",
				label: "Too early to tell", currency: "Fewer than 2 people have confirmed this yet.",
			},
		},
		{
			name: "hidden by disputes", vis: VisibilityHidden, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{dispute(201, "g3", h(time.Hour)), dispute(202, "g4", h(time.Hour))},
			expires: expiresLow, now: h(time.Hour),
			want: want{confirmed: "No one has confirmed this yet.", disputed: "2 people at different spots disputed this."},
		},
		{
			name: "retracted", vis: VisibilityRetracted, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour)), confirm(102, "g2", h(time.Hour))},
			expires: expiresLow, now: h(time.Hour),
			want: want{confirmed: "Confirmed by 2 people at different spots.", disputed: "No one has disputed this."},
		},
		{
			name: "live and expired", vis: VisibilityLive, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour)), confirm(102, "g2", h(time.Hour))},
			expires: expiresLow, now: expiresLow,
			want: want{
				lead: "Expired, so it can no longer be confirmed.", confirmed: "Confirmed by 2 people at different spots.",
				disputed: "No one has disputed this.",
			},
		},
		{
			name: "provisional and expired with one person confirmed", vis: VisibilityProvisional, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour))},
			expires: expiresLow, now: expiresLow.Add(time.Minute),
			want: want{
				lead: "Expired, so it can no longer be confirmed.", confirmed: "1 person confirmed this.",
				disputed: "No one has disputed this.",
			},
		},
	}
	for _, s := range states {
		t.Run(s.name, func(t *testing.T) {
			tr := BuildTrust(TrustInput{
				Rows: s.rows, ReportID: 1, Meta: s.meta, Visibility: s.vis,
				CreatedAt: p08Created, ExpiresAt: s.expires, Now: s.now,
			})
			got := want{
				lead: tr.LeadSentence, confirmed: tr.ConfirmedSentence, disputed: tr.DisputedSentence,
				label: tr.Currency.Label, currency: tr.Currency.Sentence, score: tr.Currency.ScoreSentence,
			}
			if got != s.want {
				t.Errorf("panel copy\n got  %+v\n want %+v", got, s.want)
			}
			for field, text := range p30Sentences(tr) {
				p30Check(t, s.name+" "+field, text)
			}
		})
	}

	t.Run("reporter tags", func(t *testing.T) {
		const needs = "Not rated yet. It needs 5 reports with a clear result."
		type rep struct{ label, sentence, score string }
		ids, hist, votes := p10GroupFixture()
		signals := ReporterSignals(ids, hist, votes, p10Now)
		for id, w := range map[int64]rep{
			101: {"Reliable reporter", "5 of their last 5 reports were confirmed.", "Score 86 out of 100."},
			102: {"Mixed record", "4 of their last 5 reports were confirmed.", "Score 71 out of 100."},
			103: {"Unreliable reporter", "0 of their last 5 reports were confirmed.", "Score 32 out of 100."},
			105: {"New reporter", needs, ""},
			106: {"New reporter", needs, ""},
		} {
			s := signals[id]
			tr := BuildTrust(p10TrustInput(VisibilityLive, &s))
			got := rep{tr.Reporter.Label, tr.Reporter.Sentence, tr.Reporter.ScoreSentence}
			if got != w {
				t.Errorf("account %d: reporter copy %+v, want %+v", id, got, w)
			}
		}
		none := BuildTrust(p10TrustInput(VisibilityLive, nil)).Reporter
		if none.Tag != ReporterNone || none.Label != "" || none.Sentence != "" || none.ScoreSentence != "" {
			t.Errorf("no reporter signal: %+v, want the none signal with no text", none)
		}
	})
}

// TestPanelWordingSingularAndPlural pins the singular and plural forms
// wherever a number can be 1 (A88).
func TestPanelWordingSingularAndPlural(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"confirmed 0", confirmedSentence(0), "No one has confirmed this yet."},
		{"confirmed 1", confirmedSentence(1), "1 person confirmed this."},
		{"confirmed 2", confirmedSentence(2), "Confirmed by 2 people at different spots."},
		{"disputed 0", disputedSentence(0), "No one has disputed this."},
		{"disputed 1", disputedSentence(1), "1 person nearby disputed this."},
		{"disputed 2", disputedSentence(2), "2 people at different spots disputed this."},
		{"up to date 1", upToDateSentence(1, "1 hour"), "1 confirmation in the last 1 hour."},
		{"up to date 2", upToDateSentence(2, "1 hour"), "2 confirmations in the last 1 hour."},
		{"raised bar 1", disputeRaisedBarSentence(1, "2 hours"), "Disputed nearby, so it needs 1 confirmation in the last 2 hours."},
		{"raised bar 3", disputeRaisedBarSentence(3, "2 hours"), "Disputed nearby, so it needs 3 confirmations in the last 2 hours."},
		{"rated, 0 of 5", reporterSentence(ReporterUnreliable, 5, 0), "0 of their last 5 reports were confirmed."},
		{"rated, 1 of 5", reporterSentence(ReporterUnreliable, 5, 1), "1 of their last 5 reports was confirmed."},
		{"rated, 2 of 5", reporterSentence(ReporterMixed, 5, 2), "2 of their last 5 reports were confirmed."},
		{"rated, 1 of 1", reporterSentence(ReporterMixed, 1, 1), "1 of their last 1 report was confirmed."},
		{"new reporter, none judged", reporterSentence(ReporterNew, 0, 0), "Not rated yet. It needs 5 reports with a clear result."},
		{"new reporter, 1 judged", reporterSentence(ReporterNew, 1, 1), "Not rated yet. It needs 5 reports with a clear result."},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestPanelWordingPluralConstants guards the one assumption behind the fixed
// plural words in the threshold sentences: both constants are 2 or more. A
// threshold of 1 would not be independent agreement, and a judged minimum of 1
// would need Needs 1 report in the New reporter sentence.
func TestPanelWordingPluralConstants(t *testing.T) {
	if IndependentAgreementThreshold < 2 {
		t.Errorf("IndependentAgreementThreshold = %d, the threshold sentences assume 2 or more", IndependentAgreementThreshold)
	}
	if ReliabilityMinJudged < 2 {
		t.Errorf("ReliabilityMinJudged = %d, the New reporter sentence assumes 2 or more", ReliabilityMinJudged)
	}
}
