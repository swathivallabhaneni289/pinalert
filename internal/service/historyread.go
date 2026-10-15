// historyread.go holds the history reads of the report service (plan 03.1-06):
// the viewer fields every read path fills, and the reads behind the Past tab,
// the empty Active tab counts and a report link. Membership of every view comes
// from Resolve and ViewOf only. There is no second visibility authority here
// and no SQL predicate behind it (T-02-04, D-03).

package service

import (
	"context"
	"sort"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// HistoryQuery is one Past read: a centre, a radius, the viewer from the gate
// and an optional category chip ("" means all). Defaults and bounds on RadiusKm
// are the handler's job, like NearbyQuery.
type HistoryQuery struct {
	Latitude        float64
	Longitude       float64
	RadiusKm        float64
	ViewerAccountID int64
	Category        Category
}

// HistoryPage is the Past read: the Resolved group first, then the Expired
// group, each at most HistoryGroupCap long. ResolvedCut and ExpiredCut are true
// when that group had more entries than the cap, so a show more control is
// honest.
type HistoryPage struct {
	Reports     []ReportView
	ResolvedCut bool
	ExpiredCut  bool
}

// historyCandidate is one report the Past read may list. expired says it came
// from the expired set, which is the only place a Past entry can come from.
type historyCandidate struct {
	report  Report
	expired bool
}

// readContext is what one request's batched reads gave: the reporter account of
// each report, the reporters' composed signals and the vote rows by report.
type readContext struct {
	reporterByReport map[int64]*int64
	signals          map[int64]ReporterSignal
	votesByReport    map[int64][]sqlcgen.CurrentVotesForReportsRow
}

// The three candidate row types are the same twelve columns as NearbyReports,
// in the same order, so each converts to NearbyReportsRow and shares its mapper.
// A column added to one query and not the others fails to compile here.
func reportFromExpiredRow(row sqlcgen.ExpiredReportsInWindowRow) Report {
	return reportFromNearbyRow(sqlcgen.NearbyReportsRow(row))
}

func reportFromOpenRow(row sqlcgen.OpenReportsInAreaRow) Report {
	return reportFromNearbyRow(sqlcgen.NearbyReportsRow(row))
}

func reportFromFocusRow(row sqlcgen.ReportForFocusRow) Report {
	return reportFromNearbyRow(sqlcgen.NearbyReportsRow(row))
}

// historyCandidates runs the two bounded candidate reads and returns the
// expired ones first, then the open ones, each id once (Pitfall 9). The two
// sets are separate reads with separate caps so live reports can never crowd
// expired ones out (Pitfall 2). Both take the one instant the caller judges
// against.
func (s *ReportService) historyCandidates(ctx context.Context, lat, lon, radiusKm float64, now, windowStart time.Time) ([]historyCandidate, error) {
	latMin, latMax, lonMin, lonMax := BoundingBox(lat, lon, radiusKm)
	expired, err := s.q.ExpiredReportsInWindow(ctx, sqlcgen.ExpiredReportsInWindowParams{
		Lat:         lat,
		Lon:         lon,
		Now:         now,
		WindowStart: windowStart,
		LatMin:      latMin,
		LatMax:      latMax,
		LonMin:      lonMin,
		LonMax:      lonMax,
		RadiusKm:    radiusKm,
		RowCap:      int32(HistoryExpiredCandidateCap),
	})
	if err != nil {
		return nil, err
	}
	open, err := s.q.OpenReportsInArea(ctx, sqlcgen.OpenReportsInAreaParams{
		Lat:      lat,
		Lon:      lon,
		Now:      now,
		LatMin:   latMin,
		LatMax:   latMax,
		LonMin:   lonMin,
		LonMax:   lonMax,
		RadiusKm: radiusKm,
		RowCap:   int32(HistoryOpenCandidateCap),
	})
	if err != nil {
		return nil, err
	}
	out := make([]historyCandidate, 0, len(expired)+len(open))
	seen := make(map[int64]struct{}, len(expired)+len(open))
	for _, row := range expired {
		if _, ok := seen[row.ID]; ok {
			continue
		}
		seen[row.ID] = struct{}{}
		out = append(out, historyCandidate{report: reportFromExpiredRow(row), expired: true})
	}
	for _, row := range open {
		if _, ok := seen[row.ID]; ok {
			continue
		}
		seen[row.ID] = struct{}{}
		out = append(out, historyCandidate{report: reportFromOpenRow(row)})
	}
	return out, nil
}

// loadReadContext runs the batched reads of a request over the given report
// ids, in the same order Nearby does: reporter accounts once, then (only when
// withSignals and there is at least one account) the reporters' history, then
// the current votes once over the ids and the history ids, then the signals.
// Without withSignals only the reporter and vote reads run, which is all a
// count needs.
func (s *ReportService) loadReadContext(ctx context.Context, ids []int64, now time.Time, withSignals bool) (readContext, error) {
	reporterRows, err := s.q.ReporterAccountsForReports(ctx, ids)
	if err != nil {
		return readContext{}, err
	}
	reporterByReport := make(map[int64]*int64, len(reporterRows))
	for _, rr := range reporterRows {
		reporterByReport[rr.ReportID] = rr.ReporterAccountID
	}

	var accounts []int64
	var history []sqlcgen.ReporterHistoryReportsRow
	if withSignals {
		accounts = pageReporterAccounts(ids, reporterByReport)
		if len(accounts) > 0 {
			history, err = s.q.ReporterHistoryReports(ctx, reporterHistoryParams(accounts, now))
			if err != nil {
				return readContext{}, err
			}
		}
	}

	voteRows, err := s.q.CurrentVotesForReports(ctx, voteReadIDs(ids, history))
	if err != nil {
		return readContext{}, err
	}
	rc := readContext{
		reporterByReport: reporterByReport,
		votesByReport:    voteRowsByReport(voteRows),
	}
	if withSignals {
		rc.signals = ReporterSignals(accounts, history, voteRows, now)
	}
	return rc, nil
}

// classified is the one answer the resolver gives for one candidate: the
// visibility and reason from Resolve, the view from ViewOf and, for a resolved
// view, the closure replayed from the same rows.
type classified struct {
	visibility Visibility
	reason     ResolveReason
	view       FeedView
	tally      VoteTally
	closure    Closure
	reporterID *int64
	votes      []sqlcgen.CurrentVotesForReportsRow
}

// classify asks the one resolver about one candidate. Nothing here re derives a
// rung: Resolve decides the visibility, ViewOf the view and ClosureOf the
// closure, all at the request's single now (D-03, D-25).
func classify(c historyCandidate, rc readContext, now time.Time) classified {
	r := c.report
	reporterID := rc.reporterByReport[r.ID]
	votes := rc.votesByReport[r.ID]
	tally := BuildVoteTally(votes, r.ID, reporterID)
	vis, reason := Resolve(ReportMeta{Severity: r.Severity, Category: r.Category}, tally, now)
	view := ViewOf(vis, r.ExpiresAt, now)
	out := classified{visibility: vis, reason: reason, view: view, tally: tally, reporterID: reporterID, votes: votes}
	if view == FeedViewResolved {
		out.closure = ClosureOf(votes, r.ID, reporterID)
	}
	return out
}

// inHistory says whether a classified candidate is listed in the window: a
// resolved one whose current closure began after the window start, or an
// expired one that came from the expired set. Active and disputed candidates,
// and anything ViewOf does not list, are never history.
func (c classified) inHistory(fromExpiredSet bool, windowStart time.Time) bool {
	switch c.view {
	case FeedViewResolved:
		return c.closure.At.After(windowStart)
	case FeedViewPast:
		return fromExpiredSet
	default:
		return false
	}
}

// viewInput is everything one report view is built from.
type viewInput struct {
	Report          Report
	Class           classified
	Signals         map[int64]ReporterSignal
	ViewerAccountID int64
	Centre          GeoPoint
	Now             time.Time
}

// baseView builds the fields every read path fills, the same way Nearby does:
// the resolver's answer, the viewer's own votes and the trust object.
func baseView(in viewInput) ReportView {
	r := in.Report
	c := in.Class
	meta := ReportMeta{Severity: r.Severity, Category: r.Category}
	return ReportView{
		Report:           r,
		Visibility:       c.visibility,
		VisibilityReason: c.reason,
		View:             c.view,
		ViewerVote:       ViewerContentVote(c.votes, r.ID, in.ViewerAccountID),
		ViewerResolution: ViewerResolution(c.votes, r.ID, in.ViewerAccountID),
		IsOwnReport:      in.ViewerAccountID != 0 && c.reporterID != nil && *c.reporterID == in.ViewerAccountID,
		Trust: BuildTrust(TrustInput{
			Rows:              c.votes,
			ReportID:          r.ID,
			ReporterAccountID: c.reporterID,
			Meta:              meta,
			Visibility:        c.visibility,
			CreatedAt:         r.CreatedAt,
			ExpiresAt:         r.ExpiresAt,
			Now:               in.Now,
			Reporter:          reporterSignalRef(in.Signals, c.reporterID),
		}),
	}
}

// buildFinishedView builds the view of a resolved or past report: the base
// fields, the history facts and then the one coarsening of a finished Rescue
// report. Every finished view leaves the service through here, so no read path
// can forget the blur (T-31-25, D-23). The distance is measured from the query
// centre to the coarse point.
func buildFinishedView(in viewInput) ReportView {
	v := baseView(in)
	c := in.Class
	outcome := OutcomeNeutral
	if c.view == FeedViewPast {
		meta := ReportMeta{Severity: in.Report.Severity, Category: in.Report.Category}
		outcome = ClassifyOutcome(meta, c.tally, in.Report.ExpiresAt, in.Now)
	}
	viewerIsReporter := in.ViewerAccountID != 0 && c.reporterID != nil && *c.reporterID == in.ViewerAccountID
	v.History = BuildHistoryFacts(c.view, c.closure, viewerIsReporter, outcome, c.tally.ConfirmCells, in.Report.CreatedAt, in.Report.ExpiresAt)
	centre := in.Centre
	v.Report, v.Approximate = CoarsenFinishedRescue(v.Report, true, &centre)
	return v
}

// History is the Past read: what was resolved and what faded in the last
// HistoryWindow near the centre. The order is part of the specification:
//
//  1. Capture now once from the service clock, which WithReportClock can pin,
//     and derive the window start from it. The same now goes to both candidate
//     reads, to Resolve, to ViewOf, to ClosureOf and to ClassifyOutcome, so a
//     report can never be in two groups or none (Pitfall 9).
//  2. ExpiredReportsInWindow and OpenReportsInArea, each with its own candidate
//     cap, de duplicated by id. Both empty returns an empty page and no other
//     read runs.
//  3. ReporterAccountsForReports once, ReporterHistoryReports once when there
//     are accounts, CurrentVotesForReports once over the candidate ids and the
//     history ids, ReporterSignals once.
//  4. Per candidate: tally, Resolve, ViewOf. A resolved view is kept when its
//     closure began inside the window, a past view only from the expired set.
//     A category mismatch is skipped here, before any cap, so counts are
//     honest (D-17).
//  5. Sort each group (Resolved by closure time, Expired by expiry time, newest
//     first, ties by id descending), cap each at HistoryGroupCap and set its cut
//     flag, then build each kept view with buildFinishedView.
//  6. attachUndos over the kept views: one conditional ResolutionUndoCounts.
//
// The cost is a Nearby read plus one report read, and the browser does not poll
// it. Membership comes from the resolver only: there is no SQL predicate on a
// vote or a visibility (T-31-29).
func (s *ReportService) History(ctx context.Context, q HistoryQuery) (HistoryPage, error) {
	now := s.now().UTC()
	windowStart := now.Add(-HistoryWindow)

	cands, err := s.historyCandidates(ctx, q.Latitude, q.Longitude, q.RadiusKm, now, windowStart)
	if err != nil {
		return HistoryPage{}, err
	}
	if len(cands) == 0 {
		return HistoryPage{Reports: []ReportView{}}, nil
	}
	ids := make([]int64, 0, len(cands))
	for _, c := range cands {
		ids = append(ids, c.report.ID)
	}
	rc, err := s.loadReadContext(ctx, ids, now, true)
	if err != nil {
		return HistoryPage{}, err
	}

	type kept struct {
		cand  historyCandidate
		class classified
		key   time.Time
	}
	var resolved, expired []kept
	for _, c := range cands {
		cl := classify(c, rc, now)
		if !cl.inHistory(c.expired, windowStart) {
			continue
		}
		if q.Category != "" && c.report.Category != q.Category {
			continue
		}
		switch cl.view {
		case FeedViewResolved:
			resolved = append(resolved, kept{cand: c, class: cl, key: cl.closure.At})
		case FeedViewPast:
			expired = append(expired, kept{cand: c, class: cl, key: c.report.ExpiresAt})
		}
	}
	order := func(group []kept) {
		sort.SliceStable(group, func(i, j int) bool {
			if !group[i].key.Equal(group[j].key) {
				return group[i].key.After(group[j].key)
			}
			return group[i].cand.report.ID > group[j].cand.report.ID
		})
	}
	order(resolved)
	order(expired)

	page := HistoryPage{}
	if len(resolved) > HistoryGroupCap {
		resolved = resolved[:HistoryGroupCap]
		page.ResolvedCut = true
	}
	if len(expired) > HistoryGroupCap {
		expired = expired[:HistoryGroupCap]
		page.ExpiredCut = true
	}

	centre := GeoPoint{Lat: q.Latitude, Lon: q.Longitude}
	page.Reports = make([]ReportView, 0, len(resolved)+len(expired))
	for _, group := range [][]kept{resolved, expired} {
		for _, k := range group {
			page.Reports = append(page.Reports, buildFinishedView(viewInput{
				Report:          k.cand.report,
				Class:           k.class,
				Signals:         rc.signals,
				ViewerAccountID: q.ViewerAccountID,
				Centre:          centre,
				Now:             now,
			}))
		}
	}
	if err := s.attachUndos(ctx, q.ViewerAccountID, page.Reports, now); err != nil {
		return HistoryPage{}, err
	}
	return page, nil
}

// attachUndos fills UndosLeft on every view and reads the undo counts only
// when they can matter (D-22). A view is a candidate when it has not expired
// and the viewer either has a standing resolution row on it or posted it, which
// are the only two cases where a take back or a poster's reopen can exist. The
// common page has no candidate, so it returns at once and costs zero reads.
// Every other view reads UndoLimit, because nothing the viewer did can have
// spent the budget. The viewer comes from the gate, never from a query value
// (T-31-27); 0 means no identified viewer and never reads.
func (s *ReportService) attachUndos(ctx context.Context, viewerAccountID int64, views []ReportView, now time.Time) error {
	for i := range views {
		views[i].UndosLeft = UndoLimit
	}
	if viewerAccountID == 0 {
		return nil
	}
	var ids []int64
	for _, v := range views {
		if !v.ExpiresAt.After(now) {
			continue
		}
		if v.ViewerResolution == "" && !v.IsOwnReport {
			continue
		}
		ids = append(ids, v.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ResolutionUndoCounts(ctx, undoCountParams(viewerAccountID, ids))
	if err != nil {
		return err
	}
	for i := range views {
		for _, row := range rows {
			if row.ReportID != views[i].ID {
				continue
			}
			views[i].UndosLeft = UndosLeft(UndosUsed(row, views[i].IsOwnReport))
			break
		}
	}
	return nil
}
