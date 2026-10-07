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
		scoreTail   = " out of 100, based on how often their reports with a clear result were confirmed."
	)
	want := map[int64]ReporterSignal{
		101: {Tag: ReporterReliable, Score: p10Int(86), Label: "Reliable reporter", Sentence: "5 of 5" + confirmedOf, ScoreSentence: "Score 86" + scoreTail, JudgedReports: 5, CorroboratedReports: 5},
		102: {Tag: ReporterMixed, Score: p10Int(71), Label: "Mixed record", Sentence: "4 of 5" + confirmedOf, ScoreSentence: "Score 71" + scoreTail, JudgedReports: 5, CorroboratedReports: 4},
		103: {Tag: ReporterUnreliable, Score: p10Int(32), Label: "Unreliable reporter", Sentence: "0 of 5" + confirmedOf, ScoreSentence: "Score 32" + scoreTail, JudgedReports: 5, CorroboratedReports: 0},
		104: {Tag: ReporterReliable, Score: p10Int(75), Label: "Reliable reporter", Sentence: "4 of 5" + confirmedOf, ScoreSentence: "Score 75" + scoreTail, JudgedReports: 5, CorroboratedReports: 4},
		105: {Tag: ReporterNew, Label: "New reporter", Sentence: "Only 4 of their reports have had a clear result so far. A rating needs 5 reports with a clear result.", JudgedReports: 4, CorroboratedReports: 4},
		106: {Tag: ReporterNew, Label: "New reporter", Sentence: "None of their reports has had a clear result yet. A rating needs 5 reports with a clear result."},
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
