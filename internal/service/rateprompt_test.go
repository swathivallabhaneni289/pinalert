package service_test

// rateprompt_test.go proves the rate prompt of the resolve vote (Phase 9, plan
// 09-17, D-02): the server alone decides when the skippable star box after Yes,
// mark resolved is offered, and a failed read never fails the vote (T-09-62,
// T-09-63, T-09-64). The tests skip without DATABASE_URL (NewTestDB).

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// The report of testutil.SeedHistoryReport sits here.
const ratePromptLat, ratePromptLon = 12.9716, 77.5946

// ratePromptScene is one public Reliable reporter with one live report, and a
// voting service that has a rate prompter.
type ratePromptScene struct {
	t        *testing.T
	pool     *pgxpool.Pool
	q        *sqlcgen.Queries
	votes    *service.VotingService
	ratings  *service.RatingService
	reporter int64
	report   int64
	username string
	seq      int
}

func newRatePromptScene(t *testing.T) *ratePromptScene {
	t.Helper()
	pool := testutil.NewTestDB(t)
	s := &ratePromptScene{t: t, pool: pool, q: sqlcgen.New(pool), username: "prompt.reporter"}
	s.ratings = service.NewRatingService(s.q)
	s.votes = service.NewVotingService(s.q, service.WithVotingSignals(s.ratings), service.WithVotingRatePrompts(s.ratings))
	s.reporter = testutil.SeedPublicReporter(t, pool, "prompt-reporter@example.com", s.username, "prompt-session", time.Now().UTC())
	s.report = testutil.SeedHistoryReport(t, pool, "prompt-session", "low", "flood", time.Now().UTC().Add(time.Hour))
	return s
}

// reader makes a signed in account with a clear username and no vote.
func (s *ratePromptScene) reader() int64 {
	s.t.Helper()
	s.seq++
	name := fmt.Sprintf("prompt.reader.%d", s.seq)
	id := testutil.SeedReporterAccount(s.t, s.pool, name+"@example.com")
	testutil.SeedUsername(s.t, s.pool, id, name, "clear")
	return id
}

// cast casts one vote from the report's own place.
func (s *ratePromptScene) cast(svc *service.VotingService, account int64, kind service.VoteKind, value service.VoteValue) service.CastVoteResult {
	s.t.Helper()
	res, err := svc.CastVote(context.Background(), service.CastVoteInput{
		ReportID: s.report, AccountID: account, Kind: kind, Value: value,
		Latitude: ratePromptLat, Longitude: ratePromptLon,
	})
	if err != nil {
		s.t.Fatalf("CastVote %s %s by %d: %v", kind, value, account, err)
	}
	return res
}

func (s *ratePromptScene) wantPrompt(label string, res service.CastVoteResult) {
	s.t.Helper()
	if res.RatePrompt == nil {
		s.t.Errorf("%s: RatePrompt is nil, want a prompt for %s", label, s.username)
		return
	}
	if res.RatePrompt.Username != s.username || res.RatePrompt.ProfilePath != "/u/"+s.username {
		s.t.Errorf("%s: RatePrompt = %+v, want %s and /u/%s", label, *res.RatePrompt, s.username, s.username)
	}
}

func (s *ratePromptScene) wantNone(label string, res service.CastVoteResult) {
	s.t.Helper()
	if res.RatePrompt != nil {
		s.t.Errorf("%s: RatePrompt = %+v, want none", label, *res.RatePrompt)
	}
}

// failingPrompter answers every question with an error.
type failingPrompter struct{}

func (failingPrompter) MayRate(context.Context, int64, int64) (bool, error) {
	return false, errors.New("prompter down")
}

func (failingPrompter) YourRating(context.Context, int64, int64) (int, bool, error) {
	return 0, false, errors.New("prompter down")
}

// TestRatePromptConditions covers every condition of the prompt: it appears
// after a resolve for an eligible reader who has not rated, and nowhere else.
func TestRatePromptConditions(t *testing.T) {
	resolve := func(s *ratePromptScene, account int64) service.CastVoteResult {
		return s.cast(s.votes, account, service.VoteKindResolution, service.VoteResolve)
	}

	t.Run("a witness at the scene whose only vote is the resolve", func(t *testing.T) {
		s := newRatePromptScene(t)
		s.wantPrompt("resolve from the report's own cell", resolve(s, s.reader()))
	})

	t.Run("a reader with an earlier confirm", func(t *testing.T) {
		s := newRatePromptScene(t)
		r := s.reader()
		s.cast(s.votes, r, service.VoteKindContent, service.VoteConfirm)
		s.wantPrompt("resolve after a confirm", resolve(s, r))
	})

	t.Run("your own report", func(t *testing.T) {
		s := newRatePromptScene(t)
		s.wantNone("the reporter's own resolve", resolve(s, s.reporter))
	})

	t.Run("a private reporter", func(t *testing.T) {
		s := newRatePromptScene(t)
		testutil.MustExec(t, s.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, s.reporter)
		s.wantNone("profile off", resolve(s, s.reader()))
	})

	t.Run("a reporter who blocked the reader", func(t *testing.T) {
		s := newRatePromptScene(t)
		r := s.reader()
		testutil.SeedBlock(t, s.pool, s.reporter, r)
		s.wantNone("blocked by the reporter", resolve(s, r))
	})

	t.Run("a reporter the reader blocked", func(t *testing.T) {
		s := newRatePromptScene(t)
		r := s.reader()
		testutil.SeedBlock(t, s.pool, r, s.reporter)
		s.wantNone("blocked by the reader", resolve(s, r))
	})

	t.Run("a reader who already rated", func(t *testing.T) {
		s := newRatePromptScene(t)
		r := s.reader()
		s.cast(s.votes, r, service.VoteKindContent, service.VoteConfirm)
		if err := s.ratings.Rate(context.Background(), r, s.username, 4); err != nil {
			t.Fatalf("Rate: %v", err)
		}
		s.wantNone("resolve after a rating", resolve(s, r))
	})

	t.Run("confirm, dispute and reopen carry no prompt", func(t *testing.T) {
		s := newRatePromptScene(t)
		s.wantNone("confirm", s.cast(s.votes, s.reader(), service.VoteKindContent, service.VoteConfirm))
		s.wantNone("dispute", s.cast(s.votes, s.reader(), service.VoteKindContent, service.VoteDispute))
		s.wantNone("reopen", s.cast(s.votes, s.reader(), service.VoteKindResolution, service.VoteReopen))
	})

	t.Run("a take back carries no prompt", func(t *testing.T) {
		s := newRatePromptScene(t)
		r := s.reader()
		resolve(s, r)
		s.wantNone("unresolve", s.cast(s.votes, r, service.VoteKindResolution, service.VoteUnresolve))
	})

	t.Run("no prompter is set", func(t *testing.T) {
		s := newRatePromptScene(t)
		plain := service.NewVotingService(s.q, service.WithVotingSignals(s.ratings))
		s.wantNone("no prompter", s.cast(plain, s.reader(), service.VoteKindResolution, service.VoteResolve))
	})

	t.Run("a failing prompter never fails the vote", func(t *testing.T) {
		s := newRatePromptScene(t)
		broken := service.NewVotingService(s.q, service.WithVotingRatePrompts(failingPrompter{}))
		res := s.cast(broken, s.reader(), service.VoteKindResolution, service.VoteResolve)
		s.wantNone("prompter error", res)
		if res.Visibility == "" {
			t.Errorf("the vote answered with no visibility: %+v", res)
		}
	})
}
