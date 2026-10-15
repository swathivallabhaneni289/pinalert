package service

// flag.go is the server half of the Flag control (D-09). A signed in person
// can flag someone else's report once, with one of five reasons. A flag only
// queues the report for the moderator screen: it never hides or changes the
// report, which is why FlagQuerier can read one report row and insert one flag
// row and nothing else. A report nobody may see (missing or withheld) reads as
// not found, and the flagger is never exposed to anyone.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

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
// enumerates exactly this list.
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

// The four server messages of the flag route. The first, third and fourth are
// the dialog's own Validation, Report gone and Already flagged lines.
const (
	FlagReasonMessage    = "Choose a reason to continue."
	FlagOwnReportMessage = "You can't flag your own report."
	FlagNotFoundMessage  = "This report is no longer available."
	FlagAlreadyMessage   = "You already flagged this report. Moderators will see it."
)

var (
	// ErrFlagOwnReport is returned when the flagger is the reporter.
	ErrFlagOwnReport = errors.New("service: cannot flag your own report")
	// ErrAlreadyFlagged is returned when this account already flagged this
	// report, even if a moderator dismissed that flag.
	ErrAlreadyFlagged = errors.New("service: report already flagged by this account")
)

// FlagQuerier is the narrow store surface of FlagService. It has exactly two
// methods on purpose: a flag can read the one vote context row and insert one
// flag row, so it has no way to write a reports column or feed the resolver.
type FlagQuerier interface {
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
	InsertReportFlag(ctx context.Context, arg sqlcgen.InsertReportFlagParams) (int64, error)
}

// FlagService applies the flag rules.
type FlagService struct {
	q FlagQuerier
}

// NewFlagService stores the querier. It never dereferences it, so a router
// built with a nil sessions store still constructs.
func NewFlagService(q FlagQuerier) *FlagService {
	return &FlagService{q: q}
}

// FlagInput is one flag attempt. AccountID is the verified account of the
// caller and nothing else.
type FlagInput struct {
	ReportID  int64
	AccountID int64
	Reason    FlagReason
}

// Flag stores one flag. The order of the checks is part of the specification:
//
//  1. The reason must be one of the five (ValidationError on field reason).
//  2. The report is read once; a missing report is ErrReportNotFound.
//  3. A withheld report is ErrReportNotFound (R-01, R-04).
//  4. The caller's own report is ErrFlagOwnReport.
//  5. The flag is inserted; no row back means ErrAlreadyFlagged.
//
// Withheld comes before ownership so an author's own held or removed report
// looks exactly like a missing one and never reveals itself through a 403.
// There is deliberately no expiry step: a Past report can still be flagged,
// and 409 is reserved for already flagged because the dialog reads 409 as
// Flagged. Flag never calls the resolver and builds no VoteTally.
func (s *FlagService) Flag(ctx context.Context, in FlagInput) error {
	if !in.Reason.Valid() {
		return ValidationError{Field: "reason", Message: FlagReasonMessage}
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

	_, err = s.q.InsertReportFlag(ctx, sqlcgen.InsertReportFlagParams{
		ReportID:  in.ReportID,
		AccountID: in.AccountID,
		Reason:    string(in.Reason),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAlreadyFlagged
		}
		return fmt.Errorf("service: Flag: storing flag on report %d: %w", in.ReportID, err)
	}
	return nil
}
