package service

// flag.go is the server half of the Flag control (D-09). A signed in person
// can flag someone else's report once, with one or more of five reasons and an
// optional note for the moderators (plan 04-31, owner 2026-10-22). A flag only
// queues the report for the moderator screen: it never hides or changes the
// report, which is why FlagQuerier can read one report row and insert one flag
// row and nothing else. A report nobody may see (missing or withheld) reads as
// not found, and the flagger is never exposed to anyone. The note goes through
// the abuse screen of the report description's text check (ScreenAbuse, the
// screener half of CheckText) and is shown to moderators only. It skips the
// gibberish and minimum length rule on purpose: a one word note such as
// "fake" or "duplicate" is a good flag note (owner, 2026-10-23).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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
// flag row, so it has no way to write a reports column or feed the resolver.
type FlagQuerier interface {
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
	InsertReportFlag(ctx context.Context, arg sqlcgen.InsertReportFlagParams) (int64, error)
}

// FlagService applies the flag rules.
type FlagService struct {
	q        FlagQuerier
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

// NewFlagService stores the querier. It never dereferences it, so a router
// built with a nil sessions store still constructs.
func NewFlagService(q FlagQuerier, opts ...FlagOption) *FlagService {
	s := &FlagService{q: q}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// FlagInput is one flag attempt. AccountID is the verified account of the
// caller and nothing else. Reasons may be in any order; Note is the raw note
// and may be empty.
type FlagInput struct {
	ReportID  int64
	AccountID int64
	Reasons   []FlagReason
	Note      string
}

// ValidateFlagInput checks the shape of a flag and returns the reasons in the
// FlagReasons order with the trimmed note. The rules: at least one reason
// (FlagReasonMessage), every reason one of the five and none twice
// (FlagReasonsBadMessage), and a note of at most FlagNoteMaxChars characters
// after trimming (FlagNoteTooLongMessage). An empty trimmed note means none.
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
	return ordered, note, nil
}

// Flag stores one flag. The order of the checks is part of the specification:
//
//  1. ValidateFlagInput: the reasons and the note length (ValidationError).
//  2. The report is read once; a missing report is ErrReportNotFound.
//  3. A withheld report is ErrReportNotFound (R-01, R-04).
//  4. The caller's own report is ErrFlagOwnReport.
//  5. A non empty note goes through ScreenAbuse; a refused note is
//     FlagNoteRejectedError and nothing is stored. A borderline or unfinished
//     check still stores the note: only moderators read it, and a provider
//     outage must not stop a flag, which is a safety action.
//  6. The flag is inserted; no row back means ErrAlreadyFlagged.
//
// Withheld comes before ownership so an author's own held or removed report
// looks exactly like a missing one and never reveals itself through a 403.
// The note is checked after the report checks so a 404 or 403 never costs a
// screener call. There is deliberately no expiry step: a Past report can still
// be flagged, and 409 is reserved for already flagged because the dialog reads
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
	_, err = s.q.InsertReportFlag(ctx, sqlcgen.InsertReportFlagParams{
		ReportID:  in.ReportID,
		AccountID: in.AccountID,
		Reasons:   slugs,
		Note:      notePtr,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAlreadyFlagged
		}
		return fmt.Errorf("service: Flag: storing flag on report %d: %w", in.ReportID, err)
	}
	return nil
}
