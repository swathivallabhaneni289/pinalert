package service

// updatesarea.go starts the one area lookup of the reports on a bell or Updates
// read that have no stored name yet (quick 261119-sps, owner 2026-11-19: "in
// those updates it should display the area name"). The read never waits for
// it: the caller starts it after the answer is written, the work runs on its
// own context, and the name shows on the next read. The lookup itself is
// LookUpReportAreaName, so a report is looked up at most once ever, whoever
// asks. Only the report's own point is sent, never the reader's.

import (
	"context"
	"log"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// UpdatesAreaLookupsPerRead caps how many lookups one read starts.
	UpdatesAreaLookupsPerRead = 3
	// updatesAreaLookupSpacing is the pause between two lookups of one run.
	// The geocoder takes one request a second and refuses rather than waits,
	// and a refused lookup is already claimed, so the run goes one at a time,
	// just slower than the geocoder's pace.
	updatesAreaLookupSpacing = 1100 * time.Millisecond
	// updatesAreaLookupBudget bounds the whole of one lookup on its own
	// context: the point read, the claim, the outside call (itself bounded by
	// PushAreaLookupTimeout) and the store.
	updatesAreaLookupBudget = PushAreaLookupTimeout + 3*time.Second
)

// UpdatesAreaStore is the store surface of the background lookup: the claim
// and the stored name, and the report's own point, read only to feed the
// lookup. *sqlcgen.Queries holds all three.
type UpdatesAreaStore interface {
	AreaNameStore
	ReportPointForAreaLookup(ctx context.Context, reportID int64) (sqlcgen.ReportPointForAreaLookupRow, error)
}

var _ UpdatesAreaStore = (*sqlcgen.Queries)(nil)

// WithUpdatesAreaLookup sets the store and the reverse area lookup behind
// StartAreaLookups. Without it, or with a nil namer, StartAreaLookups does
// nothing. Only the signed in router's UpdatesService gets one.
func WithUpdatesAreaLookup(store UpdatesAreaStore, namer AreaNamer) UpdatesOption {
	return func(s *UpdatesService) {
		if store == nil || namer == nil {
			return
		}
		s.areaStore = store
		s.areaNamer = namer
	}
}

// WithUpdatesAreaLookupPace sets the pause between two lookups of one run and a
// hook called once when a run ends. Tests use it to run fast and to wait for
// the background work without sleeping; production code never needs it.
func WithUpdatesAreaLookupPace(spacing time.Duration, done func()) UpdatesOption {
	return func(s *UpdatesService) {
		s.areaSpacing = spacing
		s.areaDone = done
	}
}

// StartAreaLookups starts, in the background, the lookups of the first
// UpdatesAreaLookupsPerRead reports of ids (UpdatesPage.AreaMissing) and
// returns at once. Call it only after the answer is written, and only from a
// signed in route. The lookups run one after another on their own context,
// never the request's, each claiming its report first, so a report another
// read, a phone alert or Share already claimed is skipped with no outside call.
// A failure logs the report id only.
func (s *UpdatesService) StartAreaLookups(ids []int64) {
	if s == nil || s.areaNamer == nil || s.areaStore == nil || len(ids) == 0 {
		return
	}
	if len(ids) > UpdatesAreaLookupsPerRead {
		ids = ids[:UpdatesAreaLookupsPerRead]
	}
	run := append([]int64(nil), ids...)
	go func() {
		if s.areaDone != nil {
			defer s.areaDone()
		}
		for i, id := range run {
			if i > 0 && s.areaSpacing > 0 {
				time.Sleep(s.areaSpacing)
			}
			s.lookUpOneArea(id)
		}
	}()
}

// lookUpOneArea reads one report's point and runs the shared lookup on a fresh
// context bounded by updatesAreaLookupBudget.
func (s *UpdatesService) lookUpOneArea(reportID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), updatesAreaLookupBudget)
	defer cancel()
	pt, err := s.areaStore.ReportPointForAreaLookup(ctx, reportID)
	if err != nil {
		log.Printf("area name: reading the point of report %d for the lookup", reportID)
		return
	}
	LookUpReportAreaName(ctx, s.areaStore, s.areaNamer, reportID, pt.Latitude, pt.Longitude, s.now().UTC())
}
