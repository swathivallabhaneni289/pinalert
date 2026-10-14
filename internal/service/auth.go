package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/auth"
	"pinalert/internal/mailer"
	sqlcgen "pinalert/internal/store/sqlc"
)

// VerifyOutcome discriminates the five ways GET /auth/verify (plan 01.1-02)
// can land. The handler passes the value straight into the outcome
// template's discriminator, so these are kept lowercase and stable — never
// renamed once shipped, since a template branch and this constant must stay
// in lockstep.
type VerifyOutcome string

const (
	OutcomeVerified  VerifyOutcome = "verified"
	OutcomeExpired   VerifyOutcome = "expired"
	OutcomeUsed      VerifyOutcome = "used"
	OutcomeMalformed VerifyOutcome = "malformed"
	OutcomeConflict  VerifyOutcome = "conflict"
)

// rawTokenBytes is the decoded length internal/auth.GenerateToken produces
// (tokenBytes there) — a token that doesn't decode to exactly this many
// bytes is malformed by construction, never a database lookup away from
// being valid.
const rawTokenBytes = 32

// ResendCooldown is the minimum time a caller must wait between successive
// verification-email requests for the same address (D-03: the midpoint of
// the 30-60 second range CONTEXT.md leaves to Claude's discretion).
// Exported so handlers and templates share one source of truth rather than
// each hard-coding the number separately.
const ResendCooldown = 45 * time.Second

// AuthQuerier is the subset of the sqlc-generated Queries type AuthService
// needs — small enough to fake in a test without a real Postgres. Later
// plans widen this interface as more of the verification flow lands.
type AuthQuerier interface {
	InsertMagicLinkToken(ctx context.Context, arg sqlcgen.InsertMagicLinkTokenParams) (sqlcgen.InsertMagicLinkTokenRow, error)
	GetMagicLinkTokenByHash(ctx context.Context, tokenHash string) (sqlcgen.GetMagicLinkTokenByHashRow, error)
	ConsumeToken(ctx context.Context, tokenHash string) (string, error)
	InsertAccount(ctx context.Context, email string) error
	GetAccountByEmail(ctx context.Context, email string) (sqlcgen.Account, error)
	GetAccountBySessionID(ctx context.Context, sessionID string) (sqlcgen.GetAccountBySessionIDRow, error)
	BindSessionAccount(ctx context.Context, arg sqlcgen.BindSessionAccountParams) error
	ClaimEmailCooldown(ctx context.Context, arg sqlcgen.ClaimEmailCooldownParams) (time.Time, error)
	ReportsByAccount(ctx context.Context, accountID *int64) ([]sqlcgen.ReportsByAccountRow, error)
	// CurrentVotesForReports backs ActivityForAccount (02-07). Signature
	// copied verbatim from trust.go's VotingQuerier — the same generated
	// method, named a second time by a second consumer-defined interface
	// (idiomatic Go: each interface stays small enough to fake without a
	// real Postgres, deliberately not deduplicated into a shared one).
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	// ReporterHistoryReports gives the Activity page the account's bounded
	// history, so every row carries the account's reporter signal.
	ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error)
	// VotedReportsByAccount gives the Activity page the account's current
	// content vote per report (quick task 261013-wm5). A second consumer
	// defined interface naming one generated method, as the two above do.
	VotedReportsByAccount(ctx context.Context, arg sqlcgen.VotedReportsByAccountParams) ([]sqlcgen.VotedReportsByAccountRow, error)
	// ReporterAccountsForReports gives the Activity page the reporter of each
	// voted report, so its trust carries that reporter's signal and not the
	// viewer's. A second consumer defined interface naming one generated
	// method, with the signature ReportQuerier names.
	ReporterAccountsForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error)
}

// ErrRateLimited is returned by RequestLink when a second verification-
// email request for the same normalised address arrives inside
// ResendCooldown of the last one (D-03/D-04, IDENT-04). It is a distinct
// sentinel — never a ValidationError — so the handler maps it to 429
// (errors.Is), not 400. Declared via errors.New rather than as a typed
// error since no additional data ever needs to travel with it.
var ErrRateLimited = errors.New("service: rate limited")

// AuthService validates a requested email address, mints a magic-link
// token, persists it, and hands the emailed link to a Mailer.
type AuthService struct {
	q       AuthQuerier
	mailer  mailer.Mailer
	baseURL string
	now     func() time.Time
}

// AuthOption configures an AuthService at construction time.
type AuthOption func(*AuthService)

// WithClock overrides the AuthService's clock. Tests use this to advance
// time without real sleeps; production code never needs to call it. The
// seam is introduced here — rather than added later alongside plan
// 01.1-05's cooldown logic — because internal/service/auth_test.go lives in
// the external service_test package (matching report_test.go) and so
// cannot reach an unexported field after the fact; adding it now means that
// later plan changes no constructor signature and breaks no call site.
func WithClock(fn func() time.Time) AuthOption {
	return func(s *AuthService) {
		s.now = fn
	}
}

// NewAuthService constructs an AuthService backed by q and m, minting
// absolute verification links rooted at baseURL. The clock defaults to
// time.Now() unless overridden via WithClock.
func NewAuthService(q AuthQuerier, m mailer.Mailer, baseURL string, opts ...AuthOption) *AuthService {
	s := &AuthService{q: q, mailer: m, baseURL: baseURL, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// RequestLink validates rawEmail, claims D-03/D-04's per-address resend
// cooldown, mints a single-use magic-link token, persists its hash before
// attempting delivery, and hands the raw link — never a code to type back
// in; D-01 chose a link over a passcode, so no passcode field exists
// anywhere in this flow — to the configured Mailer. It returns the
// normalized address (trimmed, lowercased, no display-name form) that was
// actually mailed, so a caller such as the HTTP handler can echo it back
// without duplicating the normalization rule.
//
// The cooldown is claimed in a single atomic statement (ClaimEmailCooldown)
// between email normalisation and token generation — before anything else
// is persisted or sent. A refused claim writes nothing at all, so it can
// never extend or reset the window that refused it (DEC-J, DEC-X, T-01-93,
// T-01-94). Because the claim is made with the normalised lowercase
// address, case variants of one address ("A@Example.com" vs
// "a@example.com") share one cooldown.
//
// A granted claim commits before token generation, the insert, and the
// send, so a failed or quota-exhausted send — or even a failed token
// insert — still consumes the cooldown (RESEARCH.md Pitfall 3, DEC-J,
// DEC-X): that is what stops a failing provider being hammered in a retry
// loop. A mailer failure is wrapped so the caller can log detail
// server-side while returning a generic error to the visitor.
func (s *AuthService) RequestLink(ctx context.Context, rawEmail string) (string, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return "", err
	}

	now := s.now()
	if _, err := s.q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
		Email:          email,
		RequestedAt:    now,
		CooldownCutoff: now.Add(-ResendCooldown),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The claim was refused: a cooldown claim for this address
			// already exists and is still within the window.
			return "", ErrRateLimited
		}
		return "", fmt.Errorf("service: claiming email cooldown for %s: %w", email, err)
	}

	raw, hash, err := auth.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("service: generating magic-link token: %w", err)
	}

	if _, err := s.q.InsertMagicLinkToken(ctx, sqlcgen.InsertMagicLinkTokenParams{
		TokenHash: hash,
		Email:     email,
		ExpiresAt: now.Add(auth.TokenTTL),
	}); err != nil {
		return "", fmt.Errorf("service: persisting magic-link token: %w", err)
	}

	link := s.baseURL + "/auth/verify?token=" + url.QueryEscape(raw)
	if err := s.mailer.SendVerificationLink(ctx, email, link); err != nil {
		return "", fmt.Errorf("service: sending verification link: %w", err)
	}

	return email, nil
}

// VerifyToken decides the outcome of a clicked magic link for the given
// sessionID. It returns the outcome, an email address (populated only for
// OutcomeVerified and OutcomeConflict — empty for every other outcome), and
// a transport error for anything that isn't a clean outcome decision — a
// database error other than pgx.ErrNoRows is always returned as an error,
// never mapped to a fabricated outcome, so the handler can log it and
// render a generic 500. For OutcomeVerified the email is the
// newly-verified address; for OutcomeConflict it is the session's EXISTING
// verified account (what the visitor is currently signed in as), since the
// token's own target address was never proven in this browser.
//
// Sequence, in this exact order (DEC-D, DEC-E, RESEARCH.md Pattern 1 and
// Pitfall 2):
//  1. Reject an empty or malformed-shape token as OutcomeMalformed without
//     touching the database — a cheap shape check that keeps garbage out.
//  2. Hash the raw token and look up the row; pgx.ErrNoRows maps to
//     OutcomeMalformed (unknown token).
//  3. If the row is already used or expired, return that outcome. These
//     reads exist only to pick the right copy for the visitor, never to
//     authorise.
//  4. If the current session is already verified as a different account,
//     return OutcomeConflict WITHOUT consuming the token, so the rightful
//     owner's single-use link survives being opened in the wrong browser.
//  5. Consume the token atomically. This is the branch that actually
//     decides the request.
//  6. Lazily resolve (create-if-absent) the account for the token's email —
//     only now that ownership of the address is proven.
//  7. Bind the account onto the current session.
//  8. Return OutcomeVerified.
func (s *AuthService) VerifyToken(ctx context.Context, sessionID, rawToken string) (VerifyOutcome, string, error) {
	if !isWellFormedToken(rawToken) {
		return OutcomeMalformed, "", nil
	}

	hash := auth.HashToken(rawToken)
	row, err := s.q.GetMagicLinkTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OutcomeMalformed, "", nil
		}
		return "", "", fmt.Errorf("service: reading magic-link token: %w", err)
	}

	if row.UsedAt.Valid {
		return OutcomeUsed, "", nil
	}
	if !row.ExpiresAt.After(s.now()) {
		return OutcomeExpired, "", nil
	}

	existing, err := s.q.GetAccountBySessionID(ctx, sessionID)
	switch {
	case err == nil:
		if existing.Email != row.Email {
			// DEC-D/DEC-E: refuse to re-bind, and never consume the token —
			// the rightful owner's single-use link must survive being
			// opened in the wrong (already-verified) browser. The returned
			// email is the session's EXISTING verified account (what the
			// visitor is currently signed in as), not the token's target
			// address — that address was never proven in this browser,
			// which is the entire reason this is a conflict.
			return OutcomeConflict, existing.Email, nil
		}
	case errors.Is(err, pgx.ErrNoRows):
		// Unverified session opening a link it may not have requested
		// itself (D-07: cross-device verification is the supported happy
		// path, not an anomaly).
	default:
		return "", "", fmt.Errorf("service: reading current session's account: %w", err)
	}

	email, err := s.q.ConsumeToken(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if row.UsedAt.Valid {
				return OutcomeUsed, "", nil
			}
			return OutcomeExpired, "", nil
		}
		return "", "", fmt.Errorf("service: consuming magic-link token: %w", err)
	}

	if err := s.q.InsertAccount(ctx, email); err != nil {
		return "", "", fmt.Errorf("service: resolving account for %s: %w", email, err)
	}
	account, err := s.q.GetAccountByEmail(ctx, email)
	if err != nil {
		return "", "", fmt.Errorf("service: reading resolved account for %s: %w", email, err)
	}

	if err := s.q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{
		SessionID: sessionID,
		AccountID: &account.ID,
	}); err != nil {
		return "", "", fmt.Errorf("service: binding session to account: %w", err)
	}

	return OutcomeVerified, email, nil
}

// isWellFormedToken reports whether raw could plausibly be a value
// GenerateToken produced: non-empty, valid base64url (RawURLEncoding, no
// padding — matching GenerateToken's own encoding), and decoding to exactly
// rawTokenBytes bytes.
func isWellFormedToken(raw string) bool {
	if raw == "" {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return false
	}
	return len(decoded) == rawTokenBytes
}

// ReportsForAccount returns every report filed by any session ever bound to
// accountID, newest first — the profile page's core read (01.1-07-PLAN.md).
// Aggregation stays in the service layer rather than the handler, matching
// how ReportService owns the nearby query: sqlcgen.ReportsByAccountRow is a
// generated type callers should not need to know about directly, and a
// second caller (Phase 2's own profile-shaped read, if one ever exists)
// gets this same seam rather than a second hand-rolled query. accountID is
// always non-nil in practice — it's the ID of an account the request's
// gate has already resolved via a real database row (internal/api/gate.go)
// — but the underlying column is nullable (sessions.account_id), which is
// why sqlcgen.Queries.ReportsByAccount itself takes a pointer.
func (s *AuthService) ReportsForAccount(ctx context.Context, accountID int64) ([]sqlcgen.ReportsByAccountRow, error) {
	rows, err := s.q.ReportsByAccount(ctx, &accountID)
	if err != nil {
		return nil, fmt.Errorf("service: reading reports for account %d: %w", accountID, err)
	}
	return rows, nil
}

// ActivityReport is one Activity-page row: the ReportsByAccount row
// (unmodified) plus the VALIDATED category and severity and the resolved
// visibility and reason. Category/Severity are separate fields rather than
// an embed — embedding sqlcgen.ReportsByAccountRow would collide its own
// raw string Category/Severity fields with these validated typed ones.
type ActivityReport struct {
	Row        sqlcgen.ReportsByAccountRow
	Category   Category
	Severity   Severity
	Visibility Visibility
	Reason     ResolveReason
	// Trust comes from the same BuildTrust as the feed and the vote response
	// (D-21).
	Trust Trust
}

// VotedReport is one report the account currently confirms or disputes: the
// ActivityReport built exactly like a feed row (its own reporter and that
// reporter's signal), plus the account's latest content vote on it and when it
// was cast (D-02, latest wins). The embedded Row is the report's own columns,
// copied from the votes read into the ReportsByAccountRow shape so one handler
// mapper serves both lists.
type VotedReport struct {
	ActivityReport
	Vote    VoteValue
	VotedAt time.Time
}

// ActivityPage is everything the Activity page lists for one account: its own
// posts, newest first, and the reports it currently votes on, newest vote
// first. A report is on exactly one list (a post is never also a vote).
type ActivityPage struct {
	Posts []ActivityReport
	Votes []VotedReport
}

// validatedActivityMeta applies the one fallback both
// Activity lists share, so a post and a voted report can never disagree about
// an unknown value (T-01-17): other for a category, low for a severity.
func validatedActivityMeta(rawCategory, rawSeverity string) (Category, Severity) {
	category := Category(rawCategory)
	if !category.Valid() {
		category = CategoryOther
	}
	severity := Severity(rawSeverity)
	if !severity.Valid() {
		severity = SeverityLow
	}
	return category, severity
}

// ActivityForAccount returns the account's own posts, newest first. It is the
// posts half of ActivityPageForAccount, kept for callers that need only that
// list, so every caller exercises the page's real read path.
func (s *AuthService) ActivityForAccount(ctx context.Context, accountID int64) ([]ActivityReport, error) {
	page, err := s.ActivityPageForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return page.Posts, nil
}

// ActivityPageForAccount is the Activity page's one pass read (02-07, extended
// by quick task 261013-wm5): the account's own posts (ReportsForAccount, the
// existing unmodified query) and the reports it currently confirms or
// disputes (VotedReportsByAccount), then ONE ReporterHistoryReports call and
// ONE CurrentVotesForReports call over the whole page, then BuildVoteTally +
// Resolve + BuildTrust per report. The same batched shape Nearby uses keeps
// the page at a constant number of round trips (V-29), and the resolver stays
// the single authority on visibility (T-02-04).
//
// Order of reads: posts, votes, then (when voted reports remain)
// ReporterAccountsForReports once over the voted ids, then history once for
// the viewer (when it has posts) and the voted reports' reporters, then votes
// once over post ids, voted ids and history ids. An account with nothing to
// list makes only the first two reads.
//
// Each post carries the account's own reporter signal. Each voted report
// carries ITS reporter's signal, never the viewer's, so a voted card's trust
// equals the feed's trust for the same report, clock and rows (D-21). A report
// that is the account's own post is dropped from the votes (the self vote
// block makes that impossible today), and so is any vote row whose value is
// not a content value.
//
// Category and severity are validated ONCE, here, rather than in the handler:
// the fallback this function computes is the SAME value the resolver sees, so
// the rendered class name and the resolved trust state can never disagree
// (T-01-17). The account id is only ever the verified account's own (T-wm5-01).
func (s *AuthService) ActivityPageForAccount(ctx context.Context, accountID int64) (ActivityPage, error) {
	rows, err := s.ReportsForAccount(ctx, accountID)
	if err != nil {
		return ActivityPage{}, err
	}
	votedRows, err := s.q.VotedReportsByAccount(ctx, sqlcgen.VotedReportsByAccountParams{
		AccountID: accountID,
		Kind:      string(VoteKindContent),
	})
	if err != nil {
		return ActivityPage{}, fmt.Errorf("service: reading voted reports for account %d: %w", accountID, err)
	}

	postIDs := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		postIDs[row.ID] = struct{}{}
	}
	voted := make([]sqlcgen.VotedReportsByAccountRow, 0, len(votedRows))
	for _, row := range votedRows {
		if _, own := postIDs[row.ID]; own {
			continue
		}
		if !VoteValue(row.VoteValue).ValidFor(VoteKindContent) {
			continue
		}
		voted = append(voted, row)
	}
	if len(rows) == 0 && len(voted) == 0 {
		return ActivityPage{Posts: []ActivityReport{}, Votes: []VotedReport{}}, nil
	}

	// now is captured once, before the batched reads, because the history read
	// takes it as a parameter and ReporterSignals must see the same one.
	now := s.now().UTC()

	ids := make([]int64, 0, len(rows)+len(voted))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	votedIDs := make([]int64, 0, len(voted))
	for _, row := range voted {
		votedIDs = append(votedIDs, row.ID)
		ids = append(ids, row.ID)
	}

	reporterByReport := make(map[int64]*int64, len(voted))
	if len(voted) > 0 {
		reporterRows, err := s.q.ReporterAccountsForReports(ctx, votedIDs)
		if err != nil {
			return ActivityPage{}, fmt.Errorf("service: reading reporters of voted reports for account %d: %w", accountID, err)
		}
		for _, rr := range reporterRows {
			reporterByReport[rr.ReportID] = rr.ReporterAccountID
		}
	}

	var accounts []int64
	if len(rows) > 0 {
		accounts = append(accounts, accountID)
	}
	for _, id := range pageReporterAccounts(votedIDs, reporterByReport) {
		if id == accountID {
			continue
		}
		accounts = append(accounts, id)
	}

	var history []sqlcgen.ReporterHistoryReportsRow
	if len(accounts) > 0 {
		history, err = s.q.ReporterHistoryReports(ctx, reporterHistoryParams(accounts, now))
		if err != nil {
			return ActivityPage{}, fmt.Errorf("service: reading reporter history for account %d: %w", accountID, err)
		}
	}

	// In production the history ids of the viewer are a subset of the post ids,
	// because both reads key on the same account, but the union keeps one rule
	// across paths.
	voteRows, err := s.q.CurrentVotesForReports(ctx, voteReadIDs(ids, history))
	if err != nil {
		return ActivityPage{}, fmt.Errorf("service: reading current votes for account %d's page: %w", accountID, err)
	}
	signals := ReporterSignals(accounts, history, voteRows, now)
	votesByReport := voteRowsByReport(voteRows)

	posts := make([]ActivityReport, 0, len(rows))
	for _, row := range rows {
		category, severity := validatedActivityMeta(row.Category, row.Severity)

		meta := ReportMeta{Severity: severity, Category: category}
		// The viewing account is always a post's reporter (D-16's whole reason
		// the Reopen path needs no identity check at all).
		tally := BuildVoteTally(voteRows, row.ID, &accountID)
		vis, reason := Resolve(meta, tally, now)

		posts = append(posts, ActivityReport{
			Row:        row,
			Category:   category,
			Severity:   severity,
			Visibility: vis,
			Reason:     reason,
			Trust: BuildTrust(TrustInput{
				Rows:              voteRows,
				ReportID:          row.ID,
				ReporterAccountID: &accountID,
				Meta:              meta,
				Visibility:        vis,
				CreatedAt:         row.CreatedAt,
				ExpiresAt:         row.ExpiresAt,
				Now:               now,
				Reporter:          reporterSignalRef(signals, &accountID),
			}),
		})
	}

	votes := make([]VotedReport, 0, len(voted))
	for _, v := range voted {
		category, severity := validatedActivityMeta(v.Category, v.Severity)
		reporterID := reporterByReport[v.ID]
		reportVotes := votesByReport[v.ID]

		meta := ReportMeta{Severity: severity, Category: category}
		tally := BuildVoteTally(reportVotes, v.ID, reporterID)
		vis, reason := Resolve(meta, tally, now)

		votes = append(votes, VotedReport{
			ActivityReport: ActivityReport{
				Row: sqlcgen.ReportsByAccountRow{
					ID:                    v.ID,
					Category:              v.Category,
					Severity:              v.Severity,
					Description:           v.Description,
					Latitude:              v.Latitude,
					Longitude:             v.Longitude,
					Geohash:               v.Geohash,
					ShelterCapacityStatus: v.ShelterCapacityStatus,
					ShelterHeadcount:      v.ShelterHeadcount,
					CreatedAt:             v.CreatedAt,
					ExpiresAt:             v.ExpiresAt,
				},
				Category:   category,
				Severity:   severity,
				Visibility: vis,
				Reason:     reason,
				Trust: BuildTrust(TrustInput{
					Rows:              reportVotes,
					ReportID:          v.ID,
					ReporterAccountID: reporterID,
					Meta:              meta,
					Visibility:        vis,
					CreatedAt:         v.CreatedAt,
					ExpiresAt:         v.ExpiresAt,
					Now:               now,
					Reporter:          reporterSignalRef(signals, reporterID),
				}),
			},
			Vote:    VoteValue(v.VoteValue),
			VotedAt: v.VotedAt,
		})
	}
	sort.SliceStable(votes, func(i, j int) bool {
		if !votes[i].VotedAt.Equal(votes[j].VotedAt) {
			return votes[i].VotedAt.After(votes[j].VotedAt)
		}
		return votes[i].Row.ID > votes[j].Row.ID
	})

	return ActivityPage{Posts: posts, Votes: votes}, nil
}

// normalizeEmail trims rawEmail, parses it with the stdlib address parser
// (RESEARCH.md "Don't Hand-Roll" — no hand-rolled regex), rejects a display-name form
// such as "Someone <a@b.com>" since only a bare address is accepted, and
// lowercases the result so one human's address is one identity regardless
// of how they typed it.
func normalizeEmail(rawEmail string) (string, error) {
	trimmed := strings.TrimSpace(rawEmail)
	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Name != "" {
		return "", ValidationError{Field: "email", Message: "Enter a valid email address."}
	}
	return strings.ToLower(addr.Address), nil
}
