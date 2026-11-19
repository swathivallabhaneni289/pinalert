package service

// areaname.go is the one bounded reverse lookup of a report's coarse area
// name. Phone alerts (PushDispatcher.ensureAreaName), Share
// (ShareService.EnsureLink) and, since quick 261119-sps (owner, 2026-11-19),
// a signed in read of the bell (GET /api/updates) or of the Updates screen
// (UpdatesService.StartAreaLookups, in the background after the answer is
// written) all call it, so a report is looked up at most once whichever of them
// asks first: the attempt is claimed with one conditional write before the
// outside call, and only the caller that won the claim calls out.
//
// It must never be called from an unauthenticated route (Phase 8 D-06). The
// public status page only ever reads a name that is already stored, and the
// Demo never looks anything up.

import (
	"context"
	"log"
	"strings"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// AreaNameStore is the two writes the lookup makes. Both the dispatcher's
// querier and the share querier hold them.
type AreaNameStore interface {
	// MarkAreaLookupTried claims the lookup: 1 when this call made the claim,
	// 0 when the lookup was already tried (quick 261119-sps).
	MarkAreaLookupTried(ctx context.Context, arg sqlcgen.MarkAreaLookupTriedParams) (int64, error)
	SetReportAreaName(ctx context.Context, arg sqlcgen.SetReportAreaNameParams) error
}

// LookUpReportAreaName looks up the coarse area name of one report and stores
// it. The order is part of the specification: a nil namer returns "" and
// writes nothing; the attempt is claimed first, so a failing lookup is not
// retried on every call (an error from that write is logged with the report id
// and returns ""), and a claim that finds the lookup already tried returns ""
// with no outside call, so two callers at the same moment look a report up
// once; the outside call is bounded by PushAreaLookupTimeout; the answer is
// trimmed; an error or an empty name returns "" and stores nothing; otherwise
// the name is stored and returned. The error text is never logged, only the
// report id.
func LookUpReportAreaName(ctx context.Context, store AreaNameStore, namer AreaNamer, reportID int64, lat, lon float64, now time.Time) string {
	if namer == nil {
		return ""
	}
	claimed, err := store.MarkAreaLookupTried(ctx, sqlcgen.MarkAreaLookupTriedParams{ReportID: reportID, Now: now})
	if err != nil {
		log.Printf("area name: marking the area lookup of report %d: %v", reportID, err)
		return ""
	}
	if claimed == 0 {
		return ""
	}
	callCtx, cancel := context.WithTimeout(ctx, PushAreaLookupTimeout)
	name, err := namer.ReverseArea(callCtx, lat, lon)
	cancel()
	if err != nil {
		log.Printf("area name: no area name for report %d", reportID)
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if err := store.SetReportAreaName(ctx, sqlcgen.SetReportAreaNameParams{AreaName: name, ReportID: reportID}); err != nil {
		log.Printf("area name: storing the area name of report %d: %v", reportID, err)
		return ""
	}
	return name
}
