// reporter_test.go declares package service (the internal test package)
// because it reaches the unexported copy builders and reuses acctPtr from
// trust_test.go. Every helper here starts with p10 so no name collides with
// the helpers of other plans. The tests turn real history rows and real vote
// rows into reporter signals through the public ReporterSignals function and
// compare them with a reference built the slow way from the 03-03 functions.

package service

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// p10Now is the fixed clock of every reporter test. p10Judged is two hours
// earlier, the expiry every deterministic history row uses, so all weights
// are equal and the scores are exact.
var p10Now = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
var p10Judged = p10Now.Add(-2 * time.Hour)

// p10History builds one history row.
func p10History(account, report int64, severity Severity, category Category, expiresAt time.Time) sqlcgen.ReporterHistoryReportsRow {
	return sqlcgen.ReporterHistoryReportsRow{
		AccountID: account,
		ReportID:  report,
		Severity:  string(severity),
		Category:  string(category),
		ExpiresAt: expiresAt,
	}
}

// p10Vote builds one current vote row, cast three hours before p10Now.
func p10Vote(report, account int64, kind VoteKind, value VoteValue, cell string) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    report,
		AccountID:   account,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
		CreatedAt:   p10Now.Add(-3 * time.Hour),
	}
}

// p10Corroborate returns two content confirms by accounts a and b in two
// different cells, enough for the Live ladder rung.
func p10Corroborate(report, a, b int64) []sqlcgen.CurrentVotesForReportsRow {
	return []sqlcgen.CurrentVotesForReportsRow{
		p10Vote(report, a, VoteKindContent, VoteConfirm, fmt.Sprintf("p10cell%da", report)),
		p10Vote(report, b, VoteKindContent, VoteConfirm, fmt.Sprintf("p10cell%db", report)),
	}
}

// p10Contradict returns two content disputes by accounts a and b in two
// different cells, enough for the Hidden ladder rung.
func p10Contradict(report, a, b int64) []sqlcgen.CurrentVotesForReportsRow {
	return []sqlcgen.CurrentVotesForReportsRow{
		p10Vote(report, a, VoteKindContent, VoteDispute, fmt.Sprintf("p10cell%da", report)),
		p10Vote(report, b, VoteKindContent, VoteDispute, fmt.Sprintf("p10cell%db", report)),
	}
}

// p10GroupFixture is the deterministic six account fixture of
// TestReporterSignalsGroupByAccount. Account 107 has history but is never
// requested, and 101 is requested twice.
func p10GroupFixture() (accountIDs []int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow) {
	accountIDs = []int64{101, 102, 103, 104, 105, 106, 101}
	add := func(account, report int64) {
		history = append(history, p10History(account, report, SeverityLow, CategoryFlood, p10Judged))
	}
	for r := int64(1011); r <= 1015; r++ {
		add(101, r)
		votes = append(votes, p10Corroborate(r, 103, 102)...)
	}
	for r := int64(1021); r <= 1024; r++ {
		add(102, r)
		votes = append(votes, p10Corroborate(r, 101, 103)...)
	}
	add(102, 1025)
	votes = append(votes, p10Contradict(1025, 101, 103)...)
	votes = append(votes, p10Vote(1025, 102, VoteKindResolution, VoteResolve, "p10cell1025own"))
	for r := int64(1031); r <= 1035; r++ {
		add(103, r)
		votes = append(votes,
			p10Vote(r, 103, VoteKindContent, VoteConfirm, fmt.Sprintf("p10own%d", r)),
			p10Vote(r, 101, VoteKindContent, VoteConfirm, fmt.Sprintf("p10other%d", r)),
		)
	}
	for r := int64(1041); r <= 1044; r++ {
		add(104, r)
		votes = append(votes, p10Corroborate(r, 101, 102)...)
	}
	add(104, 1045)
	for r := int64(1051); r <= 1054; r++ {
		add(105, r)
		votes = append(votes, p10Corroborate(r, 101, 102)...)
	}
	for r := int64(1071); r <= 1075; r++ {
		add(107, r)
		votes = append(votes, p10Corroborate(r, 101, 102)...)
	}
	votes = append(votes, p10Vote(9999, 101, VoteKindContent, VoteConfirm, "p10stray"))
	return accountIDs, history, votes
}

// p10RandomHistory builds a random history and vote batch. Votes keep at most
// one row per report, account and kind, the shape CurrentVotesForReports
// returns.
func p10RandomHistory(rng *rand.Rand) (accountIDs []int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow) {
	n := rng.IntN(41)
	for i := 0; i < n; i++ {
		offset := time.Duration(rng.Int64N(int64(101 * 24 * time.Hour)))
		expires := p10Now.Add(-100 * 24 * time.Hour).Add(offset)
		history = append(history, p10History(
			int64(1+rng.IntN(8)),
			int64(i+1),
			Severities[rng.IntN(len(Severities))],
			Categories[rng.IntN(len(Categories))],
			expires,
		))
	}
	type voteKey struct {
		report, account int64
		kind            VoteKind
	}
	seen := map[voteKey]bool{}
	nv := rng.IntN(121)
	for j := 0; j < nv; j++ {
		report := int64(1 + rng.IntN(n+2))
		account := int64(1 + rng.IntN(12))
		kind := VoteKindContent
		value := VoteConfirm
		if rng.IntN(2) == 0 {
			value = VoteDispute
		}
		if rng.IntN(4) == 0 {
			kind = VoteKindResolution
			value = VoteResolve
			if rng.IntN(2) == 0 {
				value = VoteReopen
			}
		}
		k := voteKey{report, account, kind}
		if seen[k] {
			continue
		}
		seen[k] = true
		votes = append(votes, p10Vote(report, account, kind, value, fmt.Sprintf("p10c%d", rng.IntN(6))))
	}
	count := 1 + rng.IntN(8)
	for _, p := range rng.Perm(8)[:count] {
		accountIDs = append(accountIDs, int64(p+1))
	}
	return accountIDs, history, votes
}

// p10ReferenceSignal builds one account's signal the slow way: the whole vote
// slice goes to BuildVoteTally for every history row, with that account as the
// reporter, and the copy comes straight from the builders.
func p10ReferenceSignal(account int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow, now time.Time) ReporterSignal {
	var outcomes []JudgedOutcome
	for _, row := range history {
		if row.AccountID != account {
			continue
		}
		meta := ReportMeta{Severity: Severity(row.Severity), Category: Category(row.Category)}
		if !meta.Category.Valid() {
			meta.Category = CategoryOther
		}
		if !meta.Severity.Valid() {
			meta.Severity = SeverityLow
		}
		who := account
		tally := BuildVoteTally(votes, row.ReportID, &who)
		outcomes = append(outcomes, JudgedOutcome{
			Outcome:   ClassifyOutcome(meta, tally, row.ExpiresAt, now),
			ExpiresAt: row.ExpiresAt,
		})
	}
	r := ScoreReliability(outcomes, now)
	sig := ReporterSignal{
		Tag:                 r.Tag,
		Label:               reporterLabel(r.Tag),
		Sentence:            reporterSentence(r.Tag, r.Judged, r.Corroborated),
		JudgedReports:       r.Judged,
		CorroboratedReports: r.Corroborated,
	}
	if r.Score != nil {
		s := *r.Score
		sig.Score = &s
		sig.ScoreSentence = reporterScoreSentence(s)
	}
	return sig
}

func p10Int(n int) *int { return &n }

func TestReporterSignalsGroupByAccount(t *testing.T) {
	accountIDs, history, votes := p10GroupFixture()
	got := ReporterSignals(accountIDs, history, votes, p10Now)

	const (
		confirmedOf = " earlier reports were confirmed by people nearby."
		scoreTail   = " out of 100, based on how often their reports were confirmed."
	)
	want := map[int64]ReporterSignal{
		101: {Tag: ReporterReliable, Score: p10Int(86), Label: "Reliable reporter", Sentence: "5 of 5" + confirmedOf, ScoreSentence: "Score 86" + scoreTail, JudgedReports: 5, CorroboratedReports: 5},
		102: {Tag: ReporterMixed, Score: p10Int(71), Label: "Mixed record", Sentence: "4 of 5" + confirmedOf, ScoreSentence: "Score 71" + scoreTail, JudgedReports: 5, CorroboratedReports: 4},
		103: {Tag: ReporterUnreliable, Score: p10Int(32), Label: "Unreliable reporter", Sentence: "0 of 5" + confirmedOf, ScoreSentence: "Score 32" + scoreTail, JudgedReports: 5, CorroboratedReports: 0},
		104: {Tag: ReporterReliable, Score: p10Int(75), Label: "Reliable reporter", Sentence: "4 of 5" + confirmedOf, ScoreSentence: "Score 75" + scoreTail, JudgedReports: 5, CorroboratedReports: 4},
		105: {Tag: ReporterNew, Label: "New reporter", Sentence: "A rating needs 5 reports with a clear result. They have 4 so far.", JudgedReports: 4, CorroboratedReports: 4},
		106: {Tag: ReporterNew, Label: "New reporter", Sentence: "A rating needs 5 reports with a clear result. They have none so far."},
	}

	t.Run("six signals", func(t *testing.T) {
		for id, w := range want {
			g, ok := got[id]
			if !ok {
				t.Errorf("account %d has no entry", id)
				continue
			}
			if !reflect.DeepEqual(g, w) {
				t.Errorf("account %d signal = %+v (score %v), want %+v (score %v)", id, g, p10Deref(g.Score), w, p10Deref(w.Score))
			}
		}
	})

	t.Run("own confirm does not count", func(t *testing.T) {
		// Account 103 confirmed its own five reports. If those confirms were
		// counted each report would show 2 places and 103 would read Reliable
		// reporter 86. Reading Unreliable reporter 32 proves they were not.
		if got[103].Tag != ReporterUnreliable {
			t.Errorf("103 tag = %q, want unreliable", got[103].Tag)
		}
		// And 103's confirms still count on 101's and 102's reports.
		if got[101].CorroboratedReports != 5 || got[102].CorroboratedReports != 4 {
			t.Errorf("103's votes on other reports were lost: 101 C=%d, 102 C=%d", got[101].CorroboratedReports, got[102].CorroboratedReports)
		}
	})

	t.Run("own resolve keeps the contradicted outcome", func(t *testing.T) {
		if got[102].JudgedReports != 5 || got[102].CorroboratedReports != 4 || got[102].Tag != ReporterMixed {
			t.Errorf("102 resolve erased its record: %+v", got[102])
		}
	})

	t.Run("unrequested account has no key", func(t *testing.T) {
		if _, ok := got[107]; ok {
			t.Error("account 107 was not requested but has an entry")
		}
	})

	t.Run("exactly the requested keys", func(t *testing.T) {
		if len(got) != 6 {
			t.Errorf("map has %d keys, want 6", len(got))
		}
	})

	t.Run("stray votes change nothing", func(t *testing.T) {
		without := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(votes))
		for _, v := range votes {
			if v.ReportID != 9999 {
				without = append(without, v)
			}
		}
		if !reflect.DeepEqual(got, ReporterSignals(accountIDs, history, without, p10Now)) {
			t.Error("votes on a report outside the history changed a signal")
		}
	})

	t.Run("input order never matters", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(10, 2))
		h2 := append([]sqlcgen.ReporterHistoryReportsRow(nil), history...)
		v2 := append([]sqlcgen.CurrentVotesForReportsRow(nil), votes...)
		rng.Shuffle(len(h2), func(i, j int) { h2[i], h2[j] = h2[j], h2[i] })
		rng.Shuffle(len(v2), func(i, j int) { v2[i], v2[j] = v2[j], v2[i] })
		if !reflect.DeepEqual(got, ReporterSignals(accountIDs, h2, v2, p10Now)) {
			t.Error("shuffled input gave a different map")
		}
	})

	t.Run("matches the slow reference", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(10, 1))
		rated := 0
		for i := 0; i < 200; i++ {
			ids, h, v := p10RandomHistory(rng)
			res := ReporterSignals(ids, h, v, p10Now)
			for _, id := range ids {
				ref := p10ReferenceSignal(id, h, v, p10Now)
				g, ok := res[id]
				if !ok {
					t.Fatalf("iteration %d: account %d missing", i, id)
				}
				if g.Tag != ref.Tag || g.JudgedReports != ref.JudgedReports || g.CorroboratedReports != ref.CorroboratedReports {
					t.Fatalf("iteration %d account %d: got %+v, want %+v", i, id, g, ref)
				}
				if !reflect.DeepEqual(g, ref) {
					t.Fatalf("iteration %d account %d: got %+v (score %v), want %+v (score %v)", i, id, g, p10Deref(g.Score), ref, p10Deref(ref.Score))
				}
				if g.Tag != ReporterNew {
					rated++
				}
			}
		}
		if rated == 0 {
			t.Error("no iteration produced a rated reporter, so the property proved nothing about scores")
		}
	})
}

// p10Deref prints a score pointer for failure messages.
func p10Deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// p10Part is a slice of history rows with the votes that give them an outcome.
type p10Part struct {
	history []sqlcgen.ReporterHistoryReportsRow
	votes   []sqlcgen.CurrentVotesForReportsRow
}

// p10Batch builds n reports of one account with ids from first, all expiring
// at expiresAt. outcome is "corroborated" (two confirming places),
// "contradicted" (two disputing places) or "none" (no votes).
func p10Batch(account, first int64, n int, sev Severity, cat Category, expiresAt time.Time, outcome string) p10Part {
	var p p10Part
	for i := 0; i < n; i++ {
		id := first + int64(i)
		p.history = append(p.history, p10History(account, id, sev, cat, expiresAt))
		switch outcome {
		case "corroborated":
			p.votes = append(p.votes, p10Corroborate(id, 901, 902)...)
		case "contradicted":
			p.votes = append(p.votes, p10Contradict(id, 901, 902)...)
		}
	}
	return p
}

// p10Join concatenates parts.
func p10Join(parts ...p10Part) p10Part {
	var out p10Part
	for _, p := range parts {
		out.history = append(out.history, p.history...)
		out.votes = append(out.votes, p.votes...)
	}
	return out
}

// p10TrustInput builds a trust input for report 1 with two confirming places.
func p10TrustInput(vis Visibility, reporter *ReporterSignal) TrustInput {
	created := p10Now.Add(-time.Hour)
	return TrustInput{
		Rows: []sqlcgen.CurrentVotesForReportsRow{
			p10Vote(1, 11, VoteKindContent, VoteConfirm, "p10ta"),
			p10Vote(1, 12, VoteKindContent, VoteConfirm, "p10tb"),
		},
		ReportID:          1,
		ReporterAccountID: acctPtr(7),
		Meta:              ReportMeta{Severity: SeverityLow, Category: CategoryFlood},
		Visibility:        vis,
		CreatedAt:         created,
		ExpiresAt:         created.Add(ExpiryDuration(SeverityLow)),
		Now:               p10Now,
		Reporter:          reporter,
	}
}

func TestReporterSignalsNewAndNone(t *testing.T) {
	const acct = int64(201)
	rate := func(p p10Part) ReporterSignal {
		return ReporterSignals([]int64{acct}, p.history, p.votes, p10Now)[acct]
	}
	expect := func(t *testing.T, got ReporterSignal, tag ReporterTag, score *int, judged, corroborated int) {
		t.Helper()
		if got.Tag != tag || got.JudgedReports != judged || got.CorroboratedReports != corroborated || !reflect.DeepEqual(got.Score, score) {
			t.Errorf("got tag %q score %v J %d C %d, want tag %q score %v J %d C %d",
				got.Tag, p10Deref(got.Score), got.JudgedReports, got.CorroboratedReports, tag, p10Deref(score), judged, corroborated)
		}
	}
	const needs = "A rating needs 5 reports with a clear result."

	t.Run("one unconfirmed report", func(t *testing.T) {
		got := rate(p10Batch(acct, 1, 1, SeverityLow, CategoryFlood, p10Judged, "none"))
		expect(t, got, ReporterNew, nil, 1, 0)
		if want := needs + " They have 1 so far."; got.Sentence != want {
			t.Errorf("sentence = %q, want %q", got.Sentence, want)
		}
	})

	t.Run("one corroborated and one contradicted", func(t *testing.T) {
		got := rate(p10Join(
			p10Batch(acct, 1, 1, SeverityLow, CategoryFlood, p10Judged, "corroborated"),
			p10Batch(acct, 2, 1, SeverityLow, CategoryFlood, p10Judged, "contradicted"),
		))
		expect(t, got, ReporterNew, nil, 2, 1)
		if want := needs + " They have 2 so far."; got.Sentence != want {
			t.Errorf("sentence = %q, want %q", got.Sentence, want)
		}
	})

	t.Run("reports still in flight are not judged", func(t *testing.T) {
		got := rate(p10Batch(acct, 1, 6, SeverityLow, CategoryFlood, p10Now.Add(time.Minute), "corroborated"))
		expect(t, got, ReporterNew, nil, 0, 0)
	})

	t.Run("a report expiring exactly now is judged", func(t *testing.T) {
		got := rate(p10Batch(acct, 1, 5, SeverityLow, CategoryFlood, p10Now, "corroborated"))
		expect(t, got, ReporterReliable, p10Int(86), 5, 5)
	})

	t.Run("critical and rescue needed unconfirmed are neutral", func(t *testing.T) {
		base := p10Join(
			p10Batch(acct, 1, 5, SeverityCritical, CategoryFlood, p10Judged, "none"),
			p10Batch(acct, 10, 5, SeverityLow, CategoryRescueNeeded, p10Judged, "none"),
		)
		expect(t, rate(base), ReporterNew, nil, 0, 0)

		confirmed := p10Join(base, p10Batch(acct, 20, 1, SeverityCritical, CategoryFlood, p10Judged, "corroborated"))
		expect(t, rate(confirmed), ReporterNew, nil, 1, 1)

		disputed := p10Batch(acct, 30, 1, SeverityCritical, CategoryFlood, p10Judged, "contradicted")
		expect(t, rate(disputed), ReporterNew, nil, 1, 0)
	})

	t.Run("window edge", func(t *testing.T) {
		out := rate(p10Batch(acct, 1, 5, SeverityLow, CategoryFlood, p10Now.Add(-ReliabilityWindow), "corroborated"))
		expect(t, out, ReporterNew, nil, 0, 0)
		in := rate(p10Batch(acct, 1, 5, SeverityLow, CategoryFlood, p10Now.Add(-ReliabilityWindow).Add(time.Minute), "corroborated"))
		expect(t, in, ReporterReliable, p10Int(86), 5, 5)
	})

	t.Run("cap counts judged reports, not expired ones", func(t *testing.T) {
		// WR-05: neutral rows are dropped before the cap of 30 applies, so
		// 30 newer neutral reports no longer push five older corroborated
		// ones out, and a bad record cannot be wiped by a run of neutral
		// critical reports.
		neutral := func(n int) p10Part {
			var p p10Part
			for i := 0; i < n; i++ {
				expires := p10Now.Add(-48 * time.Hour).Add(time.Duration(i) * time.Hour)
				p.history = append(p.history, p10History(acct, int64(100+i), SeverityCritical, CategoryFlood, expires))
			}
			return p
		}
		older := p10Batch(acct, 1, 5, SeverityLow, CategoryFlood, p10Now.Add(-72*time.Hour), "corroborated")
		expect(t, rate(p10Join(older, neutral(30))), ReporterReliable, p10Int(86), 5, 5)
		expect(t, rate(p10Join(older, neutral(29))), ReporterReliable, p10Int(86), 5, 5)

		bad := p10Batch(acct, 1, 6, SeverityLow, CategoryFlood, p10Now.Add(-72*time.Hour), "contradicted")
		without := rate(bad)
		if without.Tag != ReporterUnreliable {
			t.Fatalf("setup: six contradicted reports read %q, want unreliable", without.Tag)
		}
		expect(t, rate(p10Join(bad, neutral(30))), ReporterUnreliable, without.Score, 6, 0)
	})

	t.Run("unknown severity and category fall back to low and other", func(t *testing.T) {
		row := sqlcgen.ReporterHistoryReportsRow{AccountID: acct, ReportID: 1, Severity: "bogus", Category: "bogus", ExpiresAt: p10Judged}
		got := ReporterSignals([]int64{acct}, []sqlcgen.ReporterHistoryReportsRow{row}, nil, p10Now)[acct]
		expect(t, got, ReporterNew, nil, 1, 0)
	})

	t.Run("empty request gives an empty non nil map", func(t *testing.T) {
		_, history, votes := p10GroupFixture()
		got := ReporterSignals(nil, history, votes, p10Now)
		if got == nil || len(got) != 0 {
			t.Errorf("got %v, want an empty non nil map", got)
		}
	})

	t.Run("no reporter signal reads none", func(t *testing.T) {
		want := ReporterSignal{Tag: ReporterNone}
		got := BuildTrust(p10TrustInput(VisibilityLive, nil)).Reporter
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(got, noneReporterSignal()) {
			t.Errorf("nil reporter gave %+v, want %+v", got, want)
		}
		stray := ReporterSignal{Tag: ReporterNone, Label: "stray", Sentence: "stray", JudgedReports: 3}
		if got := BuildTrust(p10TrustInput(VisibilityLive, &stray)).Reporter; !reflect.DeepEqual(got, want) {
			t.Errorf("stray none gave %+v, want %+v", got, want)
		}
		bogus := ReporterSignal{Tag: ReporterTag("bogus"), Score: p10Int(80), Label: "stray"}
		if got := BuildTrust(p10TrustInput(VisibilityLive, &bogus)).Reporter; !reflect.DeepEqual(got, want) {
			t.Errorf("bogus tag gave %+v, want %+v", got, want)
		}
	})

	ids, history, votes := p10GroupFixture()
	signals := ReporterSignals(ids, history, votes, p10Now)
	fixtureIDs := []int64{101, 102, 103, 104, 105, 106}

	t.Run("a composed signal passes through with its own score", func(t *testing.T) {
		for _, id := range fixtureIDs {
			sig := signals[id]
			in := sig
			if sig.Score != nil {
				s := *sig.Score
				in.Score = &s
			}
			got := BuildTrust(p10TrustInput(VisibilityLive, &in)).Reporter
			if !reflect.DeepEqual(got, sig) {
				t.Errorf("account %d: got %+v, want %+v", id, got, sig)
			}
			if sig.Score != nil {
				if got.Score == in.Score {
					t.Errorf("account %d: trust shares the input's score pointer", id)
				}
				*in.Score = -1
				if *got.Score != *sig.Score {
					t.Errorf("account %d: writing through the input changed the built trust", id)
				}
			}
		}
	})

	t.Run("the signal changes nothing else", func(t *testing.T) {
		stray := ReporterSignal{Tag: ReporterNone, Label: "stray"}
		inputs := []*ReporterSignal{nil, &stray}
		for _, id := range fixtureIDs {
			s := signals[id]
			inputs = append(inputs, &s)
		}
		for _, vis := range Visibilities {
			base := BuildTrust(p10TrustInput(vis, nil))
			base.Reporter = ReporterSignal{}
			for _, in := range inputs {
				got := BuildTrust(p10TrustInput(vis, in))
				got.Reporter = ReporterSignal{}
				if !reflect.DeepEqual(got, base) {
					t.Errorf("%s: a reporter signal changed the rest of the trust object: %+v vs %+v", vis, got, base)
				}
			}
		}
	})

	t.Run("field shape", func(t *testing.T) {
		rt := reflect.TypeOf(ReporterSignal{})
		want := []struct {
			name string
			typ  reflect.Type
		}{
			{"Tag", reflect.TypeOf(ReporterTag(""))},
			{"Score", reflect.TypeOf((*int)(nil))},
			{"Label", reflect.TypeOf("")},
			{"Sentence", reflect.TypeOf("")},
			{"ScoreSentence", reflect.TypeOf("")},
			{"JudgedReports", reflect.TypeOf(0)},
			{"CorroboratedReports", reflect.TypeOf(0)},
		}
		if rt.NumField() != len(want) {
			t.Fatalf("ReporterSignal has %d fields, want %d", rt.NumField(), len(want))
		}
		for i, w := range want {
			if f := rt.Field(i); f.Name != w.name || f.Type != w.typ {
				t.Errorf("field %d is %s %s, want %s %s", i, f.Name, f.Type, w.name, w.typ)
			}
		}
		tt := reflect.TypeOf(Trust{})
		last, prev := tt.Field(tt.NumField()-1), tt.Field(tt.NumField()-2)
		if last.Name != "Reporter" || last.Type != reflect.TypeOf(ReporterSignal{}) || prev.Name != "Currency" {
			t.Errorf("Trust ends with %s then %s, want Currency then Reporter ReporterSignal", prev.Name, last.Name)
		}
		f, ok := reflect.TypeOf(TrustInput{}).FieldByName("Reporter")
		if !ok || f.Type != reflect.TypeOf((*ReporterSignal)(nil)) {
			t.Errorf("TrustInput.Reporter missing or wrong type: %v", f.Type)
		}
	})
}
