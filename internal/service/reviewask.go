// reviewask.go is the author's side of quick 261118-rz6 (owner 2026-11-18): the
// author of a removed report sees why it was removed and may ask a moderator to
// review it, up to ReviewAskLimit times per report, one at a time. The
// moderator's side (the Review asks list, Restore and Keep removed) is in
// moderation.go.

package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// ReviewAskLimit is the most asks for review one report can have in total,
// answered or not. The count carries over a Restore and a later Remove.
const ReviewAskLimit = 3

// ReviewAskNoteMaxChars is the longest note, counted in characters (Unicode
// code points) after trimming, as the box's maxlength counts them.
const ReviewAskNoteMaxChars = 200

// The plain lines of the author's API. They stay inside the closed character
// set the server copy test checks.
const (
	// ReviewAskOpenMessage answers an ask while one is open.
	ReviewAskOpenMessage = "You already asked. Wait for the answer."
	// ReviewAskNoneLeftMessage answers an ask after ReviewAskLimit asks.
	ReviewAskNoneLeftMessage = "No asks left."
	// ReviewAskNoteTooLongMessage answers a note over ReviewAskNoteMaxChars.
	ReviewAskNoteTooLongMessage = "Keep the note to 200 characters or fewer."
)

// ErrReviewAskOpen and ErrReviewAskNoneLeft are the two 409 answers of Ask.
var (
	ErrReviewAskOpen     = errors.New("service: an ask for review is already open")
	ErrReviewAskNoneLeft = errors.New("service: no asks for review left")
)

// ReviewAskState is the closed state of the author's box.
type ReviewAskState string

// The three states of the box.
const (
	// ReviewAskCanAsk: the box offers the note and Ask for review.
	ReviewAskCanAsk ReviewAskState = "can_ask"
	// ReviewAskAsked: an ask is open, the box says Asked.
	ReviewAskAsked ReviewAskState = "asked"
	// ReviewAskNoneLeft: three asks were used and answered, the box says No
	// asks left.
	ReviewAskNoneLeft ReviewAskState = "no_asks_left"
)

// ReviewAskBox is what the author's box shows: the Why line (empty when the
// removal has no recorded reason) and the state. It never carries the
// report's words, flag counts, who flagged, places or the moderator.
type ReviewAskBox struct {
	Why   string
	State ReviewAskState
}

// ReviewAskQuerier is the slice of the generated queries the author's side
// uses. LockReportModeration is the report row lock the flag hide, Keep,
// Remove, Restore and Keep removed take.
type ReviewAskQuerier interface {
	LockReportModeration(ctx context.Context, reportID int64) (sqlcgen.LockReportModerationRow, error)
	ReviewAskFacts(ctx context.Context, reportID int64) (sqlcgen.ReviewAskFactsRow, error)
	InsertReviewAsk(ctx context.Context, arg sqlcgen.InsertReviewAskParams) error
}

var _ ReviewAskQuerier = (*sqlcgen.Queries)(nil)

// ReviewAskTxFunc runs fn inside one database transaction, like
// ModerationTxFunc.
type ReviewAskTxFunc func(ctx context.Context, fn func(q ReviewAskQuerier) error) error

// PgxReviewAskTx builds the production ReviewAskTxFunc over a pool.
func PgxReviewAskTx(db TxBeginner) ReviewAskTxFunc {
	return func(ctx context.Context, fn func(q ReviewAskQuerier) error) error {
		return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			return fn(sqlcgen.New(tx))
		})
	}
}

// ReviewAskService serves the author's box. Steps:
//  1. State reads ReviewAskFacts for the report. Anything but the report's own
//     author asking about it while it is removed returns ErrReportNotFound, so
//     a stranger, a shown report, a held report and a report under review all
//     look the same as no report (404 with nothing revealed).
//  2. The state is asked while an ask is open, no asks left once
//     ReviewAskLimit asks were made, and can ask otherwise.
//  3. Ask checks the note first (trimmed, at most ReviewAskNoteMaxChars), then
//     in one transaction locks the report row, reads the facts again under the
//     lock, refuses as in step 1, answers ErrReviewAskOpen or
//     ErrReviewAskNoneLeft, and otherwise stores the ask. The lock makes two
//     asks at once queue, so the second sees the first and the cap of 3 holds.
//
// The note is plain text for moderators only: it is never sent to the text
// check, never logged and never in a public answer.
//
// Without WithReviewAskTx the same steps run on the plain querier.
type ReviewAskService struct {
	q  ReviewAskQuerier
	tx ReviewAskTxFunc
}

// NewReviewAskService builds the service over q. A nil tx runs Ask on the
// plain querier with no transaction.
func NewReviewAskService(q ReviewAskQuerier, tx ReviewAskTxFunc) *ReviewAskService {
	return &ReviewAskService{q: q, tx: tx}
}

// reviewAskBoxOf decides the box from the facts, or ErrReportNotFound.
func reviewAskBoxOf(f sqlcgen.ReviewAskFactsRow, accountID int64) (ReviewAskBox, error) {
	if f.AuthorAccountID == nil || *f.AuthorAccountID != accountID || f.AuthorDeleted ||
		ModerationState(f.ModerationState) != ModerationRemoved {
		return ReviewAskBox{}, ErrReportNotFound
	}
	box := ReviewAskBox{Why: RemovalWhyLine(RemovalReason(derefString(f.RemovedReason)))}
	switch {
	case f.AskOpen:
		box.State = ReviewAskAsked
	case f.AskTotal >= ReviewAskLimit:
		box.State = ReviewAskNoneLeft
	default:
		box.State = ReviewAskCanAsk
	}
	return box, nil
}

// State is the author's box for one report. See the steps on the type.
func (s *ReviewAskService) State(ctx context.Context, accountID, reportID int64) (ReviewAskBox, error) {
	f, err := s.q.ReviewAskFacts(ctx, reportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReviewAskBox{}, ErrReportNotFound
		}
		return ReviewAskBox{}, err
	}
	return reviewAskBoxOf(f, accountID)
}

// NormalizeReviewAskNote trims the note and checks its length. An empty note
// is no note (nil).
func NormalizeReviewAskNote(raw string) (*string, error) {
	note := strings.TrimSpace(raw)
	if utf8.RuneCountInString(note) > ReviewAskNoteMaxChars {
		return nil, ValidationError{Field: "note", Message: ReviewAskNoteTooLongMessage}
	}
	if note == "" {
		return nil, nil
	}
	return &note, nil
}

// Ask stores one ask for review and returns the box as it is now (asked). See
// the steps on the type.
func (s *ReviewAskService) Ask(ctx context.Context, accountID, reportID int64, rawNote string) (ReviewAskBox, error) {
	note, err := NormalizeReviewAskNote(rawNote)
	if err != nil {
		return ReviewAskBox{}, err
	}
	run := func(q ReviewAskQuerier) error {
		if _, err := q.LockReportModeration(ctx, reportID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrReportNotFound
			}
			return err
		}
		f, err := q.ReviewAskFacts(ctx, reportID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrReportNotFound
			}
			return err
		}
		box, err := reviewAskBoxOf(f, accountID)
		if err != nil {
			return err
		}
		switch box.State {
		case ReviewAskAsked:
			return ErrReviewAskOpen
		case ReviewAskNoneLeft:
			return ErrReviewAskNoneLeft
		}
		return q.InsertReviewAsk(ctx, sqlcgen.InsertReviewAskParams{ReportID: reportID, AccountID: accountID, Note: note})
	}
	if s.tx == nil {
		err = run(s.q)
	} else {
		err = s.tx(ctx, run)
	}
	if err != nil {
		return ReviewAskBox{}, err
	}
	return s.State(ctx, accountID, reportID)
}
