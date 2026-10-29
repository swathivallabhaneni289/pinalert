// profilereports.go is the server read behind a profile's Recent reports (plan
// 09-05, PROF-02, D-06, D-07): the person's reports from the last 30 days,
// newest first, at most 100, through the same loadReadContext, classify and view
// builders as Past and Focus. There is no second visibility rule here and no SQL
// predicate on moderation or view: a withheld report fails closed in classify
// and is never listed (T-09-18).
//
// A live report keeps its exact point and its words, as on the map, because the
// same point is already public while the report is live (T-09-17). A finished
// report (resolved or expired) is blurred by the profile coarsener inside
// buildFinishedView and then loses everything that could lead back to its exact
// place: its id (so Focus, which does not blur a non Rescue report, is never one
// call away), its description, and every time except the end date (T-09-16,
// D-20).

package service

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// ProfileReportsWindow is how far back a profile lists reports (D-07).
	ProfileReportsWindow = 30 * 24 * time.Hour
	// ProfileReportsCap is the most rows a profile lists (D-07).
	ProfileReportsCap = 100
	// ProfileReportsCutText says the list was cut at the cap.
	ProfileReportsCutText = "Showing the newest 100."
	// DisputedMayNotBeTrueLine is the Nearby wording for a disputed report.
	DisputedMayNotBeTrueLine = "Disputed: may not be true"
)

// The five status values of a profile row and their words.
const (
	ProfileStatusOngoing      = "ongoing"
	ProfileStatusNotConfirmed = "not_confirmed"
	ProfileStatusDisputed     = "disputed"
	ProfileStatusResolved     = "resolved"
	ProfileStatusExpired      = "expired"

	ProfileStatusWordOngoing      = "Ongoing"
	ProfileStatusWordNotConfirmed = "Not confirmed yet"
	ProfileStatusWordDisputed     = "Disputed"
	ProfileStatusWordResolved     = "Resolved"
	ProfileStatusWordExpired      = "Expired"
)

// profileDateZone is the zone an end date is shown in: India Standard Time, a
// fixed offset so no time zone database is needed.
var profileDateZone = time.FixedZone("IST", 5*3600+30*60)

// ErrProfileReadUnavailable is returned when the service's querier cannot read
// profile candidates. The real querier always can.
var ErrProfileReadUnavailable = errors.New("service: profile reports are not available on this querier")

// profileCandidateReader is the one read ProfileReports adds. It is its own
// small interface, like VotingQuerier, so the many fakes of Querier need no new
// method; the generated *sqlcgen.Queries satisfies it.
type profileCandidateReader interface {
	ProfileReportCandidates(ctx context.Context, arg sqlcgen.ProfileReportCandidatesParams) ([]sqlcgen.ProfileReportCandidatesRow, error)
}

// ProfileQuery is one profile list read. AccountID is the person whose reports
// are listed; ViewerAccountID is the reader from the gate. Centre is the
// reader's own centre, nil when none is known, and Now is the instant to judge
// against (zero means the service clock).
type ProfileQuery struct {
	AccountID       int64
	ViewerAccountID int64
	Centre          *GeoPoint
	Now             time.Time
}

// ProfileReportRow is one listed report. A live row carries the full view. A
// finished row carries a blurred view with no id, no description and no time:
// its CreatedAt, ExpiresAt and closure time are cleared here, so no mapper can
// send a time that would match the row to the anonymous Past row at its exact
// point. The row's words are filled by the service, not by the browser.
type ProfileReportRow struct {
	View          ReportView
	Live          bool
	Status        string
	StatusWord    string
	FinishedTitle string
	AreaLine      string
	DistanceText  string
	DisputedLine  string
	// EndedOn is the end date only, such as "12 October 2026", never a time (D-20).
	EndedOn string
}

// ProfileReportsPage is the list and whether it was cut at the cap.
type ProfileReportsPage struct {
	Rows    []ProfileReportRow
	Cut     bool
	CutText string
}

// ProfileReports lists one person's reports from the last ProfileReportsWindow.
// The order is part of the specification: one candidate read, then the shared
// loadReadContext and classify, then a live view from baseView or a finished
// view from buildFinishedView with the profile coarsener.
func (s *ReportService) ProfileReports(ctx context.Context, q ProfileQuery) (ProfileReportsPage, error) {
	reader, ok := s.q.(profileCandidateReader)
	if !ok {
		return ProfileReportsPage{}, ErrProfileReadUnavailable
	}
	now := q.Now
	if now.IsZero() {
		now = s.now()
	}
	now = now.UTC()

	rows, err := reader.ProfileReportCandidates(ctx, sqlcgen.ProfileReportCandidatesParams{
		AccountID: q.AccountID,
		Since:     now.Add(-ProfileReportsWindow),
		RowLimit:  int32(ProfileReportsCap + 1),
	})
	if err != nil {
		return ProfileReportsPage{}, err
	}
	page := ProfileReportsPage{Rows: []ProfileReportRow{}}
	if len(rows) == 0 {
		return page, nil
	}
	if len(rows) > ProfileReportsCap {
		rows = rows[:ProfileReportsCap]
		page.Cut = true
		page.CutText = ProfileReportsCutText
	}

	cands := make([]historyCandidate, 0, len(rows))
	areas := make(map[int64]string, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		r := reportFromNearbyRow(sqlcgen.NearbyReportsRow{
			ID:                    row.ID,
			Category:              row.Category,
			Severity:              row.Severity,
			Description:           row.Description,
			Latitude:              row.Latitude,
			Longitude:             row.Longitude,
			Geohash:               row.Geohash,
			ShelterCapacityStatus: row.ShelterCapacityStatus,
			ShelterHeadcount:      row.ShelterHeadcount,
			CreatedAt:             row.CreatedAt,
			ExpiresAt:             row.ExpiresAt,
		})
		// The candidate read measured no distance. The service sets one below,
		// to the exact point of a live report and to the coarse point of a
		// finished one, never to a finished report's exact point.
		r.DistanceKm = nil
		cands = append(cands, historyCandidate{report: r, expired: !r.ExpiresAt.After(now)})
		ids = append(ids, r.ID)
		if row.AreaName != nil {
			areas[r.ID] = *row.AreaName
		}
	}

	rc, err := s.loadReadContext(ctx, ids, now, true, q.ViewerAccountID)
	if err != nil {
		return ProfileReportsPage{}, err
	}

	for _, c := range cands {
		cl := classify(c, rc, now)
		in := viewInput{
			Report:          c.report,
			Class:           cl,
			Signals:         rc.signals,
			ViewerAccountID: q.ViewerAccountID,
			Now:             now,
			ProfileCoarse:   true,
			ProfileCentre:   q.Centre,
		}
		switch cl.view {
		case FeedViewActive, FeedViewDisputed:
			page.Rows = append(page.Rows, liveProfileRow(in, q.Centre))
		case FeedViewResolved, FeedViewPast:
			page.Rows = append(page.Rows, finishedProfileRow(in, areas[c.report.ID]))
		default:
			// FeedViewNone, and anything a later view adds, is never listed.
		}
	}
	return page, nil
}

// liveProfileRow builds a live row: the exact point and the words as on the map.
func liveProfileRow(in viewInput, centre *GeoPoint) ProfileReportRow {
	v := baseView(in)
	row := ProfileReportRow{View: v, Live: true}
	if centre != nil {
		d := haversineKm(centre.Lat, centre.Lon, v.Latitude, v.Longitude)
		row.View.DistanceKm = &d
		row.DistanceText = DistanceWords(d)
	}
	switch {
	case v.View == FeedViewDisputed:
		row.Status, row.StatusWord = ProfileStatusDisputed, ProfileStatusWordDisputed
		row.DisputedLine = DisputedMayNotBeTrueLine
	case v.ConfirmedByPeople:
		row.Status, row.StatusWord = ProfileStatusOngoing, ProfileStatusWordOngoing
	default:
		row.Status, row.StatusWord = ProfileStatusNotConfirmed, ProfileStatusWordNotConfirmed
	}
	return row
}

// finishedProfileRow builds a finished row through buildFinishedView, the one
// place every finished view leaves the service (T-31-25), then strips the id and
// every time. The end date is read before the times are cleared.
func finishedProfileRow(in viewInput, areaName string) ProfileReportRow {
	endedAt := in.Report.ExpiresAt
	status, word := ProfileStatusExpired, ProfileStatusWordExpired
	if in.Class.view == FeedViewResolved {
		endedAt = in.Class.closure.At
		status, word = ProfileStatusResolved, ProfileStatusWordResolved
	}
	v := buildFinishedView(in)

	row := ProfileReportRow{
		Status:        status,
		StatusWord:    word,
		FinishedTitle: CategoryWord(v.Category) + ", " + SeverityWord(v.Severity),
		EndedOn:       endedAt.In(profileDateZone).Format("2 January 2006"),
	}
	if areaName != "" {
		if area := AreaWords(areaName); area != "" {
			row.AreaLine = "Near " + area
		}
	}
	if v.DistanceKm != nil {
		n := int(math.Round(*v.DistanceKm))
		if n < 1 {
			n = 1
		}
		row.DistanceText = "About " + strconv.Itoa(n) + " km"
	}

	// A finished row must never be one call away from Focus, which does not blur
	// a non Rescue report, so it carries no id. Its description is already empty.
	// Its creation and expiry times and the closure time stay on the server: a
	// time would let a reader match the row to the anonymous Past row at its exact
	// point (T-09-16, D-20).
	v.Report.ID = 0
	v.Report.CreatedAt = time.Time{}
	v.Report.ExpiresAt = time.Time{}
	v.WrittenAt = nil
	if v.History != nil {
		h := *v.History
		h.At = time.Time{}
		h.LifetimeText = ""
		v.History = &h
	}
	row.View = v
	return row
}
