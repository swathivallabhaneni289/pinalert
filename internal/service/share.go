package service

// share.go is the service half of Share (COORD-08, Phase 8 D-04 to D-06).
//
// A shared link names a report by a random 22 character token made on the
// first Share tap, never by the numeric id, so nobody without an account can
// walk through reports one id at a time (research Pitfall 1). EnsureLink is
// the authenticated call: it makes the token once, looks up the coarse area
// name once, and returns the title and the fixed line. PublicStatus is the read
// the no account page renders: it takes a token and answers one of six
// states, and it never calls the area lookup, so the public route can never
// spend the geocoder budget.
//
// Nothing here reads or returns the reporter's words, a coordinate, a name or a
// reporter tag. The status words are the trust block's own sentences, taken
// from the ReportView the app already builds, never recounted.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// ShareTokenLen is the length of a share token: 16 random bytes as unpadded
// base64url.
const ShareTokenLen = 22

// ShareQuerier is the store surface of ShareService: the two area name writes
// the shared lookup makes plus the three share queries.
type ShareQuerier interface {
	AreaNameStore
	SetReportShareToken(ctx context.Context, arg sqlcgen.SetReportShareTokenParams) (*string, error)
	ReportShareFacts(ctx context.Context, reportID int64) (sqlcgen.ReportShareFactsRow, error)
	ReportByShareToken(ctx context.Context, token string) (sqlcgen.ReportByShareTokenRow, error)
}

var _ ShareQuerier = (*sqlcgen.Queries)(nil)

// ShareService makes share links and reads their public status.
type ShareService struct {
	q       ShareQuerier
	reports *ReportService
	namer   AreaNamer
	now     func() time.Time
	token   func() (string, error)
}

// ShareOption configures a ShareService.
type ShareOption func(*ShareService)

// WithShareAreaNamer sets the reverse area lookup. Without one a share title
// has no area, and nothing is looked up.
func WithShareAreaNamer(n AreaNamer) ShareOption {
	return func(s *ShareService) { s.namer = n }
}

// WithShareClock overrides the clock the attempt record uses. Tests only.
func WithShareClock(fn func() time.Time) ShareOption {
	return func(s *ShareService) { s.now = fn }
}

// WithShareTokenSource overrides where tokens come from. Tests only.
func WithShareTokenSource(fn func() (string, error)) ShareOption {
	return func(s *ShareService) { s.token = fn }
}

// NewShareService stores its parts. reports supplies Focus, the one read that
// decides whether a report may be shown at all.
func NewShareService(q ShareQuerier, reports *ReportService, opts ...ShareOption) *ShareService {
	s := &ShareService{q: q, reports: reports, now: time.Now, token: NewShareToken}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewShareToken makes a share token: 16 bytes from crypto/rand, written as
// unpadded base64url, which is 22 characters. It never uses a seeded or
// predictable source (T-08-06).
func NewShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ValidShareToken reports whether s has the shape of a share token: exactly 22
// characters, each a letter, a digit, an underscore or a hyphen. It is the same
// rule as the CHECK constraint on the column, so a value that fails it can
// never name a report and needs no query.
func ValidShareToken(s string) bool {
	if len(s) != ShareTokenLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// ShareLink is what one Share tap returns: the token that makes the link, the
// title and the fixed line. The absolute address is built by the handler from
// configuration, never here and never from a Host header.
type ShareLink struct {
	Token string
	Title string
	Line  string
}

// EnsureLink makes, or returns, the share link of one report. The order is part
// of the specification:
//
//  1. Focus with the viewer: a withheld, unknown or too old report answers
//     ErrReportNotFound before any token is written.
//  2. ReportShareFacts. When the report has no token, make one and write it
//     with a WHERE share_token IS NULL guard; no row back means another request
//     won, so its token is re-read and used, and this call leaves the area
//     lookup to the winner.
//  3. In the call that made the token, when the stored area name is empty, the
//     lookup was never tried and a namer is set, LookUpReportAreaName runs (it
//     records the attempt first), so the lookup happens at most once per report.
//  4. The title is the category and, when known, the coarse area.
func (s *ShareService) EnsureLink(ctx context.Context, reportID, viewerAccountID int64) (ShareLink, error) {
	now := s.now().UTC()
	focus, err := s.reports.Focus(ctx, FocusQuery{ReportID: reportID, ViewerAccountID: viewerAccountID})
	if err != nil {
		return ShareLink{}, err
	}
	if focus.Status != FocusOK {
		return ShareLink{}, ErrReportNotFound
	}
	facts, err := s.q.ReportShareFacts(ctx, reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShareLink{}, ErrReportNotFound
	}
	if err != nil {
		return ShareLink{}, err
	}

	// minted is true only for the one call whose write made the token. Only that
	// call may look the area up: a call that lost the race, and a late call that
	// finds the token already there, would otherwise look it up again before the
	// winner has recorded its attempt (T-08-08, at most once per report).
	minted := false
	if facts.ShareToken == nil || *facts.ShareToken == "" {
		token, err := s.token()
		if err != nil {
			return ShareLink{}, err
		}
		stored, err := s.q.SetReportShareToken(ctx, sqlcgen.SetReportShareTokenParams{Token: token, ReportID: reportID})
		switch {
		case err == nil && stored != nil:
			facts.ShareToken = stored
			minted = true
		case err == nil || errors.Is(err, pgx.ErrNoRows):
			facts, err = s.q.ReportShareFacts(ctx, reportID)
			if err != nil {
				return ShareLink{}, err
			}
			if facts.ShareToken == nil || *facts.ShareToken == "" {
				return ShareLink{}, errors.New("service: share token missing after a lost write")
			}
		default:
			return ShareLink{}, err
		}
	}

	area := ""
	if facts.AreaName != nil {
		area = *facts.AreaName
	}
	if minted && area == "" && !facts.AreaLookupTried && s.namer != nil {
		area = LookUpReportAreaName(ctx, s.q, s.namer, facts.ID, facts.Latitude, facts.Longitude, now)
	}
	return ShareLink{
		Token: *facts.ShareToken,
		Title: ShareTitle(Category(facts.Category), area),
		Line:  ShareFixedLine,
	}, nil
}

// PublicState is the one word the public page branches on.
type PublicState string

const (
	PublicActive      PublicState = "active"
	PublicDisputed    PublicState = "disputed"
	PublicResolved    PublicState = "resolved"
	PublicEnded       PublicState = "ended"
	PublicRemoved     PublicState = "removed"
	PublicUnavailable PublicState = "unavailable"
)

// PublicStatus is everything the no account page may show. It has no field for
// the reporter's words, a coordinate, a distance, a name or a comment. Chip and
// Sentence are empty for the removed and unavailable states, and so are the
// severity, the happening word, the times and the ended label.
type PublicStatus struct {
	State        PublicState
	Title        string
	Chip         string
	Sentence     string
	Severity     Severity
	SeverityWord string
	Happening    string
	PostedAt     time.Time
	// EndedLabel is "Resolved" or "Ended" and EndedAt when it happened, for the
	// two finished states only.
	EndedLabel string
	EndedAt    *time.Time
}

func unavailableStatus() PublicStatus {
	return PublicStatus{State: PublicUnavailable, Title: PublicUnavailableTitle}
}

// PublicStatus reads a share token into one of the six public states. The order
// is part of the specification:
//
//  1. A token with the wrong shape is unavailable and runs no query.
//  2. ReportByShareToken: no row is unavailable. A moderator removal is the
//     one moderation state that is named (removed); a held report, an unknown
//     token and a report past the history window all read unavailable.
//  3. Focus with viewer 0, so the page shows what a stranger would see; any
//     status but ok is unavailable.
//  4. The view maps to the state table, reusing the sentences the view already
//     carries. The title comes from the stored area name only: this method never
//     calls the area lookup.
func (s *ShareService) PublicStatus(ctx context.Context, token string) (PublicStatus, error) {
	if !ValidShareToken(token) {
		return unavailableStatus(), nil
	}
	named, err := s.q.ReportByShareToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return unavailableStatus(), nil
	}
	if err != nil {
		return PublicStatus{}, err
	}
	if ModerationState(named.ModerationState) == ModerationRemoved {
		return PublicStatus{State: PublicRemoved, Title: PublicRemovedTitle}, nil
	}
	focus, err := s.reports.Focus(ctx, FocusQuery{ReportID: named.ID})
	if err != nil {
		return PublicStatus{}, err
	}
	if focus.Status != FocusOK || focus.View == nil {
		return unavailableStatus(), nil
	}
	facts, err := s.q.ReportShareFacts(ctx, named.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return unavailableStatus(), nil
	}
	if err != nil {
		return PublicStatus{}, err
	}

	view := focus.View
	area := ""
	if facts.AreaName != nil {
		area = *facts.AreaName
	}
	severity := Severity(facts.Severity)
	out := PublicStatus{
		Title:        ShareTitle(Category(facts.Category), area),
		Severity:     severity,
		SeverityWord: SeverityWord(severity),
		PostedAt:     view.CreatedAt,
	}
	if severity == SeverityCritical && facts.Happening != nil {
		out.Happening = HappeningWord(Happening(*facts.Happening))
	}

	switch view.View {
	case FeedViewActive:
		out.State = PublicActive
		out.Chip = PublicChipActive
		out.Sentence = UnconfirmedLine
		if view.Trust.ConfirmedPlaces > 0 {
			out.Sentence = view.Trust.ConfirmedSentence
		}
	case FeedViewDisputed:
		out.State = PublicDisputed
		out.Chip = PublicChipDisputed
		out.Sentence = view.Trust.DisputedSentence
	case FeedViewResolved:
		out.State = PublicResolved
		out.Chip = PublicChipResolved
		out.EndedLabel = PublicChipResolved
		by := ClosedBy("")
		if view.History != nil {
			by = view.History.ClosedBy
			at := view.History.At
			out.EndedAt = &at
		}
		out.Sentence = publicResolvedSentence(by)
	case FeedViewPast:
		out.State = PublicEnded
		out.Chip = PublicChipEnded
		out.Sentence = PublicEndedSentence
		out.EndedLabel = PublicChipEnded
		if view.History != nil {
			at := view.History.At
			out.EndedAt = &at
		}
	default:
		return unavailableStatus(), nil
	}
	return out, nil
}
