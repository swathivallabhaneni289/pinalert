package service

// flag.go is the server half of the Flag control (D-09). A signed in person
// can flag someone else's report once, with one or more of five reasons and an
// optional note for the moderators (plan 04-31, owner 2026-10-22). A flag
// queues the report for the moderator screen, and on its own it never hides or
// changes the report. Amended 2026-11-17 (quick 261117-vvb, owner): once the
// open flags of a shown report come from FlagHideThreshold different place
// cells, the report is hidden for review until a moderator keeps or removes
// it, and its author is told in the bell. Amended 2026-11-18 (quick
// 261118-saj, owner): a serious report is never hidden that way; it stays shown
// with the Being checked mark and sends no phone notification until a
// moderator looks (writeFlag). The flagger's position, when the
// browser knows one, becomes a place cell here and only the cell is stored; it
// is never returned, shown or logged. A report nobody may see (missing or
// withheld) reads as not found, and the flagger is never exposed to anyone.
// The note goes through the abuse screen of the report description's text
// check (ScreenAbuse, the screener half of CheckText) and is shown to
// moderators only. It skips the gibberish and minimum length rule on purpose:
// a one word note such as "fake" or "duplicate" is a good flag note (owner,
// 2026-10-23).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// FlagHideThreshold is how many different place cells the open flags of a
// shown report must come from before it is hidden for review (quick
// 261117-vvb). One flag per account per report makes that many cells that many
// accounts too.
const FlagHideThreshold = 3

// FlagReason is one of the five reasons a reader can give when flagging.
type FlagReason string

const (
	FlagSpam      FlagReason = "spam"
	FlagAbusive   FlagReason = "abusive"
	FlagFalse     FlagReason = "false"
	FlagDangerous FlagReason = "dangerous"
	FlagOther     FlagReason = "other"
)

// FlagReasons is the closed, ordered list of reasons. The API reference
// enumerates exactly this list, and a stored set of reasons is kept in this
// order.
var FlagReasons = []FlagReason{FlagSpam, FlagAbusive, FlagFalse, FlagDangerous, FlagOther}

// Valid reports whether r is one of the five reasons. Comparison is exact.
func (r FlagReason) Valid() bool {
	for _, x := range FlagReasons {
		if x == r {
			return true
		}
	}
	return false
}

// FlagReasonLabel is the one label of a reason, as the dialog and the
// moderator screen show it. It is the empty string for anything else.
func FlagReasonLabel(r FlagReason) string {
	switch r {
	case FlagSpam:
		return "Spam or gibberish"
	case FlagAbusive:
		return "Abusive or hateful"
	case FlagFalse:
		return "False or a hoax"
	case FlagDangerous:
		return "Dangerous or illegal"
	case FlagOther:
		return "Something else"
	}
	return ""
}

// FlagNoteMaxChars is the longest note, counted in characters (Unicode code
// points) after trimming, the way a report description is counted.
const FlagNoteMaxChars = 300

// The server messages of the flag route. The first, third and fourth are the
// dialog's own Validation, Report gone and Already flagged lines.
const (
	FlagReasonMessage    = "Choose a reason to continue."
	FlagOwnReportMessage = "You can't flag your own report."
	FlagNotFoundMessage  = "This report is no longer available."
	FlagAlreadyMessage   = "You already flagged this report. Moderators will see it."
	// FlagReasonsBadMessage answers a reasons list with an unknown or a
	// repeated reason, which the dialog never sends.
	FlagReasonsBadMessage = "Choose each reason from the list once."
	// FlagNoteTooLongMessage answers a note over FlagNoteMaxChars.
	FlagNoteTooLongMessage = "Keep the note to 300 characters or fewer."
	// FlagPositionMessage answers a position outside the map, which the dialog
	// never sends (quick 261117-vvb).
	FlagPositionMessage = "Your location could not be read. Try again."
)

var (
	// ErrFlagOwnReport is returned when the flagger is the reporter.
	ErrFlagOwnReport = errors.New("service: cannot flag your own report")
	// ErrAlreadyFlagged is returned when this account already flagged this
	// report, even if a moderator dismissed that flag.
	ErrAlreadyFlagged = errors.New("service: report already flagged by this account")
	// ErrFlagNoteRejected is the sentinel a refused note wraps. Nothing was
	// stored.
	ErrFlagNoteRejected = errors.New("service: flag note rejected by the text check")
)

// FlagNoteRejectedError is returned by Flag when the text check refused the
// note. RejectMessage(Reason) is the message the dialog shows.
type FlagNoteRejectedError struct {
	Reason RejectReason
}

func (e FlagNoteRejectedError) Error() string {
	return fmt.Sprintf("service: flag note rejected (%s)", e.Reason)
}

// Unwrap lets errors.Is(err, ErrFlagNoteRejected) match.
func (e FlagNoteRejectedError) Unwrap() error {
	return ErrFlagNoteRejected
}

// FlagQuerier is the narrow store surface of FlagService. It has exactly two
// methods on purpose: a flag can read the one vote context row and insert one
// flag row, so through it a flag has no way to write a reports column or feed
// the resolver. The review hide of quick 261117-vvb goes through FlagHideQuerier
// instead, inside its own transaction.
type FlagQuerier interface {
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
	InsertReportFlag(ctx context.Context, arg sqlcgen.InsertReportFlagParams) (int64, error)
}

// FlagHideQuerier is the store surface of the flag write with the review hide
// (quick 261117-vvb): lock the report, insert the flag, hide the report when
// the threshold is met, and tell its author. Its only reports writes are
// HideReportForReview, which moves a clear report to review and nothing else,
// and, since quick 261118-saj, MarkReportForFlagCheck, which only sets the
// Being checked mark of a serious report and never hides it.
type FlagHideQuerier interface {
	LockReportModeration(ctx context.Context, reportID int64) (sqlcgen.LockReportModerationRow, error)
	InsertReportFlag(ctx context.Context, arg sqlcgen.InsertReportFlagParams) (int64, error)
	HideReportForReview(ctx context.Context, arg sqlcgen.HideReportForReviewParams) (int64, error)
	MarkReportForFlagCheck(ctx context.Context, arg sqlcgen.MarkReportForFlagCheckParams) (int64, error)
	InsertOwnReportUpdate(ctx context.Context, arg sqlcgen.InsertOwnReportUpdateParams) (int64, error)
}

var _ FlagHideQuerier = (*sqlcgen.Queries)(nil)

// FlagTxFunc runs fn inside one database transaction with a querier bound to
// it, commits when fn returns nil and rolls back otherwise.
type FlagTxFunc func(ctx context.Context, fn func(q FlagHideQuerier) error) error

// PgxFlagTx builds the production FlagTxFunc over a pool.
func PgxFlagTx(db TxBeginner) FlagTxFunc {
	return func(ctx context.Context, fn func(q FlagHideQuerier) error) error {
		return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			return fn(sqlcgen.New(tx))
		})
	}
}

// FlagService applies the flag rules.
type FlagService struct {
	q        FlagQuerier
	tx       FlagTxFunc
	screener TextScreener
}

// FlagOption configures a FlagService.
type FlagOption func(*FlagService)

// WithFlagScreener sets the text screener the note goes through. It is the
// same screener the report description goes through. Without one, the note
// passes the screen, as a report description does without one.
func WithFlagScreener(s TextScreener) FlagOption {
	return func(fs *FlagService) {
		fs.screener = s
	}
}

// WithFlagTx runs the flag write and the review hide inside one transaction
// (PgxFlagTx in production). A nil tx leaves the plain querier.
func WithFlagTx(tx FlagTxFunc) FlagOption {
	return func(fs *FlagService) {
		fs.tx = tx
	}
}

// NewFlagService stores the querier. It never dereferences it, so a router
// built with a nil sessions store still constructs.
func NewFlagService(q FlagQuerier, opts ...FlagOption) *FlagService {
	s := &FlagService{q: q}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// FlagPosition is where the flagger is, as the browser last knew it. It is
// turned into a place cell and never stored or returned itself.
type FlagPosition struct {
	Latitude  float64
	Longitude float64
}

// FlagInput is one flag attempt. AccountID is the verified account of the
// caller and nothing else. Reasons may be in any order; Note is the raw note
// and may be empty. Position is nil when the browser knew no position, which
// is allowed: such a flag reaches the moderators and never counts toward a
// review hide.
type FlagInput struct {
	ReportID  int64
	AccountID int64
	Reasons   []FlagReason
	Note      string
	Position  *FlagPosition
}

// ValidateFlagInput checks the shape of a flag and returns the reasons in the
// FlagReasons order with the trimmed note. The rules: at least one reason
// (FlagReasonMessage), every reason one of the five and none twice
// (FlagReasonsBadMessage), a note of at most FlagNoteMaxChars characters
// after trimming (FlagNoteTooLongMessage), and a position, when there is one,
// with a latitude from -90 to 90 and a longitude from -180 to 180, the vote
// route's bounds (FlagPositionMessage). An empty trimmed note means none.
func ValidateFlagInput(in FlagInput) ([]FlagReason, string, error) {
	if len(in.Reasons) == 0 {
		return nil, "", ValidationError{Field: "reasons", Message: FlagReasonMessage}
	}
	seen := make(map[FlagReason]bool, len(in.Reasons))
	for _, r := range in.Reasons {
		if !r.Valid() || seen[r] {
			return nil, "", ValidationError{Field: "reasons", Message: FlagReasonsBadMessage}
		}
		seen[r] = true
	}
	ordered := make([]FlagReason, 0, len(in.Reasons))
	for _, r := range FlagReasons {
		if seen[r] {
			ordered = append(ordered, r)
		}
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > FlagNoteMaxChars {
		return nil, "", ValidationError{Field: "note", Message: FlagNoteTooLongMessage}
	}
	if p := in.Position; p != nil {
		// Written as negated within tests so a NaN is refused too.
		if !(p.Latitude >= -90 && p.Latitude <= 90) {
			return nil, "", ValidationError{Field: "latitude", Message: FlagPositionMessage}
		}
		if !(p.Longitude >= -180 && p.Longitude <= 180) {
			return nil, "", ValidationError{Field: "longitude", Message: FlagPositionMessage}
		}
	}
	return ordered, note, nil
}

// flagCell is the place cell of a flag's position at the voter cell precision,
// the precision confirmations count by, or nil with no position.
func flagCell(p *FlagPosition) *string {
	if p == nil {
		return nil
	}
	cell := geohash.EncodeWithPrecision(p.Latitude, p.Longitude, voterGeohashPrecision)
	return &cell
}

// Flag stores one flag. The order of the checks is part of the specification:
//
//  1. ValidateFlagInput: the reasons, the note length and the position
//     (ValidationError).
//  2. The report is read once; a missing report is ErrReportNotFound.
//  3. A withheld report is ErrReportNotFound (R-01, R-04).
//  4. The caller's own report is ErrFlagOwnReport.
//  5. A non empty note goes through ScreenAbuse; a refused note is
//     FlagNoteRejectedError and nothing is stored. A borderline or unfinished
//     check still stores the note: only moderators read it, and a provider
//     outage must not stop a flag, which is a safety action.
//  6. The flag is written (writeFlag); no row back means ErrAlreadyFlagged.
//
// Withheld comes before ownership so an author's own held or removed report
// looks exactly like a missing one and never reveals itself through a 403.
// The note is checked after the report checks so a 404 or 403 never costs a
// screener call, and before the write so no lock is held across the screener
// call. There is deliberately no expiry step: a Past report can still be
// flagged, and 409 is reserved for already flagged because the dialog reads
// 409 as Flagged. Flag never calls the resolver and builds no VoteTally.
func (s *FlagService) Flag(ctx context.Context, in FlagInput) error {
	reasons, note, err := ValidateFlagInput(in)
	if err != nil {
		return err
	}

	rc, err := s.q.ReportVoteContext(ctx, in.ReportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReportNotFound
		}
		return fmt.Errorf("service: Flag: reading report %d: %w", in.ReportID, err)
	}

	meta := ReportMeta{
		Severity:   Severity(rc.Severity),
		Category:   Category(rc.Category),
		Moderation: ModerationState(rc.ModerationState),
	}
	if meta.Withheld() {
		return ErrReportNotFound
	}

	// A report with no reporter account (made before accounts existed) belongs
	// to nobody, so anyone may flag it.
	if rc.ReporterAccountID != nil && *rc.ReporterAccountID == in.AccountID {
		return ErrFlagOwnReport
	}

	var notePtr *string
	if note != "" {
		check := ScreenAbuse(ctx, s.screener, note)
		if check.Outcome == TextRejected {
			return FlagNoteRejectedError{Reason: check.Reason}
		}
		notePtr = &note
	}

	slugs := make([]string, len(reasons))
	for i, r := range reasons {
		slugs[i] = string(r)
	}
	arg := sqlcgen.InsertReportFlagParams{
		ReportID:    in.ReportID,
		AccountID:   in.AccountID,
		Reasons:     slugs,
		Note:        notePtr,
		FlaggerCell: flagCell(in.Position),
	}

	switch {
	case s.tx != nil:
		err = s.tx(ctx, func(q FlagHideQuerier) error { return writeFlag(ctx, q, arg) })
	default:
		// Without a transaction (router harnesses that build no pool) the same
		// steps run on the plain querier when it can take them, and a querier
		// that cannot (the unit test fakes) only stores the flag.
		if hq, ok := s.q.(FlagHideQuerier); ok {
			err = writeFlag(ctx, hq, arg)
		} else {
			_, err = s.q.InsertReportFlag(ctx, arg)
			if errors.Is(err, pgx.ErrNoRows) {
				err = ErrAlreadyFlagged
			}
		}
	}
	if err != nil {
		if errors.Is(err, ErrAlreadyFlagged) || errors.Is(err, ErrReportNotFound) {
			return err
		}
		return fmt.Errorf("service: Flag: storing flag on report %d: %w", in.ReportID, err)
	}
	return nil
}

// writeFlag is the write of one flag with the review hide (quick 261117-vvb),
// in this order, inside the caller's transaction:
//
//  1. LockReportModeration takes the report's row lock, so two flags on one
//     report queue here. Under READ COMMITTED every later statement of the
//     second flag then sees the first one's committed flag, so concurrent
//     flags can neither skip the hide nor make it twice. A report that is gone
//     or became withheld since Flag read it is ErrReportNotFound.
//  2. InsertReportFlag; no row back is ErrAlreadyFlagged.
//  3. HideReportForReview moves a clear report that no moderator kept after a
//     review hide to review once its open flags come from FlagHideThreshold
//     different cells. It runs after every flag, even one with no cell, which
//     never counts itself, so a report that met the rule while it could not
//     be hidden (a pending serious report the late check then passed) is
//     hidden by the next flag.
//  4. Only when that statement hid the report, its author gets the own_review
//     bell row, so the row is written exactly once.
//
// Quick 261118-saj (owner, 2026-11-18): a serious report (ReportMeta.Serious,
// read from the locked row) is never hidden by flags, as disputes never hide
// it. Step 3 is MarkReportForFlagCheck for it instead: at the same threshold,
// by the same count and under the same lock, it gets the Being checked mark,
// stays shown, sends no phone notification until a moderator looks and comes
// first on the moderator's flagged list. A shown pending serious report gets
// the mark at once. No bell row is written for the mark: the report was never
// hidden.
func writeFlag(ctx context.Context, q FlagHideQuerier, arg sqlcgen.InsertReportFlagParams) error {
	locked, err := q.LockReportModeration(ctx, arg.ReportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReportNotFound
		}
		return err
	}
	meta := ReportMeta{
		Severity:   Severity(locked.Severity),
		Category:   Category(locked.Category),
		Moderation: ModerationState(locked.ModerationState),
	}
	if meta.Withheld() {
		return ErrReportNotFound
	}
	if _, err := q.InsertReportFlag(ctx, arg); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAlreadyFlagged
		}
		return err
	}
	if meta.Serious() {
		_, err := q.MarkReportForFlagCheck(ctx, sqlcgen.MarkReportForFlagCheckParams{
			ReportID:  arg.ReportID,
			Threshold: FlagHideThreshold,
		})
		return err
	}
	hid, err := q.HideReportForReview(ctx, sqlcgen.HideReportForReviewParams{
		ReportID:  arg.ReportID,
		Threshold: FlagHideThreshold,
	})
	if err != nil {
		return err
	}
	if hid != 1 {
		return nil
	}
	_, err = q.InsertOwnReportUpdate(ctx, sqlcgen.InsertOwnReportUpdateParams{
		ReportID: arg.ReportID,
		Kind:     string(UpdateOwnReview),
	})
	return err
}
