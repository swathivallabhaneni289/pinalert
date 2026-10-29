// timeline_test.go pins BuildTimeline (plan 08-01, SC2): the history is built
// only from the vote log, every count is the one BuildVoteTally gives, and a
// take back never writes a downward line. It is package service and needs no
// database. Every helper starts with p801 so it cannot collide with a name from
// another plan.

package service

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

var p801Base = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

const p801Reporter int64 = 1

// p801Row is one log row on the minute-th minute after the base time. The id is
// the minute, so rows are in id order and strictly increasing in time.
func p801Row(minute int, account int64, kind VoteKind, value VoteValue, cell string) sqlcgen.TimelineVoteLogRow {
	return sqlcgen.TimelineVoteLogRow{
		ID:          int64(minute),
		AccountID:   account,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
		CreatedAt:   p801Base.Add(time.Duration(minute) * time.Minute),
	}
}

func p801Content(minute int, account int64, value VoteValue, cell string) sqlcgen.TimelineVoteLogRow {
	return p801Row(minute, account, VoteKindContent, value, cell)
}

func p801Resolution(minute int, account int64, value VoteValue, cell string) sqlcgen.TimelineVoteLogRow {
	return p801Row(minute, account, VoteKindResolution, value, cell)
}

// p801Input is a medium flood report posted at the base time, expiring in a day,
// read an hour after posting, with the reporter p801Reporter.
func p801Input(sev Severity, viewer int64, log ...sqlcgen.TimelineVoteLogRow) TimelineInput {
	rep := p801Reporter
	return TimelineInput{
		ReportID:          7,
		ReporterAccountID: &rep,
		Meta:              ReportMeta{Severity: sev, Category: CategoryFlood},
		CreatedAt:         p801Base,
		ExpiresAt:         p801Base.Add(24 * time.Hour),
		Now:               p801Base.Add(time.Hour),
		Log:               log,
		ViewerAccountID:   viewer,
	}
}

func p801Texts(evs []TimelineEvent) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.Text)
	}
	return out
}

func TestBuildTimeline(t *testing.T) {
	posted := "Posted as Medium."

	// threeDisputes: two confirms make it Live, then three disputes.
	threeDisputes := []sqlcgen.TimelineVoteLogRow{
		p801Content(1, 2, VoteConfirm, "c2"),
		p801Content(2, 3, VoteConfirm, "c3"),
		p801Content(3, 4, VoteDispute, "c4"),
		p801Content(4, 5, VoteDispute, "c5"),
		p801Content(5, 6, VoteDispute, "c6"),
	}

	// countForm: five confirms, five disputes, then a confirmer switches to a
	// dispute so the sixth disputing spot enters the Disputed view off a
	// milestone.
	var countForm []sqlcgen.TimelineVoteLogRow
	minute := 1
	for a := int64(2); a <= 6; a++ {
		countForm = append(countForm, p801Content(minute, a, VoteConfirm, "cc"+strconv.Itoa(int(a))))
		minute++
	}
	for a := int64(7); a <= 11; a++ {
		countForm = append(countForm, p801Content(minute, a, VoteDispute, "cd"+strconv.Itoa(int(a))))
		minute++
	}
	countForm = append(countForm, p801Content(minute, 2, VoteDispute, "cc2"))

	expired := p801Input(SeverityMedium, 0)
	expired.Now = p801Base.Add(48 * time.Hour)

	expiredResolved := p801Input(SeverityMedium, 0, p801Resolution(1, p801Reporter, VoteResolve, "r"))
	expiredResolved.Now = p801Base.Add(48 * time.Hour)

	cases := []struct {
		name string
		in   TimelineInput
		want []string
	}{
		{"no votes", p801Input(SeverityMedium, 0), []string{posted}},
		{"two spots go live", p801Input(SeverityMedium, 0,
			p801Content(1, 2, VoteConfirm, "c2"),
			p801Content(2, 3, VoteConfirm, "c3"),
		), []string{posted, "Confirmed by someone nearby.", "Confirmed from 2 spots. Shown to everyone."}},
		{"same cell counts once", p801Input(SeverityMedium, 0,
			p801Content(1, 2, VoteConfirm, "same"),
			p801Content(2, 3, VoteConfirm, "same"),
		), []string{posted, "Confirmed by someone nearby."}},
		{"reporter confirm adds nothing", p801Input(SeverityMedium, 0,
			p801Content(1, p801Reporter, VoteConfirm, "cr"),
		), []string{posted}},
		{"take back writes no downward line", p801Input(SeverityMedium, 0,
			p801Content(1, 2, VoteConfirm, "c2"),
			p801Content(2, 3, VoteConfirm, "c3"),
			p801Content(3, 2, VoteDispute, "c2"),
			p801Content(4, 2, VoteConfirm, "c2"),
		), []string{posted, "Confirmed by someone nearby.", "Confirmed from 2 spots. Shown to everyone.", "Disputed by someone nearby."}},
		{"disputes enter the Disputed view", p801Input(SeverityMedium, 0, threeDisputes...),
			[]string{posted, "Confirmed by someone nearby.", "Confirmed from 2 spots. Shown to everyone.",
				"Disputed by someone nearby.", "Disputed from a second spot.", "Disputed from a third spot. Marked Disputed."}},
		{"leaving the Disputed view", p801Input(SeverityMedium, 0, append(append([]sqlcgen.TimelineVoteLogRow{}, threeDisputes...),
			p801Content(6, 4, VoteConfirm, "c4"))...),
			[]string{posted, "Confirmed by someone nearby.", "Confirmed from 2 spots. Shown to everyone.",
				"Disputed by someone nearby.", "Disputed from a second spot.", "Disputed from a third spot. Marked Disputed.",
				"Confirmed from a third spot.", "No longer marked Disputed."}},
		{"count form off a milestone", p801Input(SeverityMedium, 0, countForm...),
			[]string{posted, "Confirmed by someone nearby.", "Confirmed from 2 spots. Shown to everyone.",
				"Confirmed from a third spot.", "Confirmed from a fourth spot.", "Confirmed from a fifth spot.",
				"Disputed by someone nearby.", "Disputed from a second spot.", "Disputed from a third spot.",
				"Disputed from a fourth spot.", "Disputed from a fifth spot.", "Disputed from 6 spots. Marked Disputed."}},
		{"critical is never shown to everyone later", p801Input(SeverityCritical, 0,
			p801Content(1, 2, VoteConfirm, "c2"),
			p801Content(2, 3, VoteConfirm, "c3"),
		), []string{"Posted as Critical.", "Confirmed by someone nearby.", "Confirmed from a second spot."}},
		{"poster resolves", p801Input(SeverityMedium, 0,
			p801Resolution(1, p801Reporter, VoteResolve, "r"),
		), []string{posted, "Resolved by the poster."}},
		{"poster resolves seen by the poster", p801Input(SeverityMedium, p801Reporter,
			p801Resolution(1, p801Reporter, VoteResolve, "r"),
		), []string{posted, "Resolved by you."}},
		{"poster resolves then reopens", p801Input(SeverityMedium, 0,
			p801Resolution(1, p801Reporter, VoteResolve, "r"),
			p801Resolution(2, p801Reporter, VoteReopen, "r"),
		), []string{posted, "Resolved by the poster.", "Reopened by the poster."}},
		{"take back is not a reopen", p801Input(SeverityMedium, 0,
			p801Resolution(1, p801Reporter, VoteResolve, "r"),
			p801Resolution(2, p801Reporter, VoteUnresolve, "r"),
		), []string{posted, "Resolved by the poster."}},
		{"people resolve and reopen", p801Input(SeverityMedium, 0,
			p801Resolution(1, 2, VoteResolve, "c2"),
			p801Resolution(2, 3, VoteResolve, "c3"),
			p801Resolution(3, 4, VoteReopen, "c4"),
			p801Resolution(4, 5, VoteReopen, "c5"),
		), []string{posted, "Resolved by people nearby.", "Reopened by people nearby."}},
		{"expired report fades", expired, []string{posted, "Faded out. No longer updating."}},
		{"resolved then expired does not fade", expiredResolved, []string{posted, "Resolved by the poster."}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BuildTimeline(c.in)
			if !reflect.DeepEqual(p801Texts(got), c.want) {
				t.Fatalf("texts\n got %q\nwant %q", p801Texts(got), c.want)
			}
			if again := BuildTimeline(c.in); !reflect.DeepEqual(got, again) {
				t.Fatalf("two builds differ")
			}
			for i := 1; i < len(got); i++ {
				if got[i].At.Before(got[i-1].At) {
					t.Fatalf("line %d is earlier than line %d", i, i-1)
				}
			}
		})
	}

	t.Run("tones and times", func(t *testing.T) {
		got := BuildTimeline(p801Input(SeverityMedium, 0, threeDisputes...))
		if got[0].Kind != TimelinePosted || got[0].Tone != TimelineToneNeutral || !got[0].At.Equal(p801Base) {
			t.Errorf("posted line = %+v", got[0])
		}
		if got[1].Kind != TimelineConfirmed || got[1].Tone != TimelineToneGood {
			t.Errorf("confirmed line = %+v", got[1])
		}
		last := got[len(got)-1]
		if last.Kind != TimelineDisputed || last.Tone != TimelineToneBad || !last.At.Equal(p801Base.Add(5*time.Minute)) {
			t.Errorf("marked disputed line = %+v", last)
		}
	})

	t.Run("faded at the expiry", func(t *testing.T) {
		got := BuildTimeline(expired)
		last := got[len(got)-1]
		if last.Kind != TimelineFaded || !last.At.Equal(expired.ExpiresAt) || last.Tone != TimelineToneNeutral {
			t.Errorf("faded line = %+v", last)
		}
	})

	t.Run("yours lines", func(t *testing.T) {
		in := p801Input(SeverityMedium, 2,
			p801Content(1, 2, VoteConfirm, "c2"),
			p801Content(2, 3, VoteConfirm, "c3"),
			p801Content(3, 2, VoteDispute, "c2"),
			p801Resolution(4, 2, VoteResolve, "c2"),
			p801Resolution(5, 2, VoteUnresolve, "c2"),
			p801Resolution(6, 2, VoteReopen, "c2"),
		)
		var yours []TimelineEvent
		for _, e := range BuildTimeline(in) {
			if e.Yours {
				if e.Kind != TimelineYours || e.Tone != TimelineToneNeutral {
					t.Errorf("yours line = %+v", e)
				}
				yours = append(yours, e)
			}
		}
		want := []string{"You confirmed it.", "You disputed it.", "You marked it resolved.", "You reopened it."}
		if !reflect.DeepEqual(p801Texts(yours), want) {
			t.Fatalf("yours = %q, want %q", p801Texts(yours), want)
		}
		if !yours[0].At.Equal(p801Base.Add(time.Minute)) {
			t.Errorf("first yours line at %v, want the vote's own time", yours[0].At)
		}
		in.ViewerAccountID = 0
		for _, e := range BuildTimeline(in) {
			if e.Yours {
				t.Errorf("viewer 0 got a yours line %+v", e)
			}
		}
	})

	t.Run("the poster's own resolve line is not repeated as a yours line", func(t *testing.T) {
		got := BuildTimeline(p801Input(SeverityMedium, p801Reporter,
			p801Resolution(1, p801Reporter, VoteResolve, "r"),
		))
		for _, e := range got {
			if e.Yours {
				t.Errorf("unexpected yours line %+v", e)
			}
		}
	})

	t.Run("the input log is left alone", func(t *testing.T) {
		in := p801Input(SeverityMedium, 0,
			p801Content(2, 3, VoteConfirm, "c3"),
			p801Content(1, 2, VoteConfirm, "c2"),
		)
		before := append([]sqlcgen.TimelineVoteLogRow{}, in.Log...)
		BuildTimeline(in)
		if !reflect.DeepEqual(before, in.Log) {
			t.Errorf("BuildTimeline reordered its input")
		}
	})
}

// p801Number reads the spot count a confirmed or disputed sentence states.
func p801Number(t *testing.T, text string) int {
	t.Helper()
	switch {
	case strings.Contains(text, "someone nearby"):
		return 1
	case strings.Contains(text, "a second spot"):
		return 2
	case strings.Contains(text, "a third spot"):
		return 3
	case strings.Contains(text, "a fourth spot"):
		return 4
	case strings.Contains(text, "a fifth spot"):
		return 5
	}
	for _, f := range strings.Fields(text) {
		if n, err := strconv.Atoi(f); err == nil {
			return n
		}
	}
	t.Fatalf("no count in %q", text)
	return 0
}

// TestTimelineMatchesTrust replays random logs with take backs and checks that
// each count in a sentence is the BuildVoteTally count over the standing rows
// at the log row that wrote the sentence, and never above the highest count
// reached so far. A line keeps the count it had when it was written, so after a
// take back the trust block may read lower than the last line.
func TestTimelineMatchesTrust(t *testing.T) {
	rng := rand.New(rand.NewSource(8001))
	for run := 0; run < 60; run++ {
		sev := SeverityMedium
		if run%5 == 0 {
			sev = SeverityCritical
		}
		var log []sqlcgen.TimelineVoteLogRow
		for i := 1; i <= 80; i++ {
			acct := int64(1 + rng.Intn(14))
			cell := fmt.Sprintf("cell%d", rng.Intn(12))
			switch rng.Intn(6) {
			case 0, 1:
				log = append(log, p801Content(i, acct, VoteConfirm, cell))
			case 2, 3:
				log = append(log, p801Content(i, acct, VoteDispute, cell))
			case 4:
				v := []VoteValue{VoteResolve, VoteReopen, VoteUnresolve}[rng.Intn(3)]
				log = append(log, p801Resolution(i, acct, v, cell))
			default:
				log = append(log, p801Content(i, acct, []VoteValue{VoteConfirm, VoteDispute}[rng.Intn(2)], cell))
			}
		}
		in := p801Input(sev, 0, log...)
		events := BuildTimeline(in)

		rowAt := make(map[time.Time]int)
		for i, r := range log {
			rowAt[r.CreatedAt] = i
		}
		for _, e := range events {
			if e.Kind != TimelineConfirmed && e.Kind != TimelineDisputed {
				continue
			}
			if e.Tone == TimelineToneNeutral {
				continue
			}
			i, ok := rowAt[e.At]
			if !ok {
				t.Fatalf("run %d: event %q at %v matches no log row", run, e.Text, e.At)
			}
			standing := make(map[timelineKey]sqlcgen.CurrentVotesForReportsRow)
			highConfirm, highDispute := 0, 0
			for j := 0; j <= i; j++ {
				r := log[j]
				standing[timelineKey{r.AccountID, r.Kind}] = sqlcgen.CurrentVotesForReportsRow{
					ReportID: in.ReportID, AccountID: r.AccountID, Kind: r.Kind, Value: r.Value,
					GeohashCell: r.GeohashCell, CreatedAt: r.CreatedAt,
				}
				rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(standing))
				for _, v := range standing {
					rows = append(rows, v)
				}
				tl := BuildVoteTally(rows, in.ReportID, in.ReporterAccountID)
				if tl.ConfirmCells > highConfirm {
					highConfirm = tl.ConfirmCells
				}
				if tl.DisputeCells > highDispute {
					highDispute = tl.DisputeCells
				}
			}
			rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(standing))
			for _, v := range standing {
				rows = append(rows, v)
			}
			tally := BuildVoteTally(rows, in.ReportID, in.ReporterAccountID)
			n := p801Number(t, e.Text)
			if e.Kind == TimelineConfirmed {
				if n != tally.ConfirmCells || n > highConfirm {
					t.Fatalf("run %d: %q says %d, tally has %d (highest %d)", run, e.Text, n, tally.ConfirmCells, highConfirm)
				}
			} else {
				if n != tally.DisputeCells || n > highDispute {
					t.Fatalf("run %d: %q says %d, tally has %d (highest %d)", run, e.Text, n, tally.DisputeCells, highDispute)
				}
			}
		}

		// A milestone is written at most once per side. The Marked Disputed
		// count form may repeat, because the view can be entered again.
		seen := map[string]bool{}
		for _, e := range events {
			if (e.Kind == TimelineConfirmed || e.Kind == TimelineDisputed) && !strings.Contains(e.Text, "Marked") && e.Tone != TimelineToneNeutral {
				key := string(e.Kind) + "/" + e.Text
				if seen[key] {
					t.Fatalf("run %d: %q written twice", run, e.Text)
				}
				seen[key] = true
			}
		}
	}
}
