package service

// updatesarea_test.go proves quick 261119-sps (owner, 2026-11-19) with fakes:
// which update rows carry the report's area, which reports a read lists for a
// lookup, and that StartAreaLookups starts at most three lookups, returns
// before any of them finishes and never looks a claimed report up again.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

func strp(s string) *string { return &s }

func TestListArea(t *testing.T) {
	exp := updatesNow.Add(5 * time.Hour)
	past := updatesNow.Add(-time.Hour)
	long := "Benz Circle Junction Road Area Near The Old Bus Stand"
	named := func(r sqlcgen.ReportsForUpdatesRow, name string) sqlcgen.ReportsForUpdatesRow {
		r.AreaName = strp(name)
		return r
	}
	tried := func(r sqlcgen.ReportsForUpdatesRow) sqlcgen.ReportsForUpdatesRow {
		r.AreaLookupTried = true
		return r
	}
	f := &fakeUpdatesQuerier{
		updates: []sqlcgen.UpdatesForAccountRow{
			updateRow(1, i64(1), UpdateReachedLive, true, 0),      // stored name
			updateRow(2, i64(2), UpdateReachedLive, true, 0),      // no name, never tried
			updateRow(3, i64(3), UpdateReachedLive, true, 0),      // no name, tried
			updateRow(4, i64(4), UpdateReachedLive, true, 0),      // removed, named
			updateRow(5, i64(5), UpdateResolved, true, 0),         // finished Rescue, named
			updateRow(6, nil, UpdateMore, true, 3),                // rolled up
			updateRow(7, nil, UpdateReachedLive, true, 0),         // about no report
			updateRow(8, i64(8), UpdateOwnShown, true, 0),         // own, named
			updateRow(9, i64(4), UpdateOwnRemoved, true, 0),       // own, removed
			updateRow(10, i64(10), UpdateReachedLive, true, 0),    // long name
			updateRow(11, i64(2), UpdateHidden, true, 0),          // same unnamed report again
			updateRow(12, i64(12), UpdateOwnShown, true, 0),       // own, no name, never tried
			updateRow(13, i64(13), UpdateResolved, true, 0),       // finished Rescue, no name
			updateRow(14, i64(14), UpdateReachedLive, true, 0),    // pending, not serious, unnamed
			updateRow(15, i64(15), UpdateReliableNearby, true, 0), // Reliable note, named
		},
		reports: []sqlcgen.ReportsForUpdatesRow{
			named(reportRow(1, SeverityLow, CategoryRoadBlocked, "Tree down", exp, ModerationClear), "Benz Circle"),
			reportRow(2, SeverityLow, CategoryFlood, "Water", exp, ModerationClear),
			tried(reportRow(3, SeverityLow, CategoryFlood, "Water", exp, ModerationClear)),
			named(reportRow(4, SeverityLow, CategoryFlood, "Removed", exp, ModerationRemoved), "Secret Place"),
			named(reportRow(5, SeverityCritical, CategoryRescueNeeded, "Family on a roof", past, ModerationClear), "Rescue Place"),
			named(reportRow(8, SeverityLow, CategoryFlood, "Mine", exp, ModerationClear), "Labbipet"),
			named(reportRow(10, SeverityLow, CategoryFlood, "Long", exp, ModerationClear), long),
			reportRow(12, SeverityLow, CategoryFlood, "Mine too", exp, ModerationClear),
			reportRow(13, SeverityCritical, CategoryRescueNeeded, "Roof", past, ModerationClear),
			reportRow(14, SeverityLow, CategoryFlood, "Held", exp, ModerationPending),
			named(reportRow(15, SeverityLow, CategoryFlood, "Note", exp, ModerationClear), "Patamata"),
		},
		reporters: []sqlcgen.ReporterAccountsForReportsRow{
			{ReportID: 1}, {ReportID: 2}, {ReportID: 3}, {ReportID: 4}, {ReportID: 5}, {ReportID: 8},
			{ReportID: 10}, {ReportID: 12}, {ReportID: 13}, {ReportID: 14}, {ReportID: 15},
		},
	}
	page, err := newTestUpdatesService(f).List(context.Background(), 7, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"Benz Circle",   // stored name
		"",              // no name yet
		"",              // tried, none found
		"",              // removed
		"",              // finished Rescue
		"",              // rolled up
		"",              // about no report
		"Labbipet",      // own report rows carry it
		"",              // own row about a removed report
		AreaWords(long), // cut like a phone alert
		"",              // the unnamed report again
		"",              // own, unnamed
		"",              // finished Rescue, unnamed
		"",              // withheld
		"Patamata",      // Reliable note
	}
	if len(page.Items) != len(want) {
		t.Fatalf("items = %d, want %d", len(page.Items), len(want))
	}
	for i, w := range want {
		if page.Items[i].Area != w {
			t.Errorf("row %d (%s) Area = %q, want %q", i, page.Items[i].Kind, page.Items[i].Area, w)
		}
	}
	if AreaWords(long) == long || AreaWords(long) == "" {
		t.Fatalf("the long name should be cut, got %q", AreaWords(long))
	}
	// The reports a read would look up: unnamed, never tried, on a row that
	// would carry an area, once each, newest row first. The tried one, the
	// withheld one and the finished Rescue report are not on it.
	wantMissing := []int64{2, 12}
	if len(page.AreaMissing) != len(wantMissing) {
		t.Fatalf("AreaMissing = %v, want %v", page.AreaMissing, wantMissing)
	}
	for i, id := range wantMissing {
		if page.AreaMissing[i] != id {
			t.Errorf("AreaMissing = %v, want %v", page.AreaMissing, wantMissing)
			break
		}
	}
}

// areaRunStore is an in memory claim store for StartAreaLookups.
type areaRunStore struct {
	mu      sync.Mutex
	tried   map[int64]bool
	names   map[int64]string
	pointed []int64
}

func newAreaRunStore() *areaRunStore {
	return &areaRunStore{tried: map[int64]bool{}, names: map[int64]string{}}
}

func (s *areaRunStore) MarkAreaLookupTried(_ context.Context, a sqlcgen.MarkAreaLookupTriedParams) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tried[a.ReportID] {
		return 0, nil
	}
	s.tried[a.ReportID] = true
	return 1, nil
}

func (s *areaRunStore) SetReportAreaName(_ context.Context, a sqlcgen.SetReportAreaNameParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.names[a.ReportID] = a.AreaName
	return nil
}

func (s *areaRunStore) ReportPointForAreaLookup(_ context.Context, id int64) (sqlcgen.ReportPointForAreaLookupRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pointed = append(s.pointed, id)
	return sqlcgen.ReportPointForAreaLookupRow{Latitude: 16.5 + float64(id)/1000, Longitude: 80.6}, nil
}

// gatedNamer blocks every call until release is closed, and counts the calls.
type gatedNamer struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
	name    string
	err     error
}

func (n *gatedNamer) ReverseArea(ctx context.Context, _, _ float64) (string, error) {
	n.mu.Lock()
	n.calls++
	n.mu.Unlock()
	if n.started != nil {
		n.started <- struct{}{}
	}
	if n.release != nil {
		select {
		case <-n.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return n.name, n.err
}

func (n *gatedNamer) Calls() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls
}

func TestStartAreaLookups(t *testing.T) {
	t.Run("at most three, one after another, and the caller does not wait", func(t *testing.T) {
		store := newAreaRunStore()
		namer := &gatedNamer{started: make(chan struct{}, 8), release: make(chan struct{}), name: "Benz Circle"}
		done := make(chan struct{})
		svc := NewUpdatesService(&fakeUpdatesQuerier{}, nil,
			WithUpdatesAreaLookup(store, namer),
			WithUpdatesAreaLookupPace(0, func() { close(done) }))

		returned := make(chan struct{})
		go func() {
			svc.StartAreaLookups([]int64{1, 2, 3, 4, 5})
			close(returned)
		}()
		select {
		case <-returned:
		case <-time.After(2 * time.Second):
			t.Fatal("StartAreaLookups waited for the lookup")
		}
		// The first lookup is under way and blocked, and the caller is back.
		select {
		case <-namer.started:
		case <-time.After(2 * time.Second):
			t.Fatal("no lookup started")
		}
		if got := namer.Calls(); got != 1 {
			t.Errorf("calls while the first is blocked = %d, want 1 (one after another)", got)
		}
		close(namer.release)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("the run did not end")
		}
		if got := namer.Calls(); got != UpdatesAreaLookupsPerRead {
			t.Errorf("calls = %d, want %d", got, UpdatesAreaLookupsPerRead)
		}
		for _, id := range []int64{1, 2, 3} {
			if store.names[id] != "Benz Circle" {
				t.Errorf("report %d name = %q, want Benz Circle", id, store.names[id])
			}
		}
		for _, id := range []int64{4, 5} {
			if store.tried[id] {
				t.Errorf("report %d was claimed past the cap", id)
			}
		}
	})

	t.Run("a claimed report is never looked up again", func(t *testing.T) {
		store := newAreaRunStore()
		store.tried[7] = true
		namer := &gatedNamer{name: "Benz Circle"}
		done := make(chan struct{}, 2)
		svc := NewUpdatesService(&fakeUpdatesQuerier{}, nil,
			WithUpdatesAreaLookup(store, namer),
			WithUpdatesAreaLookupPace(0, func() { done <- struct{}{} }))
		svc.StartAreaLookups([]int64{7})
		<-done
		if namer.Calls() != 0 {
			t.Errorf("namer called %d times for a claimed report", namer.Calls())
		}
	})

	t.Run("a failing namer stores nothing and is not retried", func(t *testing.T) {
		store := newAreaRunStore()
		namer := &gatedNamer{err: errors.New("lookup down")}
		done := make(chan struct{}, 2)
		svc := NewUpdatesService(&fakeUpdatesQuerier{}, nil,
			WithUpdatesAreaLookup(store, namer),
			WithUpdatesAreaLookupPace(0, func() { done <- struct{}{} }))
		svc.StartAreaLookups([]int64{9})
		<-done
		svc.StartAreaLookups([]int64{9})
		<-done
		if namer.Calls() != 1 {
			t.Errorf("namer called %d times, want 1", namer.Calls())
		}
		if _, ok := store.names[9]; ok {
			t.Errorf("a failed lookup stored %q", store.names[9])
		}
	})

	t.Run("no namer, no lookup and no store read", func(t *testing.T) {
		store := newAreaRunStore()
		svc := NewUpdatesService(&fakeUpdatesQuerier{}, nil, WithUpdatesAreaLookup(store, nil))
		svc.StartAreaLookups([]int64{1, 2})
		plain := NewUpdatesService(&fakeUpdatesQuerier{}, nil)
		plain.StartAreaLookups([]int64{1, 2})
		time.Sleep(50 * time.Millisecond)
		if len(store.pointed) != 0 || len(store.tried) != 0 {
			t.Errorf("a service with no namer touched the store: %v %v", store.pointed, store.tried)
		}
	})
}
