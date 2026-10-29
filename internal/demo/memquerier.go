// Package demo is the sandbox the Demo visit runs in: one in memory world per
// visit, with the real report and voting services running on top of it.
//
// memquerier.go is the Demo's in memory twin of the SQL the services use. It
// exists so the real rules run on pretend data (D-05, TRUST-02): the same
// SubmitChecked, Nearby, CastVote, Resolve, BuildTrust, currency and trust copy
// decide everything, and no second copy of a rule lives in JavaScript or in Go.
// Each method below names the query it mirrors in internal/store/queries.
// TestDemoParityWithSQL runs one scripted scenario through this twin and
// through a real database and compares the answers, so the twin cannot drift
// silently.
//
// Nothing here opens a connection or writes a table. Synthetic account ids
// (world.go) never reach a row anywhere else.
package demo

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The two ceilings that bound one world (D-05, T-05-12). The Demo has no
// account to rate limit, so memory, not identity, is what is protected.
const MaxReportsPerWorld = 40

// MaxVotesPerWorld is the vote ceiling of one world.
const MaxVotesPerWorld = 400

// ErrWorldFull is returned when an insert would pass one of the two ceilings.
// The insert changes nothing.
var ErrWorldFull = errors.New("demo: this practice map is full, reload the page to start again")

// noRowsMessage is the text of pgx.ErrNoRows and of database/sql's ErrNoRows
// (which pgx wraps). The services ask errors.Is(err, pgx.ErrNoRows) in several
// places to tell a missing row from a failure. This package must not import
// pgx (TestDemoPackageHasNoDatabase), so its missing row answers that question
// through Is, by the message of the target. TestMissingRowsLookLikeNoRows and
// the service level tests pin it against the real sentinel.
const noRowsMessage = "no rows in result set"

type noRowsError struct{}

func (noRowsError) Error() string { return noRowsMessage }

// Is reports true for pgx.ErrNoRows and for the sql package's ErrNoRows that
// it wraps, found by their message while unwrapping the target.
func (noRowsError) Is(target error) bool {
	for target != nil {
		switch target.Error() {
		case noRowsMessage, "sql: " + noRowsMessage:
			return true
		}
		target = errors.Unwrap(target)
	}
	return false
}

// errNoRows is the one value every missing row answers with.
var errNoRows error = noRowsError{}

// earthRadiusKm is the radius the SQL distance expression uses.
const earthRadiusKm = 6371.0

// haversineKm is the distance expression of NearbyReports, ExpiredReportsInWindow,
// OpenReportsInArea and ReportForFocus, written the way the SQL writes it (the
// spherical law of cosines with the cosine sum clamped at 1), so a distance
// from this twin matches the database one. The service's own vote radius check
// uses the true haversine form in geo.go; that is a different question and
// stays there.
func haversineKm(lat, lon, rLat, rLon float64) float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	v := math.Cos(rad(lat))*math.Cos(rad(rLat))*math.Cos(rad(rLon)-rad(lon)) +
		math.Sin(rad(lat))*math.Sin(rad(rLat))
	return earthRadiusKm * math.Acos(math.Min(1.0, v))
}

// inBox is the SQL's inclusive BETWEEN pair on latitude and longitude.
func inBox(lat, lon, latMin, latMax, lonMin, lonMax float64) bool {
	return lat >= latMin && lat <= latMax && lon >= lonMin && lon <= lonMax
}

// memReport is one stored report. The whole insert parameter set is kept, with
// created_at and expires_at restamped onto the world clock, so no column the
// reads need is lost and no database only type has to be named here.
type memReport struct {
	id int64
	p  sqlcgen.InsertReportIdempotentParams
}

// memVote is one row of the append only vote log.
type memVote struct {
	id        int64
	reportID  int64
	accountID int64
	kind      string
	value     string
	cell      string
	createdAt time.Time
}

// memQuerier implements service.Querier and service.VotingQuerier over slices
// held behind one mutex. now is the world clock: where the SQL reads the
// database clock (the NearbyReports expiry edge and the vote column default)
// this twin reads it, and where a query takes the instant as a parameter it
// uses the parameter, exactly as the SQL does.
type memQuerier struct {
	mu       sync.Mutex
	now      func() time.Time
	reports  []memReport
	votes    []memVote
	sessions map[string]int64
	lastRep  int64
	lastVote int64
}

var _ service.Querier = (*memQuerier)(nil)
var _ service.VotingQuerier = (*memQuerier)(nil)

func newMemQuerier(now func() time.Time) *memQuerier {
	return &memQuerier{now: now, sessions: make(map[string]int64)}
}

// BindSession records that sessionID belongs to accountID, the in memory
// version of a sessions row with an account. An unbound session has no
// reporter account, as a LEFT JOIN row with a NULL account would.
func (m *memQuerier) BindSession(sessionID string, accountID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = accountID
}

func (m *memQuerier) reportCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.reports)
}

func (m *memQuerier) voteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.votes)
}

// stamp truncates to microseconds, as a Postgres timestamptz keeps them.
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// reporterOf is the sessions LEFT JOIN: the account bound to the report's
// session, nil when there is none. The caller holds the lock.
func (m *memQuerier) reporterOf(r memReport) *int64 {
	acct, ok := m.sessions[r.p.SessionID]
	if !ok {
		return nil
	}
	return &acct
}

func (m *memQuerier) find(id int64) (memReport, bool) {
	for _, r := range m.reports {
		if r.id == id {
			return r, true
		}
	}
	return memReport{}, false
}

func idSet(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// reportRow builds the twelve column row every report read returns, with the
// distance from the query point.
func reportRow(r memReport, dist float64) sqlcgen.NearbyReportsRow {
	return sqlcgen.NearbyReportsRow{
		ID:                    r.id,
		Category:              r.p.Category,
		Severity:              r.p.Severity,
		Description:           r.p.Description,
		Latitude:              r.p.Latitude,
		Longitude:             r.p.Longitude,
		Geohash:               r.p.Geohash,
		ShelterCapacityStatus: r.p.ShelterCapacityStatus,
		ShelterHeadcount:      r.p.ShelterHeadcount,
		CreatedAt:             r.p.CreatedAt,
		ExpiresAt:             r.p.ExpiresAt,
		DistanceKm:            dist,
	}
}

// InsertReportIdempotent mirrors reports.sql InsertReportIdempotent: an insert
// that, when the (request_account_id, request_id) pair already exists, returns
// no row (ON CONFLICT DO NOTHING RETURNING). SubmitChecked stamps created_at
// and expires_at from the wall clock; the twin restamps created_at from the
// world clock and keeps the lifetime, so a world running in the past still
// gives a report its full ExpiryDuration. The conflict is checked before the
// ceiling so a replay still works in a full world.
func (m *memQuerier) InsertReportIdempotent(_ context.Context, arg sqlcgen.InsertReportIdempotentParams) (sqlcgen.InsertReportIdempotentRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if arg.RequestID != nil && arg.RequestAccountID != nil {
		for _, r := range m.reports {
			if r.p.RequestID != nil && r.p.RequestAccountID != nil &&
				*r.p.RequestID == *arg.RequestID && *r.p.RequestAccountID == *arg.RequestAccountID {
				return sqlcgen.InsertReportIdempotentRow{}, errNoRows
			}
		}
	}
	if len(m.reports) >= MaxReportsPerWorld {
		return sqlcgen.InsertReportIdempotentRow{}, ErrWorldFull
	}

	life := arg.ExpiresAt.Sub(arg.CreatedAt)
	created := stamp(m.now())
	arg.CreatedAt = created
	arg.ExpiresAt = created.Add(life)

	m.lastRep++
	r := memReport{id: m.lastRep, p: arg}
	m.reports = append(m.reports, r)
	return insertRow(r), nil
}

func insertRow(r memReport) sqlcgen.InsertReportIdempotentRow {
	return sqlcgen.InsertReportIdempotentRow{
		ID:                    r.id,
		Category:              r.p.Category,
		Severity:              r.p.Severity,
		Description:           r.p.Description,
		Latitude:              r.p.Latitude,
		Longitude:             r.p.Longitude,
		Geohash:               r.p.Geohash,
		ShelterCapacityStatus: r.p.ShelterCapacityStatus,
		ShelterHeadcount:      r.p.ShelterHeadcount,
		CreatedAt:             r.p.CreatedAt,
		ExpiresAt:             r.p.ExpiresAt,
		ModerationState:       r.p.ModerationState,
	}
}

// ReportByRequest mirrors reports.sql ReportByRequest: scoped to the caller's
// account together with the request id (T-04-05), no row when there is none.
func (m *memQuerier) ReportByRequest(_ context.Context, arg sqlcgen.ReportByRequestParams) (sqlcgen.ReportByRequestRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reports {
		if r.p.RequestAccountID != nil && r.p.RequestID != nil &&
			*r.p.RequestAccountID == arg.AccountID && *r.p.RequestID == arg.RequestID {
			return sqlcgen.ReportByRequestRow(insertRow(r)), nil
		}
	}
	return sqlcgen.ReportByRequestRow{}, errNoRows
}

// InsertModerationCheck mirrors moderation.sql InsertModerationCheck. The Demo
// services are built with no screener, so the local gibberish prefilter is the
// only text check and no pending or borderline row is ever asked for; nothing
// the reads need comes from this table, so the call stores nothing.
func (m *memQuerier) InsertModerationCheck(_ context.Context, _ sqlcgen.InsertModerationCheckParams) error {
	return nil
}

// NearbyReports mirrors reports.sql NearbyReports: unexpired by the world
// clock, inside the inclusive box, within the radius, nearest first. The SQL
// leaves ties unordered; the twin breaks them by id so a read is repeatable.
func (m *memQuerier) NearbyReports(_ context.Context, arg sqlcgen.NearbyReportsParams) ([]sqlcgen.NearbyReportsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	var out []sqlcgen.NearbyReportsRow
	for _, r := range m.reports {
		if !r.p.ExpiresAt.After(now) {
			continue
		}
		if !inBox(r.p.Latitude, r.p.Longitude, arg.LatMin, arg.LatMax, arg.LonMin, arg.LonMax) {
			continue
		}
		d := haversineKm(arg.Lat, arg.Lon, r.p.Latitude, r.p.Longitude)
		if d <= arg.RadiusKm {
			out = append(out, reportRow(r, d))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DistanceKm != out[j].DistanceKm {
			return out[i].DistanceKm < out[j].DistanceKm
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// ExpiredReportsInWindow mirrors history.sql ExpiredReportsInWindow: expired at
// or before the passed instant, after the window start, inside the box and the
// radius, newest expiry first, capped.
func (m *memQuerier) ExpiredReportsInWindow(_ context.Context, arg sqlcgen.ExpiredReportsInWindowParams) ([]sqlcgen.ExpiredReportsInWindowRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var picked []sqlcgen.NearbyReportsRow
	for _, r := range m.reports {
		if r.p.ExpiresAt.After(arg.Now) || !r.p.ExpiresAt.After(arg.WindowStart) {
			continue
		}
		if !inBox(r.p.Latitude, r.p.Longitude, arg.LatMin, arg.LatMax, arg.LonMin, arg.LonMax) {
			continue
		}
		d := haversineKm(arg.Lat, arg.Lon, r.p.Latitude, r.p.Longitude)
		if d <= arg.RadiusKm {
			picked = append(picked, reportRow(r, d))
		}
	}
	sort.SliceStable(picked, func(i, j int) bool {
		if !picked[i].ExpiresAt.Equal(picked[j].ExpiresAt) {
			return picked[i].ExpiresAt.After(picked[j].ExpiresAt)
		}
		return picked[i].ID > picked[j].ID
	})
	picked = capRows(picked, arg.RowCap)
	out := make([]sqlcgen.ExpiredReportsInWindowRow, 0, len(picked))
	for _, p := range picked {
		out = append(out, sqlcgen.ExpiredReportsInWindowRow(p))
	}
	return out, nil
}

// OpenReportsInArea mirrors history.sql OpenReportsInArea: unexpired after the
// passed instant (a strict edge, the mirror of the expired set), inside the box
// and the radius, nearest first then by id, capped.
func (m *memQuerier) OpenReportsInArea(_ context.Context, arg sqlcgen.OpenReportsInAreaParams) ([]sqlcgen.OpenReportsInAreaRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var picked []sqlcgen.NearbyReportsRow
	for _, r := range m.reports {
		if !r.p.ExpiresAt.After(arg.Now) {
			continue
		}
		if !inBox(r.p.Latitude, r.p.Longitude, arg.LatMin, arg.LatMax, arg.LonMin, arg.LonMax) {
			continue
		}
		d := haversineKm(arg.Lat, arg.Lon, r.p.Latitude, r.p.Longitude)
		if d <= arg.RadiusKm {
			picked = append(picked, reportRow(r, d))
		}
	}
	sort.SliceStable(picked, func(i, j int) bool {
		if picked[i].DistanceKm != picked[j].DistanceKm {
			return picked[i].DistanceKm < picked[j].DistanceKm
		}
		return picked[i].ID < picked[j].ID
	})
	picked = capRows(picked, arg.RowCap)
	out := make([]sqlcgen.OpenReportsInAreaRow, 0, len(picked))
	for _, p := range picked {
		out = append(out, sqlcgen.OpenReportsInAreaRow(p))
	}
	return out, nil
}

// capRows applies LIMIT n.
func capRows(rows []sqlcgen.NearbyReportsRow, n int32) []sqlcgen.NearbyReportsRow {
	if n < 0 {
		return rows
	}
	if int(n) < len(rows) {
		return rows[:n]
	}
	return rows
}

// ReportForFocus mirrors history.sql ReportForFocus: one report by id with no
// expiry, area or radius predicate, no row when it does not exist.
func (m *memQuerier) ReportForFocus(_ context.Context, arg sqlcgen.ReportForFocusParams) (sqlcgen.ReportForFocusRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.find(arg.ReportID)
	if !ok {
		return sqlcgen.ReportForFocusRow{}, errNoRows
	}
	return sqlcgen.ReportForFocusRow(reportRow(r, haversineKm(arg.Lat, arg.Lon, r.p.Latitude, r.p.Longitude))), nil
}

// ReporterAccountsForReports mirrors reports.sql ReporterAccountsForReports:
// one row per existing requested report, the account NULL when the session is
// unbound (the LEFT JOIN).
func (m *memQuerier) ReporterAccountsForReports(_ context.Context, reportIDs []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := idSet(reportIDs)
	var out []sqlcgen.ReporterAccountsForReportsRow
	for _, r := range m.reports {
		if _, ok := want[r.id]; ok {
			out = append(out, sqlcgen.ReporterAccountsForReportsRow{ReportID: r.id, ReporterAccountID: m.reporterOf(r)})
		}
	}
	return out, nil
}

// PublicProfilePathsForViewer answers nothing: the Demo has no people profiles,
// so no report links to one (plan 09-09).
func (m *memQuerier) PublicProfilePathsForViewer(_ context.Context, _ sqlcgen.PublicProfilePathsForViewerParams) ([]sqlcgen.PublicProfilePathsForViewerRow, error) {
	return nil, nil
}

// ReportExtrasForReports mirrors reports.sql ReportExtrasForReports: one row
// per existing requested report with its moderation state and display only
// written time. The Demo has no flags and no mutes, so both viewer booleans
// are false.
func (m *memQuerier) ReportExtrasForReports(_ context.Context, arg sqlcgen.ReportExtrasForReportsParams) ([]sqlcgen.ReportExtrasForReportsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := idSet(arg.ReportIds)
	var out []sqlcgen.ReportExtrasForReportsRow
	for _, r := range m.reports {
		if _, ok := want[r.id]; ok {
			out = append(out, sqlcgen.ReportExtrasForReportsRow{
				ReportID:        r.id,
				ModerationState: r.p.ModerationState,
				ClientWrittenAt: r.p.ClientWrittenAt,
			})
		}
	}
	return out, nil
}

// ReporterHistoryReports mirrors reliability.sql ReporterHistoryReports: for
// each requested account, its latest reports that expired at or before the
// passed instant and after the window start, newest expiry first, at most
// RowCap per account, ordered by account then expiry. Only the world's own
// reports can appear, under synthetic accounts, so no real rating is read.
func (m *memQuerier) ReporterHistoryReports(_ context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := idSet(arg.AccountIds)
	var all []sqlcgen.ReporterHistoryReportsRow
	for _, r := range m.reports {
		acct := m.reporterOf(r)
		if acct == nil {
			continue
		}
		if _, ok := want[*acct]; !ok {
			continue
		}
		if r.p.ExpiresAt.After(arg.Now) || !r.p.ExpiresAt.After(arg.WindowStart) {
			continue
		}
		all = append(all, sqlcgen.ReporterHistoryReportsRow{
			AccountID:       *acct,
			ReportID:        r.id,
			Severity:        r.p.Severity,
			Category:        r.p.Category,
			ExpiresAt:       r.p.ExpiresAt,
			ModerationState: r.p.ModerationState,
		})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].AccountID != all[j].AccountID {
			return all[i].AccountID < all[j].AccountID
		}
		if !all[i].ExpiresAt.Equal(all[j].ExpiresAt) {
			return all[i].ExpiresAt.After(all[j].ExpiresAt)
		}
		return all[i].ReportID > all[j].ReportID
	})
	var out []sqlcgen.ReporterHistoryReportsRow
	perAccount := make(map[int64]int32)
	for _, row := range all {
		if perAccount[row.AccountID] >= arg.RowCap {
			continue
		}
		perAccount[row.AccountID]++
		out = append(out, row)
	}
	return out, nil
}

// InsertVote mirrors votes.sql InsertVote: a bare append stamped from the world
// clock (the column default is the database clock) with a rising id. The
// foreign key to reports is kept: a vote on a missing report is an error.
func (m *memQuerier) InsertVote(_ context.Context, arg sqlcgen.InsertVoteParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.find(arg.ReportID); !ok {
		return errors.New("demo: vote on a report that does not exist")
	}
	if len(m.votes) >= MaxVotesPerWorld {
		return ErrWorldFull
	}
	m.lastVote++
	m.votes = append(m.votes, memVote{
		id:        m.lastVote,
		reportID:  arg.ReportID,
		accountID: arg.AccountID,
		kind:      arg.Kind,
		value:     arg.Value,
		cell:      arg.GeohashCell,
		createdAt: stamp(m.now()),
	})
	return nil
}

// newestPerReportAccountKind is DISTINCT ON (report_id, account_id, kind) with
// ORDER BY report_id, account_id, kind, created_at DESC, id DESC over the votes
// of the wanted reports: the newest row of each triple wins, ties on time go
// to the higher id, and the output is ordered by report, account and kind.
func newestPerReportAccountKind(votes []memVote, reportIDs map[int64]struct{}) []sqlcgen.CurrentVotesForReportsRow {
	type key struct {
		report, account int64
		kind            string
	}
	best := make(map[key]memVote)
	for _, v := range votes {
		if _, ok := reportIDs[v.reportID]; !ok {
			continue
		}
		k := key{v.reportID, v.accountID, v.kind}
		cur, seen := best[k]
		if !seen || v.createdAt.After(cur.createdAt) || (v.createdAt.Equal(cur.createdAt) && v.id > cur.id) {
			best[k] = v
		}
	}
	keys := make([]key, 0, len(best))
	for k := range best {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.report != b.report {
			return a.report < b.report
		}
		if a.account != b.account {
			return a.account < b.account
		}
		return a.kind < b.kind
	})
	out := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(keys))
	for _, k := range keys {
		v := best[k]
		out = append(out, sqlcgen.CurrentVotesForReportsRow{
			ReportID:    v.reportID,
			AccountID:   v.accountID,
			Kind:        v.kind,
			Value:       v.value,
			GeohashCell: v.cell,
			CreatedAt:   v.createdAt,
		})
	}
	return out
}

// CurrentVotesForReports mirrors votes.sql CurrentVotesForReports.
func (m *memQuerier) CurrentVotesForReports(_ context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return newestPerReportAccountKind(m.votes, idSet(reportIDs)), nil
}

// ReportVoteContext mirrors votes.sql ReportVoteContext: the severity,
// category, expiry, reporter account (NULL for an unbound session), position,
// creation time and moderation state of one report, no row when it is missing.
func (m *memQuerier) ReportVoteContext(_ context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.find(reportID)
	if !ok {
		return sqlcgen.ReportVoteContextRow{}, errNoRows
	}
	return sqlcgen.ReportVoteContextRow{
		Severity:          r.p.Severity,
		Category:          r.p.Category,
		ExpiresAt:         r.p.ExpiresAt,
		ReporterAccountID: m.reporterOf(r),
		Latitude:          r.p.Latitude,
		Longitude:         r.p.Longitude,
		CreatedAt:         r.p.CreatedAt,
		ModerationState:   r.p.ModerationState,
	}, nil
}

// ResolutionUndoCounts mirrors undos.sql ResolutionUndoCounts: for one account
// and kind, the number of unresolve rows and reopen rows per report over the
// whole log (every row, not only the newest), one row per report that has any
// matching row, ordered by report id.
func (m *memQuerier) ResolutionUndoCounts(_ context.Context, arg sqlcgen.ResolutionUndoCountsParams) ([]sqlcgen.ResolutionUndoCountsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := idSet(arg.ReportIds)
	counts := make(map[int64]*sqlcgen.ResolutionUndoCountsRow)
	for _, v := range m.votes {
		if v.accountID != arg.AccountID || v.kind != arg.Kind {
			continue
		}
		if _, ok := want[v.reportID]; !ok {
			continue
		}
		row, ok := counts[v.reportID]
		if !ok {
			row = &sqlcgen.ResolutionUndoCountsRow{ReportID: v.reportID}
			counts[v.reportID] = row
		}
		switch v.value {
		case arg.UnresolveValue:
			row.Unresolves++
		case arg.ReopenValue:
			row.Reopens++
		}
	}
	out := make([]sqlcgen.ResolutionUndoCountsRow, 0, len(counts))
	for _, row := range counts {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReportID < out[j].ReportID })
	return out, nil
}
