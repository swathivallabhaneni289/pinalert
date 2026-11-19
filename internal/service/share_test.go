package service_test

// share_test.go proves ShareService on a real database (plan 08-02, COORD-08):
// one random token per report even under a burst of taps, a link title that
// never carries the reporter's words, a withheld report that cannot be shared,
// the area lookup at most once, and the public status read of a token in each
// of its six states.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// The description every seeded report carries. No title and no status may hold it.
const shareSecretWords = "water at my gate call 9999912345"

type shareNamer struct {
	mu    sync.Mutex
	name  string
	err   error
	calls int
}

func (n *shareNamer) ReverseArea(_ context.Context, _, _ float64) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
	return n.name, n.err
}

func (n *shareNamer) Calls() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls
}

type shareRig struct {
	pool     *pgxpool.Pool
	reports  *service.ReportService
	reporter int64
	session  string
	seq      int
}

func newShareRig(t *testing.T) *shareRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	r := &shareRig{pool: pool, session: "share-reporter-session"}
	r.reporter = testutil.SeedReporterAccount(t, pool, "share-reporter@example.com", r.session)
	r.reports = service.NewReportService(sqlcgen.New(pool))
	return r
}

func (r *shareRig) service(opts ...service.ShareOption) *service.ShareService {
	return service.NewShareService(sqlcgen.New(r.pool), r.reports, opts...)
}

// report stores one report posted by the rig's reporter.
func (r *shareRig) report(t *testing.T, severity, category, moderation string, createdAgo, expiresIn time.Duration) int64 {
	t.Helper()
	now := time.Now()
	return testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
		SessionID:       r.session,
		Category:        category,
		Severity:        severity,
		Description:     shareSecretWords,
		Latitude:        16.5062,
		Longitude:       80.6480,
		CreatedAt:       now.Add(-createdAgo),
		ExpiresAt:       now.Add(expiresIn),
		ModerationState: moderation,
	})
}

// voter makes a fresh account and returns its id.
func (r *shareRig) voter(t *testing.T) int64 {
	t.Helper()
	r.seq++
	return testutil.SeedReporterAccount(t, r.pool, fmt.Sprintf("share-voter-%d@example.com", r.seq))
}

func (r *shareRig) vote(t *testing.T, reportID, account int64, kind, value, cell string, ago time.Duration) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(),
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		reportID, account, kind, value, cell, time.Now().Add(-ago)); err != nil {
		t.Fatalf("seeding a %s %s vote: %v", kind, value, err)
	}
}

func (r *shareRig) setToken(t *testing.T, reportID int64, token string) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(), `UPDATE reports SET share_token = $1 WHERE id = $2`, token, reportID); err != nil {
		t.Fatalf("setting the token: %v", err)
	}
}

// padToken makes a 22 character token from a readable prefix.
func padToken(prefix string) string {
	return prefix + strings.Repeat("x", service.ShareTokenLen-len(prefix))
}

func (r *shareRig) tokenOf(t *testing.T, reportID int64) *string {
	t.Helper()
	var token *string
	if err := r.pool.QueryRow(context.Background(), `SELECT share_token FROM reports WHERE id = $1`, reportID).Scan(&token); err != nil {
		t.Fatalf("reading the token: %v", err)
	}
	return token
}

func TestShareLinkMintsTokenOnce(t *testing.T) {
	rig := newShareRig(t)
	namer := &shareNamer{name: "Benz Circle"}
	svc := rig.service(service.WithShareAreaNamer(namer))
	ctx := context.Background()
	id := rig.report(t, "medium", "flood", "clear", 10*time.Minute, 2*time.Hour)

	link, err := svc.EnsureLink(ctx, id, rig.reporter)
	if err != nil {
		t.Fatalf("EnsureLink: %v", err)
	}
	if !service.ValidShareToken(link.Token) {
		t.Errorf("Token = %q, want 22 base64url characters", link.Token)
	}
	if link.Title != "Flood report near Benz Circle" {
		t.Errorf("Title = %q, want Flood report near Benz Circle", link.Title)
	}
	if link.Line != service.ShareFixedLine {
		t.Errorf("Line = %q, want the fixed line", link.Line)
	}
	if strings.Contains(link.Title, "9999912345") || strings.Contains(link.Title, "gate") {
		t.Errorf("Title %q carries the reporter's words", link.Title)
	}

	again, err := svc.EnsureLink(ctx, id, rig.reporter)
	if err != nil {
		t.Fatalf("second EnsureLink: %v", err)
	}
	if again.Token != link.Token || again.Title != link.Title {
		t.Errorf("second link = %+v, want the same token and title as %+v", again, link)
	}
	if got := namer.Calls(); got != 1 {
		t.Errorf("namer called %d times over two taps, want 1", got)
	}
	if stored := rig.tokenOf(t, id); stored == nil || *stored != link.Token {
		t.Errorf("stored token = %v, want %q", stored, link.Token)
	}

	t.Run("ten taps at the same moment give one token", func(t *testing.T) {
		busy := rig.report(t, "low", "fire", "clear", 5*time.Minute, time.Hour)
		namer2 := &shareNamer{name: "Governorpet"}
		var next atomic.Int64
		// Every caller offers a different token, so only the guard can make them agree.
		source := func() (string, error) {
			n := next.Add(1)
			return fmt.Sprintf("race%018d", n), nil
		}
		racing := rig.service(service.WithShareAreaNamer(namer2), service.WithShareTokenSource(source))
		const taps = 10
		tokens := make([]string, taps)
		errs := make([]error, taps)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < taps; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				l, err := racing.EnsureLink(ctx, busy, rig.reporter)
				tokens[i], errs[i] = l.Token, err
			}(i)
		}
		close(start)
		wg.Wait()
		for i := 0; i < taps; i++ {
			if errs[i] != nil {
				t.Fatalf("tap %d: %v", i, errs[i])
			}
			if tokens[i] != tokens[0] {
				t.Errorf("tap %d token %q differs from %q", i, tokens[i], tokens[0])
			}
		}
		if got := namer2.Calls(); got != 1 {
			t.Errorf("namer called %d times across ten taps, want 1", got)
		}
		var rows int
		if err := rig.pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE share_token = $1`, tokens[0]).Scan(&rows); err != nil || rows != 1 {
			t.Errorf("reports holding the token = %d (%v), want 1", rows, err)
		}
	})
}

func TestShareLinkWithheld(t *testing.T) {
	rig := newShareRig(t)
	namer := &shareNamer{name: "Benz Circle"}
	svc := rig.service(service.WithShareAreaNamer(namer))
	ctx := context.Background()

	cases := map[string]int64{
		"a held report":         rig.report(t, "low", "flood", "pending", 5*time.Minute, time.Hour),
		"a removed report":      rig.report(t, "low", "flood", "removed", 5*time.Minute, time.Hour),
		"a report past history": rig.report(t, "low", "flood", "clear", 10*24*time.Hour, -9*24*time.Hour),
		"an unknown report":     987654321,
	}
	for name, id := range cases {
		link, err := svc.EnsureLink(ctx, id, rig.reporter)
		if !errors.Is(err, service.ErrReportNotFound) {
			t.Errorf("%s: err = %v, want ErrReportNotFound", name, err)
		}
		if link != (service.ShareLink{}) {
			t.Errorf("%s: link = %+v, want the zero link", name, link)
		}
		if id != 987654321 {
			if tok := rig.tokenOf(t, id); tok != nil {
				t.Errorf("%s: a token %q was written", name, *tok)
			}
		}
	}
	if got := namer.Calls(); got != 0 {
		t.Errorf("namer called %d times for withheld reports, want 0", got)
	}
}

func TestShareLinkAreaLookupOnce(t *testing.T) {
	ctx := context.Background()

	t.Run("a failing lookup gives a title with no area and is not retried", func(t *testing.T) {
		rig := newShareRig(t)
		namer := &shareNamer{err: errors.New("geocoder down")}
		svc := rig.service(service.WithShareAreaNamer(namer))
		id := rig.report(t, "medium", "flood", "clear", 10*time.Minute, 2*time.Hour)

		first, err := svc.EnsureLink(ctx, id, rig.reporter)
		if err != nil {
			t.Fatalf("EnsureLink: %v", err)
		}
		if first.Title != "Flood report" {
			t.Errorf("Title = %q, want Flood report", first.Title)
		}
		var tried bool
		if err := rig.pool.QueryRow(ctx, `SELECT area_lookup_at IS NOT NULL FROM push_report_progress WHERE report_id = $1`, id).Scan(&tried); err != nil || !tried {
			t.Errorf("attempt recorded = %v (%v), want true", tried, err)
		}
		second, err := svc.EnsureLink(ctx, id, rig.reporter)
		if err != nil {
			t.Fatalf("second EnsureLink: %v", err)
		}
		if second.Token != first.Token || second.Title != "Flood report" {
			t.Errorf("second link = %+v, want the same token and a title with no area", second)
		}
		if got := namer.Calls(); got != 1 {
			t.Errorf("namer called %d times, want 1", got)
		}
	})

	t.Run("a name the phone alerts already stored is used without a lookup", func(t *testing.T) {
		rig := newShareRig(t)
		namer := &shareNamer{name: "Somewhere Else"}
		svc := rig.service(service.WithShareAreaNamer(namer))
		id := rig.report(t, "medium", "flood", "clear", 10*time.Minute, 2*time.Hour)
		if _, err := rig.pool.Exec(ctx, `UPDATE reports SET area_name = 'Benz Circle' WHERE id = $1`, id); err != nil {
			t.Fatalf("storing the area name: %v", err)
		}
		link, err := svc.EnsureLink(ctx, id, rig.reporter)
		if err != nil {
			t.Fatalf("EnsureLink: %v", err)
		}
		if link.Title != "Flood report near Benz Circle" {
			t.Errorf("Title = %q, want the stored area", link.Title)
		}
		if got := namer.Calls(); got != 0 {
			t.Errorf("namer called %d times, want 0", got)
		}
	})

	t.Run("a lookup the dispatcher already tried is not repeated", func(t *testing.T) {
		rig := newShareRig(t)
		namer := &shareNamer{name: "Benz Circle"}
		svc := rig.service(service.WithShareAreaNamer(namer))
		id := rig.report(t, "medium", "flood", "clear", 10*time.Minute, 2*time.Hour)
		if _, err := sqlcgen.New(rig.pool).MarkAreaLookupTried(ctx, sqlcgen.MarkAreaLookupTriedParams{ReportID: id, Now: time.Now()}); err != nil {
			t.Fatalf("marking the lookup: %v", err)
		}
		link, err := svc.EnsureLink(ctx, id, rig.reporter)
		if err != nil {
			t.Fatalf("EnsureLink: %v", err)
		}
		if link.Title != "Flood report" {
			t.Errorf("Title = %q, want no area", link.Title)
		}
		if got := namer.Calls(); got != 0 {
			t.Errorf("namer called %d times, want 0", got)
		}
	})

	t.Run("no namer means no area and still a token", func(t *testing.T) {
		rig := newShareRig(t)
		id := rig.report(t, "medium", "road_blocked", "clear", 10*time.Minute, 2*time.Hour)
		link, err := rig.service().EnsureLink(ctx, id, rig.reporter)
		if err != nil {
			t.Fatalf("EnsureLink: %v", err)
		}
		if link.Title != "Road blocked report" || !service.ValidShareToken(link.Token) {
			t.Errorf("link = %+v", link)
		}
	})
}

// countingShareQuerier counts the queries a malformed token must not run.
type countingShareQuerier struct {
	service.ShareQuerier
	calls int
}

func (c *countingShareQuerier) ReportByShareToken(context.Context, string) (sqlcgen.ReportByShareTokenRow, error) {
	c.calls++
	return sqlcgen.ReportByShareTokenRow{}, errors.New("must not run")
}

func (c *countingShareQuerier) ReportShareFacts(context.Context, int64) (sqlcgen.ReportShareFactsRow, error) {
	c.calls++
	return sqlcgen.ReportShareFactsRow{}, errors.New("must not run")
}

func TestPublicStatusMalformedTokenRunsNoQuery(t *testing.T) {
	q := &countingShareQuerier{}
	svc := service.NewShareService(q, service.NewReportService(nil))
	for _, token := range []string{"", "42", "abc", "AbCdEfGhIjKlMnOpQr+-12", strings.Repeat("a", 23), strings.Repeat("a", 21)} {
		st, err := svc.PublicStatus(context.Background(), token)
		if err != nil {
			t.Fatalf("PublicStatus(%q): %v", token, err)
		}
		if st.State != service.PublicUnavailable || st.Title != service.PublicUnavailableTitle {
			t.Errorf("PublicStatus(%q) = %+v, want unavailable", token, st)
		}
	}
	if q.calls != 0 {
		t.Errorf("a malformed token ran %d queries, want 0", q.calls)
	}
}

func TestPublicStatusStates(t *testing.T) {
	ctx := context.Background()
	rig := newShareRig(t)
	namer := &shareNamer{name: "Benz Circle"}
	svc := rig.service(service.WithShareAreaNamer(namer))

	// publicOf shares the report (so the token and the area name exist) and
	// reads its public status.
	publicOf := func(t *testing.T, id int64) service.PublicStatus {
		t.Helper()
		link, err := svc.EnsureLink(ctx, id, rig.reporter)
		if err != nil {
			t.Fatalf("EnsureLink: %v", err)
		}
		st, err := svc.PublicStatus(ctx, link.Token)
		if err != nil {
			t.Fatalf("PublicStatus: %v", err)
		}
		assertNoPrivateFields(t, st)
		return st
	}

	t.Run("active with two confirming spots", func(t *testing.T) {
		id := rig.report(t, "medium", "flood", "clear", 12*time.Minute, 2*time.Hour)
		rig.vote(t, id, rig.voter(t), "content", "confirm", "tdr1v8y", 5*time.Minute)
		rig.vote(t, id, rig.voter(t), "content", "confirm", "tdr1vc3", 4*time.Minute)
		st := publicOf(t, id)
		if st.State != service.PublicActive || st.Chip != "Active" {
			t.Fatalf("state %q chip %q, want active and Active", st.State, st.Chip)
		}
		if st.Sentence != "Confirmed by 2 people at different spots." {
			t.Errorf("Sentence = %q", st.Sentence)
		}
		if st.Title != "Flood report near Benz Circle" {
			t.Errorf("Title = %q", st.Title)
		}
		if st.Severity != service.SeverityMedium || st.SeverityWord != "Medium" {
			t.Errorf("severity %q word %q", st.Severity, st.SeverityWord)
		}
		if st.Happening != "" || st.EndedLabel != "" || st.EndedAt != nil {
			t.Errorf("Happening %q EndedLabel %q EndedAt %v, want none", st.Happening, st.EndedLabel, st.EndedAt)
		}
		if st.PostedAt.IsZero() || time.Since(st.PostedAt) < 11*time.Minute || time.Since(st.PostedAt) > 14*time.Minute {
			t.Errorf("PostedAt = %v, want about 12 minutes ago", st.PostedAt)
		}
	})

	t.Run("active with no confirmation", func(t *testing.T) {
		id := rig.report(t, "low", "power_outage", "clear", 3*time.Minute, 2*time.Hour)
		st := publicOf(t, id)
		if st.State != service.PublicActive || st.Sentence != service.UnconfirmedLine {
			t.Errorf("state %q sentence %q, want active and %q", st.State, st.Sentence, service.UnconfirmedLine)
		}
	})

	t.Run("a critical report with a pick names the pick", func(t *testing.T) {
		id := rig.report(t, "critical", "flood", "clear", 3*time.Minute, 2*time.Hour)
		if _, err := rig.pool.Exec(ctx, `UPDATE reports SET happening = 'water_rising' WHERE id = $1`, id); err != nil {
			t.Fatalf("setting the pick: %v", err)
		}
		st := publicOf(t, id)
		if st.Happening != "Water rising" || st.SeverityWord != "Critical" {
			t.Errorf("Happening %q severity word %q", st.Happening, st.SeverityWord)
		}
	})

	t.Run("disputed", func(t *testing.T) {
		id := rig.report(t, "medium", "road_blocked", "clear", 20*time.Minute, 2*time.Hour)
		rig.vote(t, id, rig.voter(t), "content", "dispute", "tdr1v8y", 5*time.Minute)
		rig.vote(t, id, rig.voter(t), "content", "dispute", "tdr1vc3", 4*time.Minute)
		st := publicOf(t, id)
		if st.State != service.PublicDisputed || st.Chip != "Disputed" {
			t.Fatalf("state %q chip %q, want disputed and Disputed", st.State, st.Chip)
		}
		if st.Sentence != "2 people at different spots disputed this." {
			t.Errorf("Sentence = %q", st.Sentence)
		}
	})

	t.Run("resolved by the poster", func(t *testing.T) {
		id := rig.report(t, "medium", "flood", "clear", 90*time.Minute, 2*time.Hour)
		rig.vote(t, id, rig.reporter, "resolution", "resolve", "tdr1v8y", 40*time.Minute)
		st := publicOf(t, id)
		if st.State != service.PublicResolved || st.Chip != "Resolved" {
			t.Fatalf("state %q chip %q, want resolved and Resolved", st.State, st.Chip)
		}
		if st.Sentence != "Resolved by the poster." {
			t.Errorf("Sentence = %q", st.Sentence)
		}
		if st.EndedLabel != "Resolved" || st.EndedAt == nil {
			t.Fatalf("EndedLabel %q EndedAt %v", st.EndedLabel, st.EndedAt)
		}
		if ago := time.Since(*st.EndedAt); ago < 39*time.Minute || ago > 42*time.Minute {
			t.Errorf("EndedAt is %v ago, want about 40 minutes", ago)
		}
	})

	t.Run("ended", func(t *testing.T) {
		id := rig.report(t, "medium", "flood", "clear", 8*time.Hour, -2*time.Hour)
		st := publicOf(t, id)
		if st.State != service.PublicEnded || st.Chip != "Ended" {
			t.Fatalf("state %q chip %q, want ended and Ended", st.State, st.Chip)
		}
		if st.Sentence != service.PublicEndedSentence || st.EndedLabel != "Ended" || st.EndedAt == nil {
			t.Errorf("Sentence %q EndedLabel %q EndedAt %v", st.Sentence, st.EndedLabel, st.EndedAt)
		}
	})

	t.Run("removed names only the removal", func(t *testing.T) {
		id := rig.report(t, "medium", "flood", "removed", 20*time.Minute, 2*time.Hour)
		const token = "RemovedTokenRemoved_-1"
		rig.setToken(t, id, token)
		st, err := svc.PublicStatus(ctx, token)
		if err != nil {
			t.Fatalf("PublicStatus: %v", err)
		}
		want := service.PublicStatus{State: service.PublicRemoved, Title: service.PublicRemovedTitle}
		if st != want {
			t.Errorf("status = %+v, want %+v", st, want)
		}
	})

	t.Run("held, unknown and too old all read unavailable", func(t *testing.T) {
		held := rig.report(t, "low", "flood", "pending", 5*time.Minute, time.Hour)
		rig.setToken(t, held, padToken("Held"))
		old := rig.report(t, "low", "flood", "clear", 10*24*time.Hour, -9*24*time.Hour)
		rig.setToken(t, old, padToken("Old"))
		for name, token := range map[string]string{
			"held":    padToken("Held"),
			"too old": padToken("Old"),
			"unknown": padToken("NoSuch"),
		} {
			st, err := svc.PublicStatus(ctx, token)
			if err != nil {
				t.Fatalf("%s: PublicStatus: %v", name, err)
			}
			want := service.PublicStatus{State: service.PublicUnavailable, Title: service.PublicUnavailableTitle}
			if st != want {
				t.Errorf("%s: status = %+v, want %+v", name, st, want)
			}
		}
	})

	t.Run("the public read never calls the area lookup", func(t *testing.T) {
		id := rig.report(t, "low", "fire", "clear", 3*time.Minute, time.Hour)
		token := padToken("NoLookup")
		rig.setToken(t, id, token)
		before := namer.Calls()
		st, err := svc.PublicStatus(ctx, token)
		if err != nil {
			t.Fatalf("PublicStatus: %v", err)
		}
		if st.State != service.PublicActive || st.Title != "Fire report" {
			t.Errorf("status = %+v, want an active Fire report with no area", st)
		}
		if namer.Calls() != before {
			t.Errorf("the public read called the namer")
		}
	})
}

// assertNoPrivateFields fails when a status holds the reporter's words or a
// coordinate anywhere in its printed form.
func assertNoPrivateFields(t *testing.T, st service.PublicStatus) {
	t.Helper()
	printed := fmt.Sprintf("%+v", st)
	for _, bad := range []string{"gate", "9999912345", "16.5062", "80.648"} {
		if strings.Contains(printed, bad) {
			t.Errorf("status %q holds %q", printed, bad)
		}
	}
}
