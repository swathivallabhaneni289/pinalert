// trustfacts_test.go pins the facts list of the trust object (A89, A90, plan
// 03-31): the ordered rows the tick mark card shows, their tones, their
// strings and the rules that decide which rows exist. It is package service
// because it reuses the state grid and the fixtures of the other trust tests.
// Every helper starts with p31 so it cannot collide with another plan's name.
// No database is needed.

package service

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// p31Fact builds an expected fact in one line.
func p31Fact(kind FactKind, tone FactTone, title, detail, score string) TrustFact {
	return TrustFact{Kind: kind, Tone: tone, Title: title, Detail: detail, ScoreDetail: score}
}

// p31Check holds one fact to the closed character set, the no place rule and
// the shape rules: a closed kind and tone, and a title that is never empty.
func p31Check(t *testing.T, label string, f TrustFact) {
	t.Helper()
	if !slices.Contains(FactKinds, f.Kind) {
		t.Errorf("%s: kind %q is not a closed kind", label, f.Kind)
	}
	if !slices.Contains(FactTones, f.Tone) {
		t.Errorf("%s: tone %q is not a closed tone", label, f.Tone)
	}
	if f.Title == "" {
		t.Errorf("%s: %s fact has an empty title", label, f.Kind)
	}
	for field, s := range map[string]string{"title": f.Title, "detail": f.Detail, "score detail": f.ScoreDetail} {
		p30Check(t, label+" "+string(f.Kind)+" "+field, s)
	}
}

// TestTrustFactsEveryState pins the whole facts list of every state the owner
// can see, as one table.
func TestTrustFactsEveryState(t *testing.T) {
	expiresLow := p08Created.Add(ExpiryDuration(SeverityLow))
	expiresCritical := p08Created.Add(ExpiryDuration(SeverityCritical))
	confirm := func(account int64, cell string, at time.Time) sqlcgen.CurrentVotesForReportsRow {
		return p05VoteAt(1, account, VoteKindContent, VoteConfirm, cell, at)
	}
	dispute := func(account int64, cell string, at time.Time) sqlcgen.CurrentVotesForReportsRow {
		return p05VoteAt(1, account, VoteKindContent, VoteDispute, cell, at)
	}
	h := func(d time.Duration) time.Time { return p08Created.Add(d) }
	rows2 := func(at time.Time) []sqlcgen.CurrentVotesForReportsRow {
		return []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", at), confirm(102, "g2", at)}
	}

	noDisputes := p31Fact(FactDisputes, ToneGood, "No disputes", "No one nearby has disputed this.", "")
	twoConfirmed := p31Fact(FactConfirmations, ToneGood, "Confirmed by 2 people", "At different spots nearby.", "")
	oneDispute := p31Fact(FactDisputes, ToneBad, "Disputed by 1 person", "Someone nearby says this is wrong.", "")
	expired := p31Fact(FactStatus, ToneNeutral, "Expired", "It can no longer be confirmed.", "")

	states := []struct {
		name    string
		vis     Visibility
		meta    ReportMeta
		rows    []sqlcgen.CurrentVotesForReportsRow
		expires time.Time
		now     time.Time
		want    []TrustFact
	}{
		{
			name: "provisional, nobody has confirmed", vis: VisibilityProvisional, meta: p08Low,
			expires: expiresLow, now: h(30 * time.Minute),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneNeutral, "Not confirmed yet", "Needs 2 people at different spots.", ""),
				noDisputes,
			},
		},
		{
			name: "provisional, one person confirmed", vis: VisibilityProvisional, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(10*time.Minute))},
			expires: expiresLow, now: h(30 * time.Minute),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneNeutral, "Confirmed by 1 person", "Needs 1 more person at a different spot.", ""),
				noDisputes,
			},
		},
		{
			name: "provisional, one confirmed and one disputed", vis: VisibilityProvisional, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(10*time.Minute)), dispute(201, "g2", h(11*time.Minute))},
			expires: expiresLow, now: h(30 * time.Minute),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneNeutral, "Confirmed by 1 person", "Needs 1 more person at a different spot.", ""),
				oneDispute,
			},
		},
		{
			name: "live, report up to date", vis: VisibilityLive, meta: p08Low,
			rows: rows2(h(time.Hour)), expires: expiresLow, now: h(time.Hour),
			want: []TrustFact{
				twoConfirmed, noDisputes,
				p31Fact(FactCurrency, ToneGood, "Still happening", "2 confirmations in the last 2 hours.", "Score 100 out of 100."),
			},
		},
		{
			name: "live, report getting old", vis: VisibilityLive, meta: p08Low,
			rows: rows2(h(0)), expires: expiresLow, now: h(2*time.Hour + 10*time.Minute),
			want: []TrustFact{
				twoConfirmed, noDisputes,
				p31Fact(FactCurrency, ToneCaution, "Getting old", "Fewer than 2 confirmations in the last 2 hours.", "Score 72 out of 100."),
			},
		},
		{
			name: "live, report needs re-confirming", vis: VisibilityLive, meta: p08Low,
			rows: rows2(h(0)), expires: expiresLow, now: h(4*time.Hour + 30*time.Minute),
			want: []TrustFact{
				twoConfirmed, noDisputes,
				p31Fact(FactCurrency, ToneBad, "Gone quiet", "People nearby can confirm it again.", "Score 43 out of 100."),
			},
		},
		{
			name: "live, a dispute raised the bar", vis: VisibilityLive, meta: p08Low,
			rows:    append(rows2(h(0)), dispute(201, "g3", h(time.Hour))),
			expires: expiresLow, now: h(time.Hour),
			want: []TrustFact{
				twoConfirmed, oneDispute,
				p31Fact(FactCurrency, ToneCaution, "Needs more confirmations",
					"Disputed nearby, so it needs 3 confirmations in the last 2 hours.", "Score 0 out of 100."),
			},
		},
		{
			name: "live critical, one person confirmed, too early to tell", vis: VisibilityLive, meta: p08Critical,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour))},
			expires: expiresCritical, now: h(time.Hour),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneNeutral, "Confirmed by 1 person", "Needs 1 more person at a different spot.", ""),
				noDisputes,
				p31Fact(FactCurrency, ToneNeutral, "Too early to tell", "Fewer than 2 people have confirmed this yet.", ""),
			},
		},
		{
			name: "live rescue needed, nobody confirmed, too early to tell", vis: VisibilityLive, meta: p08RescueLow,
			expires: expiresLow, now: h(time.Hour),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneNeutral, "Not confirmed yet", "Needs 2 people at different spots.", ""),
				noDisputes,
				p31Fact(FactCurrency, ToneNeutral, "Too early to tell", "Fewer than 2 people have confirmed this yet.", ""),
			},
		},
		{
			name: "hidden by disputes, nobody confirmed", vis: VisibilityHidden, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{dispute(201, "g3", h(time.Hour)), dispute(202, "g4", h(time.Hour))},
			expires: expiresLow, now: h(time.Hour),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneNeutral, "Not confirmed yet", "", ""),
				p31Fact(FactDisputes, ToneBad, "Disputed by 2 people", "At different spots nearby.", ""),
			},
		},
		{
			name: "hidden, one person confirmed", vis: VisibilityHidden, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour)), dispute(201, "g3", h(time.Hour)), dispute(202, "g4", h(time.Hour))},
			expires: expiresLow, now: h(time.Hour),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneNeutral, "Confirmed by 1 person", "", ""),
				p31Fact(FactDisputes, ToneBad, "Disputed by 2 people", "At different spots nearby.", ""),
			},
		},
		{
			name: "retracted", vis: VisibilityRetracted, meta: p08Low,
			rows: rows2(h(time.Hour)), expires: expiresLow, now: h(time.Hour),
			want: []TrustFact{twoConfirmed, noDisputes},
		},
		{
			name: "retracted and expired reads as resolved, not expired", vis: VisibilityRetracted, meta: p08Low,
			rows: rows2(h(time.Hour)), expires: expiresLow, now: expiresLow,
			want: []TrustFact{twoConfirmed, noDisputes},
		},
		{
			name: "live and expired", vis: VisibilityLive, meta: p08Low,
			rows: rows2(h(time.Hour)), expires: expiresLow, now: expiresLow,
			want: []TrustFact{expired, twoConfirmed, noDisputes},
		},
		{
			name: "provisional and expired with one person confirmed", vis: VisibilityProvisional, meta: p08Low,
			rows:    []sqlcgen.CurrentVotesForReportsRow{confirm(101, "g1", h(time.Hour))},
			expires: expiresLow, now: expiresLow.Add(time.Minute),
			want: []TrustFact{
				expired,
				p31Fact(FactConfirmations, ToneNeutral, "Confirmed by 1 person", "", ""),
				noDisputes,
			},
		},
		{
			name: "provisional and expired, nobody confirmed", vis: VisibilityProvisional, meta: p08Low,
			expires: expiresLow, now: expiresLow,
			want: []TrustFact{
				expired,
				p31Fact(FactConfirmations, ToneNeutral, "Not confirmed yet", "", ""),
				noDisputes,
			},
		},
		{
			name: "a crowd keeps its real numbers above the cap", vis: VisibilityLive, meta: p08Low,
			rows:    append(p05DistinctRows(150, VoteConfirm, 1000), p05DistinctRows(120, VoteDispute, 3000)...),
			expires: expiresLow, now: h(time.Hour),
			want: []TrustFact{
				p31Fact(FactConfirmations, ToneGood, "Confirmed by 150 people", "At different spots nearby.", ""),
				p31Fact(FactDisputes, ToneBad, "Disputed by 120 people", "At different spots nearby.", ""),
			},
		},
	}
	for _, s := range states {
		t.Run(s.name, func(t *testing.T) {
			tr := BuildTrust(TrustInput{
				Rows: s.rows, ReportID: 1, Meta: s.meta, Visibility: s.vis,
				CreatedAt: p08Created, ExpiresAt: s.expires, Now: s.now,
			})
			got := tr.Facts
			// A crowd has a currency row whose numbers the table does not pin.
			if s.name == "a crowd keeps its real numbers above the cap" && len(got) == 3 {
				got = got[:2]
			}
			if !reflect.DeepEqual(got, s.want) {
				t.Errorf("facts\n got  %+v\n want %+v", got, s.want)
			}
			for _, f := range tr.Facts {
				p31Check(t, s.name, f)
			}
		})
	}

	t.Run("reporter row, every tag", func(t *testing.T) {
		const needs = "Not rated yet. It needs 5 reports with a clear result."
		ids, hist, votes := p10GroupFixture()
		signals := ReporterSignals(ids, hist, votes, p10Now)
		for id, want := range map[int64]TrustFact{
			101: p31Fact(FactReporter, ToneGood, "Reliable reporter", "5 of their last 5 reports were confirmed.", "Score 86 out of 100."),
			102: p31Fact(FactReporter, ToneCaution, "Mixed record", "4 of their last 5 reports were confirmed.", "Score 71 out of 100."),
			103: p31Fact(FactReporter, ToneBad, "Unreliable reporter", "0 of their last 5 reports were confirmed.", "Score 32 out of 100."),
			105: p31Fact(FactReporter, ToneNeutral, "New reporter", needs, ""),
			106: p31Fact(FactReporter, ToneNeutral, "New reporter", needs, ""),
		} {
			s := signals[id]
			facts := BuildTrust(p10TrustInput(VisibilityLive, &s)).Facts
			if len(facts) == 0 || !reflect.DeepEqual(facts[len(facts)-1], want) {
				t.Errorf("account %d: last fact %+v, want %+v", id, facts, want)
			}
		}
		none := BuildTrust(p10TrustInput(VisibilityLive, nil)).Facts
		for _, f := range none {
			if f.Kind == FactReporter {
				t.Errorf("no reporter signal still has a reporter fact: %+v", f)
			}
		}
	})
}

// TestTrustFactsRulesOverTheGrid walks every state of the grid and holds the
// facts to the documented rules, written from the inputs and from the
// existing signals, not read back from the builder.
func TestTrustFactsRulesOverTheGrid(t *testing.T) {
	cases := p30Grid()
	if len(cases) < 1000 {
		t.Fatalf("the grid has %d states, want at least 1000", len(cases))
	}
	order := map[FactKind]int{FactStatus: 0, FactConfirmations: 1, FactDisputes: 2, FactCurrency: 3, FactReporter: 4}
	currencyTone := map[CurrencyBand]FactTone{
		CurrencyUpToDate: ToneGood, CurrencyGettingOld: ToneCaution,
		CurrencyNeedsReconfirming: ToneBad, CurrencyNotYetCorroborated: ToneNeutral,
	}
	reporterTone := map[ReporterTag]FactTone{
		ReporterReliable: ToneGood, ReporterMixed: ToneCaution, ReporterUnreliable: ToneBad, ReporterNew: ToneNeutral,
	}
	for _, c := range cases {
		tr, in := c.trust, c.in
		byKind := map[FactKind]TrustFact{}
		last := -1
		for _, f := range tr.Facts {
			p31Check(t, c.label, f)
			if _, dup := byKind[f.Kind]; dup {
				t.Errorf("%s: kind %q appears twice", c.label, f.Kind)
			}
			byKind[f.Kind] = f
			if order[f.Kind] <= last {
				t.Errorf("%s: kind %q is out of order in %+v", c.label, f.Kind, tr.Facts)
			}
			last = order[f.Kind]
		}
		expired := !in.ExpiresAt.After(in.Now)

		_, hasStatus := byKind[FactStatus]
		if hasStatus != (expired && in.Visibility != VisibilityRetracted) {
			t.Errorf("%s: status fact %v, want it for an expired report that is not resolved", c.label, hasStatus)
		}
		if hasStatus && tr.Facts[0].Kind != FactStatus {
			t.Errorf("%s: the status fact is not first", c.label)
		}

		conf, okC := byKind[FactConfirmations]
		disp, okD := byKind[FactDisputes]
		if !okC || !okD {
			t.Errorf("%s: confirmations or disputes missing in %+v", c.label, tr.Facts)
			continue
		}
		switch n := tr.ConfirmedPlaces; {
		case n >= IndependentAgreementThreshold:
			if conf.Tone != ToneGood {
				t.Errorf("%s: %d confirming, tone %q, want good", c.label, n, conf.Tone)
			}
		default:
			if conf.Tone != ToneNeutral {
				t.Errorf("%s: %d confirming, tone %q, want neutral", c.label, n, conf.Tone)
			}
		}
		if wantTone := map[bool]FactTone{true: ToneBad, false: ToneGood}[tr.DisputedPlaces > 0]; disp.Tone != wantTone {
			t.Errorf("%s: %d disputing, tone %q, want %q", c.label, tr.DisputedPlaces, disp.Tone, wantTone)
		}
		// Nobody can act on a needs line for a hidden, resolved or expired report.
		quiet := in.Visibility == VisibilityHidden || in.Visibility == VisibilityRetracted || expired
		if quiet && tr.ConfirmedPlaces < IndependentAgreementThreshold && conf.Detail != "" {
			t.Errorf("%s: confirmations detail %q, want it empty for a hidden, resolved or expired report", c.label, conf.Detail)
		}
		if !quiet && tr.ConfirmedPlaces < IndependentAgreementThreshold && conf.Detail == "" {
			t.Errorf("%s: confirmations detail is empty for a report that can still be confirmed", c.label)
		}

		cur, hasCur := byKind[FactCurrency]
		if hasCur != (tr.Currency.Label != "") {
			t.Errorf("%s: currency fact %v with label %q", c.label, hasCur, tr.Currency.Label)
		}
		if hasCur {
			wantDetail := tr.Currency.Sentence
			if tr.Currency.Sentence == needsReconfirmingSentence {
				wantDetail = "People nearby can confirm it again."
			}
			if cur.Detail != wantDetail || cur.ScoreDetail != tr.Currency.ScoreSentence {
				t.Errorf("%s: currency fact %+v does not carry the signal's own sentences %q and %q",
					c.label, cur, tr.Currency.Sentence, tr.Currency.ScoreSentence)
			}
			raised := tr.Currency.Band != CurrencyUpToDate && tr.Currency.Band != CurrencyNotYetCorroborated &&
				tr.Currency.Sentence != gettingOldSentence(windowPhrase(tr.Currency.WindowMinutes)) &&
				tr.Currency.Sentence != needsReconfirmingSentence
			wantTone, wantTitle := currencyTone[tr.Currency.Band], ""
			switch {
			case raised:
				wantTone, wantTitle = ToneCaution, "Needs more confirmations"
			case tr.Currency.Band == CurrencyUpToDate:
				wantTitle = "Still happening"
			case tr.Currency.Band == CurrencyGettingOld:
				wantTitle = "Getting old"
			case tr.Currency.Band == CurrencyNeedsReconfirming:
				wantTitle = "Gone quiet"
			default:
				wantTitle = "Too early to tell"
			}
			if cur.Tone != wantTone || cur.Title != wantTitle {
				t.Errorf("%s: currency fact tone %q title %q, want %q and %q", c.label, cur.Tone, cur.Title, wantTone, wantTitle)
			}
		}

		rep, hasRep := byKind[FactReporter]
		if hasRep != (tr.Reporter.Tag != ReporterNone) {
			t.Errorf("%s: reporter fact %v with tag %q", c.label, hasRep, tr.Reporter.Tag)
		}
		if hasRep {
			if rep.Title != tr.Reporter.Label || rep.Detail != tr.Reporter.Sentence ||
				rep.ScoreDetail != tr.Reporter.ScoreSentence || rep.Tone != reporterTone[tr.Reporter.Tag] {
				t.Errorf("%s: reporter fact %+v does not match the reporter signal %+v", c.label, rep, tr.Reporter)
			}
		}
	}
}

// TestTrustFactsPlurals pins the singular and plural forms of the two
// counter rows over the numbers around 1, the threshold and the cap.
func TestTrustFactsPlurals(t *testing.T) {
	expires := p08Created.Add(ExpiryDuration(SeverityLow))
	now := p08Created.Add(time.Hour)
	for _, n := range []int{0, 1, 2, 3, 99, 100, 150} {
		rows := append(p05DistinctRows(n, VoteConfirm, 1000), p05DistinctRows(n, VoteDispute, 3000)...)
		tr := BuildTrust(TrustInput{
			Rows: rows, ReportID: 1, Meta: p08Low, Visibility: VisibilityLive,
			CreatedAt: p08Created, ExpiresAt: expires, Now: now,
		})
		var wantConf, wantDisp string
		switch {
		case n == 0:
			wantConf, wantDisp = "Not confirmed yet", "No disputes"
		case n == 1:
			wantConf, wantDisp = "Confirmed by 1 person", "Disputed by 1 person"
		default:
			wantConf, wantDisp = fmt.Sprintf("Confirmed by %d people", n), fmt.Sprintf("Disputed by %d people", n)
		}
		titles := map[FactKind]string{}
		for _, f := range tr.Facts {
			titles[f.Kind] = f.Title
		}
		if titles[FactConfirmations] != wantConf || titles[FactDisputes] != wantDisp {
			t.Errorf("n %d: titles %q and %q, want %q and %q", n, titles[FactConfirmations], titles[FactDisputes], wantConf, wantDisp)
		}
	}
}

// TestTrustFactsAreDeterministic holds that the same input gives the same
// facts, that the order of the vote rows does not matter, and that two built
// objects do not share a slice.
func TestTrustFactsAreDeterministic(t *testing.T) {
	for _, c := range p30Grid() {
		again := BuildTrust(c.in)
		if !reflect.DeepEqual(c.trust.Facts, again.Facts) {
			t.Fatalf("%s: two builds differ:\n%+v\n%+v", c.label, c.trust.Facts, again.Facts)
		}
		if len(again.Facts) > 0 {
			again.Facts[0].Title = "changed"
			if c.trust.Facts[0].Title == "changed" {
				t.Fatalf("%s: two built objects share one facts slice", c.label)
			}
		}
		if len(c.in.Rows) > 1 {
			rev := slices.Clone(c.in.Rows)
			slices.Reverse(rev)
			in := c.in
			in.Rows = rev
			if got := BuildTrust(in); !reflect.DeepEqual(c.trust.Facts, got.Facts) {
				t.Fatalf("%s: the vote row order changes the facts", c.label)
			}
		}
	}
}
