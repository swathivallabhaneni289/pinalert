// reportersignals_test.go proves the one seam in front of every reporter tag
// read (Phase 9, plan 09-03): the loader behaves exactly like ReporterSignals
// with no source, applies what a source returns, fails the read when the source
// fails, refuses an unbounded account list, and is the only production caller.
// It declares package service (the internal test package).

package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// rsFixedSource is a SignalSource that returns a fixed option list and error,
// and records how often it was asked and for which accounts.
type rsFixedSource struct {
	opts  []SignalOption
	err   error
	calls int
	asked []int64
}

func (s *rsFixedSource) SignalOptions(_ context.Context, accountIDs []int64, _ time.Time) ([]SignalOption, error) {
	s.calls++
	s.asked = append([]int64(nil), accountIDs...)
	return s.opts, s.err
}

// rsNoQueryQuerier fails the test if the loader reaches the database.
type rsNoQueryQuerier struct{ t *testing.T }

func (q rsNoQueryQuerier) ReporterHistoryReports(context.Context, sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	q.t.Helper()
	q.t.Fatal("ReporterHistoryReports was called, want no query")
	return nil, nil
}

func (q rsNoQueryQuerier) CurrentVotesForReports(context.Context, []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	q.t.Helper()
	q.t.Fatal("CurrentVotesForReports was called, want no query")
	return nil, nil
}

// rsProductionCallRE matches a call of ReporterSignals that is not a method,
// a longer name or a qualified name.
var rsProductionCallRE = regexp.MustCompile(`(^|[^A-Za-z_.])ReporterSignals\(`)

// TestReporterSignalsOneProductionCaller is the T-09-08 guard: no production
// file calls ReporterSignals except the loader, so a source plugged into the
// loader reaches every surface.
func TestReporterSignalsOneProductionCaller(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		scanned++
		for i, line := range strings.Split(string(body), "\n") {
			if !rsProductionCallRE.MatchString(line) {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if name == "reportersignals.go" {
				continue
			}
			if name == "reporter.go" && strings.HasPrefix(trimmed, "func ReporterSignals(") {
				continue
			}
			t.Errorf("%s line %d calls ReporterSignals directly, use reporterSignalsWith or LoadReporterSignals: %s", name, i+1, trimmed)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no production files")
	}
}

// TestReporterSignalsWithNilSource: no source reads exactly what
// ReporterSignals reads.
func TestReporterSignalsWithNilSource(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	accounts := []int64{7, 9}
	want := ReporterSignals(accounts, nil, nil, now)
	got, err := reporterSignalsWith(context.Background(), nil, accounts, nil, nil, now)
	if err != nil {
		t.Fatalf("reporterSignalsWith: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("with a nil source got %+v, want %+v", got, want)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want one per requested account", len(got))
	}
}

// TestSignalSourceOptionsApplied: an option a source returns is applied once.
func TestSignalSourceOptionsApplied(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	applied := 0
	src := &rsFixedSource{opts: []SignalOption{func(cfg *signalConfig) {
		if cfg == nil {
			t.Error("option got a nil config")
		}
		applied++
	}}}
	accounts := []int64{4}
	if _, err := reporterSignalsWith(context.Background(), src, accounts, nil, nil, now); err != nil {
		t.Fatalf("reporterSignalsWith: %v", err)
	}
	if applied != 1 {
		t.Fatalf("option applied %d times, want 1", applied)
	}
	if src.calls != 1 || !reflect.DeepEqual(src.asked, accounts) {
		t.Fatalf("source asked %d times for %v, want once for %v", src.calls, src.asked, accounts)
	}
}

// TestSignalSourceErrorFailsTheRead: a source error fails the read, wrapped,
// with a nil map.
func TestSignalSourceErrorFailsTheRead(t *testing.T) {
	boom := errors.New("ratings read failed")
	src := &rsFixedSource{err: boom}
	got, err := reporterSignalsWith(context.Background(), src, []int64{1}, nil, nil, time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap %v", err, boom)
	}
	if got != nil {
		t.Fatalf("got %+v, want a nil map", got)
	}
}

// TestLoadReporterSignalsBounds: an empty list and an over cap list never
// reach the database.
func TestLoadReporterSignalsBounds(t *testing.T) {
	q := rsNoQueryQuerier{t: t}
	now := time.Now()

	got, err := LoadReporterSignals(context.Background(), q, nil, nil, now)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty list = %+v, %v, want an empty map and no error", got, err)
	}

	ids := make([]int64, MaxReporterAccountsPerRead+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if _, err := LoadReporterSignals(context.Background(), q, nil, ids, now); !errors.Is(err, ErrTooManyReporterAccounts) {
		t.Fatalf("201 ids err = %v, want ErrTooManyReporterAccounts", err)
	}
}

// TestLoadReporterSignalsMatchesFeed: for a seeded Reliable reporter the loader
// answers what the Nearby feed shows on that reporter's report.
func TestLoadReporterSignalsMatchesFeed(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	now := time.Now().UTC().Truncate(time.Second)
	const session = "p9-03-reliable-session"
	account := testutil.SeedReliableReporter(t, pool, "p9-03-reliable@example.com", session, now)
	viewer := testutil.SeedReporterAccount(t, pool, "p9-03-viewer@example.com")

	const lat, lon = 12.9716, 77.5946
	var reportID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		 VALUES ($1, 'flood', 'low', 'p9-03 live report', $2, $3, $4, now(), now() + interval '2 hours')
		 RETURNING id`,
		session, lat, lon, geohash.EncodeWithPrecision(lat, lon, 8),
	).Scan(&reportID); err != nil {
		t.Fatalf("inserting the live report: %v", err)
	}

	reports := NewReportService(q, WithReportClock(func() time.Time { return now }))
	views, err := reports.Nearby(ctx, NearbyQuery{Latitude: lat, Longitude: lon, RadiusKm: 5, ViewerAccountID: viewer, All: true})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	var feed *ReporterSignal
	for i := range views {
		if views[i].Report.ID == reportID {
			feed = &views[i].Trust.Reporter
		}
	}
	if feed == nil {
		t.Fatalf("report %d not in the feed of %d views", reportID, len(views))
	}
	if feed.Tag != ReporterReliable {
		t.Fatalf("feed tag = %q, want %q", feed.Tag, ReporterReliable)
	}

	loaded, err := LoadReporterSignals(ctx, q, nil, []int64{account}, now)
	if err != nil {
		t.Fatalf("LoadReporterSignals: %v", err)
	}
	got, ok := loaded[account]
	if !ok {
		t.Fatalf("no entry for account %d in %+v", account, loaded)
	}
	if got.Tag != feed.Tag || !reflect.DeepEqual(got.Score, feed.Score) {
		t.Fatalf("loader = %q %v, feed = %q %v", got.Tag, got.Score, feed.Tag, feed.Score)
	}
}
