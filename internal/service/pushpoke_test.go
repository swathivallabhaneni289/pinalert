// pushpoke_test.go proves that posting a report and casting a vote only poke the
// phone alert dispatcher (plan 03.3-11, task 2). A poke carries the report id
// and the actor and nothing else; it decides nothing about notifications (D-07:
// never notify from Submit or CastVote). Nothing is poked for a refused post or
// a refused vote, and with no notifier set both behave exactly as before.

package service

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// pokeRecorder is a ReportChangeNotifier that records every poke. onPoke runs
// inside ReportChanged, so a test can see what had already been written.
type pokeRecorder struct {
	mu     sync.Mutex
	pokes  [][2]int64
	onPoke func()
}

func (p *pokeRecorder) ReportChanged(reportID, actorAccountID int64) {
	p.mu.Lock()
	p.pokes = append(p.pokes, [2]int64{reportID, actorAccountID})
	hook := p.onPoke
	p.mu.Unlock()
	if hook != nil {
		hook()
	}
}

func (p *pokeRecorder) got() [][2]int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][2]int64(nil), p.pokes...)
}

// pokeSubmitQuerier is the three methods SubmitChecked uses. The embedded nil
// Querier makes any other call a loud failure.
type pokeSubmitQuerier struct {
	Querier
	inserts   int
	insertErr error
	replayRow *sqlcgen.ReportByRequestRow
	nextID    int64
}

func (q *pokeSubmitQuerier) InsertReportIdempotent(_ context.Context, arg sqlcgen.InsertReportIdempotentParams) (sqlcgen.InsertReportIdempotentRow, error) {
	if q.insertErr != nil {
		return sqlcgen.InsertReportIdempotentRow{}, q.insertErr
	}
	q.inserts++
	q.nextID++
	return sqlcgen.InsertReportIdempotentRow{
		ID: q.nextID, Category: arg.Category, Severity: arg.Severity, Description: arg.Description,
		Latitude: arg.Latitude, Longitude: arg.Longitude, Geohash: arg.Geohash,
		CreatedAt: arg.CreatedAt, ExpiresAt: arg.ExpiresAt, ModerationState: arg.ModerationState,
	}, nil
}

func (q *pokeSubmitQuerier) ReportByRequest(_ context.Context, _ sqlcgen.ReportByRequestParams) (sqlcgen.ReportByRequestRow, error) {
	if q.replayRow == nil {
		return sqlcgen.ReportByRequestRow{}, pgx.ErrNoRows
	}
	return *q.replayRow, nil
}

func (q *pokeSubmitQuerier) InsertModerationCheck(context.Context, sqlcgen.InsertModerationCheckParams) error {
	return nil
}

func pokeSubmitInput() SubmitInput {
	return SubmitInput{
		Category:    CategoryFlood,
		Severity:    SeverityLow,
		Description: "Main road is under a foot of water",
		Latitude:    12.9716,
		Longitude:   77.5946,
	}
}

func TestSubmitPokesNotifier(t *testing.T) {
	ctx := context.Background()

	t.Run("a stored report pokes once, after the insert, with actor 0", func(t *testing.T) {
		q := &pokeSubmitQuerier{nextID: 500}
		rec := &pokeRecorder{}
		insertsAtPoke := -1
		rec.onPoke = func() { insertsAtPoke = q.inserts }
		svc := NewReportService(q, WithReportChangeNotifier(rec))

		res, err := svc.SubmitChecked(ctx, "sess-1", 7, pokeSubmitInput())
		if err != nil {
			t.Fatalf("SubmitChecked: %v", err)
		}
		got := rec.got()
		if len(got) != 1 {
			t.Fatalf("pokes = %v, want exactly one", got)
		}
		// The dispatcher's contract: the actor is the voter, or 0 for a new report.
		if got[0] != [2]int64{res.Report.ID, 0} {
			t.Errorf("poke = %v, want (%d, 0)", got[0], res.Report.ID)
		}
		if insertsAtPoke != 1 {
			t.Errorf("inserts written when the poke arrived = %d, want 1", insertsAtPoke)
		}
	})

	t.Run("a replayed request id pokes nothing", func(t *testing.T) {
		stored := sqlcgen.ReportByRequestRow{ID: 321, Category: "flood", Severity: "low"}
		q := &pokeSubmitQuerier{nextID: 500, insertErr: nil, replayRow: &stored}
		// The racing insert answers no rows, like the real conditional insert.
		replay := &pokeReplayQuerier{pokeSubmitQuerier: q}
		rec := &pokeRecorder{}
		svc := NewReportService(replay, WithReportChangeNotifier(rec))

		in := pokeSubmitInput()
		in.RequestID = "poke-request-1"
		res, err := svc.SubmitChecked(ctx, "sess-1", 7, in)
		if err != nil {
			t.Fatalf("SubmitChecked: %v", err)
		}
		if !res.Replayed {
			t.Fatalf("the fixture should replay the stored report")
		}
		if got := rec.got(); len(got) != 0 {
			t.Errorf("a replay poked %v", got)
		}
	})

	t.Run("a validation error pokes nothing", func(t *testing.T) {
		q := &pokeSubmitQuerier{nextID: 500}
		rec := &pokeRecorder{}
		svc := NewReportService(q, WithReportChangeNotifier(rec))
		in := pokeSubmitInput()
		in.Category = "not-a-category"
		if _, err := svc.SubmitChecked(ctx, "sess-1", 7, in); err == nil {
			t.Fatal("want a validation error")
		}
		if got := rec.got(); len(got) != 0 || q.inserts != 0 {
			t.Errorf("pokes %v, inserts %d", got, q.inserts)
		}
	})

	t.Run("a failed insert pokes nothing", func(t *testing.T) {
		q := &pokeSubmitQuerier{nextID: 500, insertErr: errors.New("database is down")}
		rec := &pokeRecorder{}
		svc := NewReportService(q, WithReportChangeNotifier(rec))
		if _, err := svc.SubmitChecked(ctx, "sess-1", 7, pokeSubmitInput()); err == nil {
			t.Fatal("want the insert error")
		}
		if got := rec.got(); len(got) != 0 {
			t.Errorf("pokes = %v", got)
		}
	})

	t.Run("without the option posting works as before", func(t *testing.T) {
		q := &pokeSubmitQuerier{nextID: 500}
		svc := NewReportService(q)
		res, err := svc.SubmitChecked(ctx, "sess-1", 7, pokeSubmitInput())
		if err != nil || res.Report.ID != 501 {
			t.Fatalf("SubmitChecked = %+v, %v", res.Report, err)
		}
	})

	t.Run("a nil notifier option is ignored", func(t *testing.T) {
		q := &pokeSubmitQuerier{nextID: 500}
		svc := NewReportService(q, WithReportChangeNotifier(nil))
		if _, err := svc.SubmitChecked(ctx, "sess-1", 7, pokeSubmitInput()); err != nil {
			t.Fatalf("SubmitChecked: %v", err)
		}
	})
}

// pokeReplayQuerier makes the insert answer no rows, as the conditional insert
// does when a racing send stored the request id first.
type pokeReplayQuerier struct {
	*pokeSubmitQuerier
}

func (q *pokeReplayQuerier) InsertReportIdempotent(context.Context, sqlcgen.InsertReportIdempotentParams) (sqlcgen.InsertReportIdempotentRow, error) {
	return sqlcgen.InsertReportIdempotentRow{}, pgx.ErrNoRows
}

func TestRejectedSubmitDoesNotPoke(t *testing.T) {
	ctx := context.Background()
	q := &pokeSubmitQuerier{nextID: 500}
	rec := &pokeRecorder{}
	svc := NewReportService(q, WithReportChangeNotifier(rec))

	in := pokeSubmitInput()
	in.Description = "zzzzzzzzzzzzzzzzzzzz"
	_, err := svc.SubmitChecked(ctx, "sess-1", 7, in)
	var rejected TextRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("err = %v, want a text rejection", err)
	}
	if got := rec.got(); len(got) != 0 || q.inserts != 0 {
		t.Errorf("pokes %v, inserts %d: a rejected text stores and pokes nothing", got, q.inserts)
	}
}

func TestCastVotePokesNotifier(t *testing.T) {
	ctx := context.Background()
	reporter := int64(7)
	fresh := func(mut func(*sqlcgen.ReportVoteContextRow)) (*fakeVotingQuerier, *pokeRecorder, *VotingService) {
		q := &fakeVotingQuerier{voteContextRow: testVoteContext(time.Now().Add(time.Hour), &reporter)}
		if mut != nil {
			mut(&q.voteContextRow)
		}
		rec := &pokeRecorder{}
		return q, rec, NewVotingService(q, WithVoteChangeNotifier(rec))
	}

	for _, c := range []struct {
		name  string
		voter int64
		kind  VoteKind
		value VoteValue
	}{
		{"a confirm", 9, VoteKindContent, VoteConfirm},
		{"a dispute", 9, VoteKindContent, VoteDispute},
		{"the reporter's own resolve", reporter, VoteKindResolution, VoteResolve},
		{"a nearby resolve", 9, VoteKindResolution, VoteResolve},
	} {
		t.Run(c.name+" pokes once, after the vote row is written", func(t *testing.T) {
			q, rec, svc := fresh(nil)
			votesAtPoke := -1
			rec.onPoke = func() { votesAtPoke = len(q.insertedVotes) }
			_, err := svc.CastVote(ctx, CastVoteInput{
				ReportID: 1, AccountID: c.voter, Kind: c.kind, Value: c.value,
				Latitude: testReportLat, Longitude: testReportLon,
			})
			if err != nil {
				t.Fatalf("CastVote: %v", err)
			}
			got := rec.got()
			if len(got) != 1 || got[0] != [2]int64{1, c.voter} {
				t.Fatalf("pokes = %v, want exactly (1, %d)", got, c.voter)
			}
			if votesAtPoke != 1 {
				t.Errorf("vote rows written when the poke arrived = %d, want 1", votesAtPoke)
			}
		})
	}

	refused := []struct {
		name  string
		mut   func(*sqlcgen.ReportVoteContextRow)
		voter int64
		lat   float64
	}{
		{"a voter too far away", nil, 9, testReportLat + 1},
		{"the author confirming their own report", nil, reporter, testReportLat},
		{"an expired report", func(r *sqlcgen.ReportVoteContextRow) { r.ExpiresAt = time.Now().Add(-time.Minute) }, 9, testReportLat},
		{"a withheld report", func(r *sqlcgen.ReportVoteContextRow) { r.ModerationState = string(ModerationRemoved) }, 9, testReportLat},
	}
	for _, c := range refused {
		t.Run(c.name+" does not poke", func(t *testing.T) {
			q, rec, svc := fresh(c.mut)
			_, err := svc.CastVote(ctx, CastVoteInput{
				ReportID: 1, AccountID: c.voter, Kind: VoteKindContent, Value: VoteConfirm,
				Latitude: c.lat, Longitude: testReportLon,
			})
			if err == nil {
				t.Fatal("the vote should be refused")
			}
			if got := rec.got(); len(got) != 0 {
				t.Errorf("a refused vote poked %v", got)
			}
			if slices.Contains(q.calls, "InsertVote") {
				t.Errorf("a refused vote wrote a row: %v", q.calls)
			}
		})
	}

	t.Run("a failed vote write does not poke", func(t *testing.T) {
		q, rec, svc := fresh(nil)
		q.insertVoteErr = errors.New("database is down")
		_, err := svc.CastVote(ctx, CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: testReportLat, Longitude: testReportLon,
		})
		if err == nil {
			t.Fatal("want the write error")
		}
		if got := rec.got(); len(got) != 0 {
			t.Errorf("pokes = %v", got)
		}
	})

	t.Run("without the option voting works as before", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextRow: testVoteContext(time.Now().Add(time.Hour), &reporter)}
		svc := NewVotingService(q)
		if _, err := svc.CastVote(ctx, CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: testReportLat, Longitude: testReportLon,
		}); err != nil {
			t.Fatalf("CastVote: %v", err)
		}
	})

	t.Run("a nil notifier option is ignored", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextRow: testVoteContext(time.Now().Add(time.Hour), &reporter)}
		svc := NewVotingService(q, WithVoteChangeNotifier(nil))
		if _, err := svc.CastVote(ctx, CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: testReportLat, Longitude: testReportLon,
		}); err != nil {
			t.Fatalf("CastVote: %v", err)
		}
	})
}
