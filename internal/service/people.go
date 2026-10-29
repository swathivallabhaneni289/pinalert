// people.go is the service behind public profiles, following and blocking
// (Phase 9, plan 09-04, D-01, D-03, D-12). Every call is keyed by the account
// id the verified gate passes in, never by a request value (T-09-12); every
// username argument is normalised and shape checked before any query. Mute is
// in people_mute.go (plan 09-18); the Notify me bell, SetNotify, is below the
// follow rules (plan 09-23).
//
// Every follow end goes through endFollowPair or endFollowsOf (review W1) and
// is soft (D-21): the rows stay with a reason, so they can be restored.

package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// ErrPersonNotFound is the one answer for a person who cannot be seen: unknown,
// private, pending, deleted, blocking the caller, or a malformed username
// (research Pitfall 9). It never says which.
var ErrPersonNotFound = errors.New("service: person not found")

// ErrCannotFollow is returned when a visible person cannot be followed: it is
// you, you blocked them, or they do not hold the Reliable reporter tag now.
// The caller shows CannotFollowMessage for all of them (T-09-13).
var ErrCannotFollow = errors.New("service: person cannot be followed")

// ErrFollowingCap is returned when the caller already follows FollowingCap
// people.
var ErrFollowingCap = errors.New("service: following cap reached")

// ErrNotifyUnavailable is returned by SetNotify when the bell cannot be changed
// for this person: the caller does not follow them now, muted them, or the
// follow ended while the call ran. The caller shows NotifyUnavailableMessage
// for all of them.
var ErrNotifyUnavailable = errors.New("service: notify me unavailable")

// ErrNotifyCap is returned when the caller already has NotifyFollowsCap bells on.
var ErrNotifyCap = errors.New("service: notify me cap reached")

// FollowingCap is how many people one account can follow (D-12).
const FollowingCap = 200

// NotifyCapMessage is the refusal of a bell over the cap, built from
// NotifyFollowsCap so the number never drifts (UI-SPEC P9-C26).
var NotifyCapMessage = "You can turn on Notify me for up to " + strconv.Itoa(NotifyFollowsCap) + " people."

// NotifyUnavailableMessage answers every other refusal of the bell (UI-SPEC
// P9-C13). It never says which rule refused.
const NotifyUnavailableMessage = "Couldn't change this. Try again."

const (
	// PeopleListPageSize is how many named people one list page holds.
	PeopleListPageSize = 50
	// BlockedListCap is how many blocked people the Blocked people list shows.
	BlockedListCap = 500
)

// PeopleQuerier is the narrow store surface of PeopleService. It holds the
// people queries of plan 09-02, the username lookup and the two reads a tag
// read needs, and nothing else.
type PeopleQuerier interface {
	GetPersonByUsername(ctx context.Context, username string) (sqlcgen.GetPersonByUsernameRow, error)
	GetPersonByID(ctx context.Context, id int64) (sqlcgen.GetPersonByIDRow, error)
	SetProfilePublic(ctx context.Context, arg sqlcgen.SetProfilePublicParams) (int64, error)
	SetProfilePublicOn(ctx context.Context, arg sqlcgen.SetProfilePublicOnParams) (int64, error)
	MarkUsernameScreenClear(ctx context.Context, arg sqlcgen.MarkUsernameScreenClearParams) (int64, error)
	FollowAccount(ctx context.Context, arg sqlcgen.FollowAccountParams) (int64, error)
	EndFollowPair(ctx context.Context, arg sqlcgen.EndFollowPairParams) (int64, error)
	EndFollowsOf(ctx context.Context, arg sqlcgen.EndFollowsOfParams) (int64, error)
	// Plan 09-26 (SC5, research Q9): the queued phone notes that the two follow
	// ending helpers and the bell and mute paths delete.
	DeleteQueuedFollowPushesForFollowee(ctx context.Context, followeeID int64) (int64, error)
	DeleteQueuedFollowPush(ctx context.Context, arg sqlcgen.DeleteQueuedFollowPushParams) (int64, error)
	CountFollowing(ctx context.Context, followerID int64) (int64, error)
	PersonRelation(ctx context.Context, arg sqlcgen.PersonRelationParams) (sqlcgen.PersonRelationRow, error)
	FollowCounts(ctx context.Context, accountID int64) (sqlcgen.FollowCountsRow, error)
	FollowersPage(ctx context.Context, arg sqlcgen.FollowersPageParams) ([]sqlcgen.FollowersPageRow, error)
	FollowingPage(ctx context.Context, arg sqlcgen.FollowingPageParams) ([]sqlcgen.FollowingPageRow, error)
	CountNamedFollowers(ctx context.Context, arg sqlcgen.CountNamedFollowersParams) (int64, error)
	CountNamedFollowing(ctx context.Context, arg sqlcgen.CountNamedFollowingParams) (int64, error)
	BlockAccount(ctx context.Context, arg sqlcgen.BlockAccountParams) error
	UnblockAccount(ctx context.Context, arg sqlcgen.UnblockAccountParams) (int64, error)
	BlockedPeople(ctx context.Context, arg sqlcgen.BlockedPeopleParams) ([]sqlcgen.BlockedPeopleRow, error)
	CountBlocked(ctx context.Context, blockerID int64) (int64, error)
	// Phase 9 plan 18 (D-12, G4): person mute, in people_mute.go.
	MutePerson(ctx context.Context, arg sqlcgen.MutePersonParams) error
	UnmutePerson(ctx context.Context, arg sqlcgen.UnmutePersonParams) (int64, error)
	PersonMuted(ctx context.Context, arg sqlcgen.PersonMutedParams) (bool, error)
	MutedPeople(ctx context.Context, arg sqlcgen.MutedPeopleParams) ([]sqlcgen.MutedPeopleRow, error)
	CountMutedPeople(ctx context.Context, muterID int64) (int64, error)
	GetAccountIDByUsername(ctx context.Context, username string) (int64, error)
	ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error)
	CurrentVotesForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	// Phase 9 plan 23 (D-08, D-12): the Notify me bell, in people_notify.go.
	SetFollowNotify(ctx context.Context, arg sqlcgen.SetFollowNotifyParams) (int64, error)
	TurnOffFollowNotify(ctx context.Context, arg sqlcgen.TurnOffFollowNotifyParams) (int64, error)
	CountNotifyFollows(ctx context.Context, followerID int64) (int64, error)
	ActiveFollowNotify(ctx context.Context, arg sqlcgen.ActiveFollowNotifyParams) (bool, error)
}

// The generated Queries value satisfies the narrow querier.
var _ PeopleQuerier = (*sqlcgen.Queries)(nil)

// PeopleService applies the people rules.
type PeopleService struct {
	q        PeopleQuerier
	now      func() time.Time
	screener TextScreener
	signals  SignalSource
	ratings  RatingReader
}

// PeopleOption configures a PeopleService.
type PeopleOption func(*PeopleService)

// WithPeopleClock replaces the clock, for tests.
func WithPeopleClock(now func() time.Time) PeopleOption {
	return func(s *PeopleService) {
		if now != nil {
			s.now = now
		}
	}
}

// WithPeopleScreener sets the text screener the opt in checks use. Without one
// a Name and a pending username pass the abuse screen (the reserved word check
// still runs), as every other caller built without a screener does.
func WithPeopleScreener(sc TextScreener) PeopleOption {
	return func(s *PeopleService) {
		s.screener = sc
	}
}

// WithPeopleSignals sets the SignalSource every tag read of this service goes
// through (plan 09-13 supplies the first real one). Nil means outcomes only.
func WithPeopleSignals(src SignalSource) PeopleOption {
	return func(s *PeopleService) {
		s.signals = src
	}
}

// NewPeopleService stores the querier. It never dereferences it, so a router
// built with a nil store still constructs.
func NewPeopleService(q PeopleQuerier, opts ...PeopleOption) *PeopleService {
	s := &PeopleService{q: q, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// PersonEntry is one named person of a list.
type PersonEntry struct {
	Username string
}

// PeopleList is one page of a Followers or Following list (D-03): the named
// people, the count of everyone else and the sentence for it, and an opaque
// cursor for the next page ("" on the last page).
type PeopleList struct {
	Entries     []PersonEntry
	OthersCount int
	OthersText  string
	Next        string
}

// PeopleCounts are the three totals Settings shows.
type PeopleCounts struct {
	Following int
	Followers int
	Blocked   int
}

// canonicalUsername normalises a typed username and answers ok only for one
// that passes the shape and reserved rules, so a malformed value never reaches
// a query.
func canonicalUsername(raw string) (string, bool) {
	name, class := ClassifyUsername(raw)
	return name, class == UsernameUsable
}

// publicFace says whether a person row is one anyone may see: opted in, a
// username whose screen is clear (07.1 D-16), and not blocking the viewer.
func publicFace(username, screen *string, public bool, blockedViewer bool) bool {
	return public && !blockedViewer && username != nil && screen != nil && *screen == UsernameScreenClear
}

// SetProfilePublic switches "Let others follow me" for the account (D-01).
//
// On: the account needs a username (a missing one is refused with
// PublicUsernameRefusedMessage); a pending one is screened again with
// ScreenAbuse over UsernameScreenText (07.1 D-16) and marked clear on a clear
// answer; the Name goes through ScreenPublicName. A refusal never turns the
// switch on, and a check that cannot finish refuses with
// PublicNameUnfinishedMessage (fail closed). A profile already on is left as it
// is, so a repeat call makes no provider call.
//
// Off: the switch turns off first, so FollowAccount refuses the account from
// then on, and then every active follower is ended with reason profile_off. The
// rows remain; turning it on later starts with no followers (D-12, D-21, SC5).
// The account's own Following rows stay active.
func (s *PeopleService) SetProfilePublic(ctx context.Context, accountID int64, on bool) error {
	now := s.now().UTC()
	if !on {
		rows, err := s.q.SetProfilePublic(ctx, sqlcgen.SetProfilePublicParams{IsPublic: false, Now: now, ID: accountID})
		if err != nil {
			return fmt.Errorf("service: SetProfilePublic: switching off account %d: %w", accountID, err)
		}
		if rows == 0 {
			return ErrPersonNotFound
		}
		if _, err := endFollowsOf(ctx, s.q, accountID, FollowEndProfileOff, now); err != nil {
			return fmt.Errorf("service: SetProfilePublic: ending the follows of account %d: %w", accountID, err)
		}
		return nil
	}

	row, err := s.q.GetPersonByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPersonNotFound
		}
		return fmt.Errorf("service: SetProfilePublic: reading account %d: %w", accountID, err)
	}
	if row.ProfilePublic {
		return nil
	}
	if row.Username == nil {
		return ValidationError{Field: "profile_public", Message: PublicUsernameRefusedMessage}
	}

	if row.UsernameScreen == nil || *row.UsernameScreen != UsernameScreenClear {
		switch ScreenAbuse(ctx, s.screener, UsernameScreenText(*row.Username)).Outcome {
		case TextClear:
			if _, err := s.q.MarkUsernameScreenClear(ctx, sqlcgen.MarkUsernameScreenClearParams{ID: accountID, Username: *row.Username}); err != nil {
				return fmt.Errorf("service: SetProfilePublic: marking the username clear for account %d: %w", accountID, err)
			}
		case TextUnfinished:
			return ValidationError{Field: "profile_public", Message: PublicNameUnfinishedMessage}
		default:
			return ValidationError{Field: "profile_public", Message: PublicUsernameRefusedMessage}
		}
		// The write is race safe (it needs the same name still pending). Read
		// the row again so a rename between the screen and the write cannot
		// leave a public profile with an unscreened name.
		again, err := s.q.GetPersonByID(ctx, accountID)
		if err != nil {
			return fmt.Errorf("service: SetProfilePublic: re-reading account %d: %w", accountID, err)
		}
		if again.Username == nil || again.UsernameScreen == nil || *again.UsernameScreen != UsernameScreenClear {
			return ValidationError{Field: "profile_public", Message: PublicUsernameRefusedMessage}
		}
	}

	if row.DisplayName == nil || strings.TrimSpace(*row.DisplayName) == "" {
		return ValidationError{Field: "profile_public", Message: PublicNameRefusedMessage}
	}
	switch ScreenPublicName(ctx, s.screener, *row.DisplayName) {
	case PublicNameRefused:
		return ValidationError{Field: "profile_public", Message: PublicNameRefusedMessage}
	case PublicNameUnfinished:
		return ValidationError{Field: "profile_public", Message: PublicNameUnfinishedMessage}
	}

	// The switch turns on only while the stored Name is still the one just
	// screened (WR-S04): a Name edit that committed during the screen call
	// leaves 0 rows, and the person tries again.
	rows, err := s.q.SetProfilePublicOn(ctx, sqlcgen.SetProfilePublicOnParams{Now: now, ID: accountID, ScreenedName: *row.DisplayName})
	if err != nil {
		return fmt.Errorf("service: SetProfilePublic: switching on account %d: %w", accountID, err)
	}
	if rows == 0 {
		if _, err := s.q.GetPersonByID(ctx, accountID); errors.Is(err, pgx.ErrNoRows) {
			return ErrPersonNotFound
		}
		return ValidationError{Field: "profile_public", Message: PublicNameUnfinishedMessage}
	}
	return nil
}

// Follow makes followerID follow the person with this username (D-01, D-12).
//
// Order of the answers: a malformed, unknown, private, pending, deleted or
// blocking person is ErrPersonNotFound; yourself, a person you blocked, or a
// person who does not hold the Reliable reporter tag now is ErrCannotFollow;
// following someone you already follow is nil and changes nothing (so a repeat
// at the cap is not an error); at FollowingCap it is ErrFollowingCap. The tag is
// read now through LoadReporterSignals, and FollowAccount re-checks opt in, the
// clear username and both block directions in its own statement.
func (s *PeopleService) Follow(ctx context.Context, followerID int64, username string) error {
	name, ok := canonicalUsername(username)
	if !ok {
		return ErrPersonNotFound
	}
	person, err := s.q.GetPersonByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPersonNotFound
		}
		return fmt.Errorf("service: Follow: reading the person: %w", err)
	}
	if person.ID == followerID {
		return ErrCannotFollow
	}
	rel, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: followerID, OtherID: person.ID})
	if err != nil {
		return fmt.Errorf("service: Follow: reading the relation: %w", err)
	}
	if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) {
		return ErrPersonNotFound
	}
	if rel.ViewerBlocked {
		return ErrCannotFollow
	}
	if rel.ViewerFollows {
		return nil
	}

	signals, err := LoadReporterSignals(ctx, s.q, s.signals, []int64{person.ID}, s.now().UTC())
	if err != nil {
		return fmt.Errorf("service: Follow: reading the tag: %w", err)
	}
	if signals[person.ID].Tag != ReporterReliable {
		return ErrCannotFollow
	}

	count, err := s.q.CountFollowing(ctx, followerID)
	if err != nil {
		return fmt.Errorf("service: Follow: counting follows: %w", err)
	}
	if count >= FollowingCap {
		return ErrFollowingCap
	}

	rows, err := s.q.FollowAccount(ctx, sqlcgen.FollowAccountParams{
		FollowerID: followerID,
		FolloweeID: person.ID,
		Now:        s.now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("service: Follow: following: %w", err)
	}
	if rows == 0 {
		// Already following (a race with another tap) or refused in the
		// statement; the relation tells which.
		again, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: followerID, OtherID: person.ID})
		if err != nil {
			return fmt.Errorf("service: Follow: re-reading the relation: %w", err)
		}
		if again.ViewerFollows {
			return nil
		}
		return ErrCannotFollow
	}
	return nil
}

// SetNotify switches Notify me for the person with this username (D-08, D-12).
//
// The bell exists only while the caller follows the person and has not muted
// them. Order of the answers: a malformed, unknown, private, pending, deleted or
// blocking person is ErrPersonNotFound (the uniform answer of every people
// route, so the call never probes who opted in); a person the caller does not
// follow, has muted or has blocked, and yourself, is ErrNotifyUnavailable;
// turning a bell on when NotifyFollowsCap are already on is ErrNotifyCap, but a
// repeat "on" for a bell that is already on changes nothing and is never
// refused by the cap. Turning on records the time in SetFollowNotify (a repeat
// keeps the first time), so only reports posted later raise a note; turning off
// clears it. SetFollowNotify itself checks nothing else, so every rule is here.
//
// The cap is read and then the bell is written, so two simultaneous calls from
// one account could pass the cap together and leave a bell or two above it. The
// producer reads at most NotifyFollowsCap bells per account, so that cannot
// fan out. The mute is read again after the write for the same reason: a mute
// that lands in between turns the bell off again and the call is refused.
func (s *PeopleService) SetNotify(ctx context.Context, followerID int64, username string, on bool) error {
	name, ok := canonicalUsername(username)
	if !ok {
		return ErrPersonNotFound
	}
	person, err := s.q.GetPersonByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPersonNotFound
		}
		return fmt.Errorf("service: SetNotify: reading the person: %w", err)
	}
	if person.ID == followerID {
		return ErrNotifyUnavailable
	}
	rel, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: followerID, OtherID: person.ID})
	if err != nil {
		return fmt.Errorf("service: SetNotify: reading the relation: %w", err)
	}
	if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) {
		return ErrPersonNotFound
	}
	if rel.ViewerBlocked || !rel.ViewerFollows {
		return ErrNotifyUnavailable
	}
	muted, err := s.q.PersonMuted(ctx, sqlcgen.PersonMutedParams{MuterID: followerID, MutedID: person.ID})
	if err != nil {
		return fmt.Errorf("service: SetNotify: reading the mute: %w", err)
	}
	if muted {
		return ErrNotifyUnavailable
	}

	pair := sqlcgen.ActiveFollowNotifyParams{FollowerID: followerID, FolloweeID: person.ID}
	if on {
		already, err := s.q.ActiveFollowNotify(ctx, pair)
		if err != nil {
			return fmt.Errorf("service: SetNotify: reading the bell: %w", err)
		}
		if !already {
			count, err := s.q.CountNotifyFollows(ctx, followerID)
			if err != nil {
				return fmt.Errorf("service: SetNotify: counting bells: %w", err)
			}
			if count >= NotifyFollowsCap {
				return ErrNotifyCap
			}
		}
	}

	rows, err := s.q.SetFollowNotify(ctx, sqlcgen.SetFollowNotifyParams{
		Notify:     on,
		Now:        s.now().UTC(),
		FollowerID: followerID,
		FolloweeID: person.ID,
	})
	if err != nil {
		return fmt.Errorf("service: SetNotify: writing the bell: %w", err)
	}
	if rows == 0 {
		// The follow ended between the read and the write.
		return ErrNotifyUnavailable
	}
	if !on {
		// The bell is off: a follow note already queued for this pair is deleted
		// (plan 09-26, SC5, research Q9, D-24). Sent notes and bell rows stay.
		if _, err := s.q.DeleteQueuedFollowPush(ctx, sqlcgen.DeleteQueuedFollowPushParams{FollowerID: followerID, FolloweeID: person.ID}); err != nil {
			return fmt.Errorf("service: SetNotify: deleting the queued follow notes: %w", err)
		}
		return nil
	}
	muted, err = s.q.PersonMuted(ctx, sqlcgen.PersonMutedParams{MuterID: followerID, MutedID: person.ID})
	if err != nil {
		return fmt.Errorf("service: SetNotify: re-reading the mute: %w", err)
	}
	if muted {
		if _, err := s.q.TurnOffFollowNotify(ctx, sqlcgen.TurnOffFollowNotifyParams{FollowerID: followerID, FolloweeID: person.ID}); err != nil {
			return fmt.Errorf("service: SetNotify: turning the bell off again: %w", err)
		}
		return ErrNotifyUnavailable
	}
	return nil
}

// idByUsername resolves a well formed username to a live account id for the
// undo actions. ok is false, with no error, when nobody holds the name, so
// Unfollow, Unblock and Remove follower reveal nothing. A malformed username is
// ErrPersonNotFound. It does not look at the profile switch, so a list owner can
// always undo, also after the other person went private.
func (s *PeopleService) idByUsername(ctx context.Context, username string) (id int64, ok bool, err error) {
	name, good := canonicalUsername(username)
	if !good {
		return 0, false, ErrPersonNotFound
	}
	id, err = s.q.GetAccountIDByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("service: resolving a username: %w", err)
	}
	return id, true, nil
}

// Unfollow ends the follow of the person with this username (reason
// unfollowed). A well formed username that matches nobody succeeds and changes
// nothing; only a malformed one is ErrPersonNotFound.
func (s *PeopleService) Unfollow(ctx context.Context, followerID int64, username string) error {
	id, ok, err := s.idByUsername(ctx, username)
	if err != nil || !ok {
		return err
	}
	if _, err := endFollowPair(ctx, s.q, followerID, id, FollowEndUnfollowed, s.now().UTC()); err != nil {
		return fmt.Errorf("service: Unfollow: ending the follow: %w", err)
	}
	return nil
}

// RemoveFollower ends the follow of this follower of the account (reason
// removed). The answer is the same as Unfollow's for an unknown username.
func (s *PeopleService) RemoveFollower(ctx context.Context, accountID int64, followerUsername string) error {
	id, ok, err := s.idByUsername(ctx, followerUsername)
	if err != nil || !ok {
		return err
	}
	if _, err := endFollowPair(ctx, s.q, id, accountID, FollowEndRemoved, s.now().UTC()); err != nil {
		return fmt.Errorf("service: RemoveFollower: ending the follow: %w", err)
	}
	return nil
}

// RemoveAllFollowers ends every active follower of the account (reason
// removed) and returns how many it ended.
func (s *PeopleService) RemoveAllFollowers(ctx context.Context, accountID int64) (int64, error) {
	n, err := endFollowsOf(ctx, s.q, accountID, FollowEndRemoved, s.now().UTC())
	if err != nil {
		return 0, fmt.Errorf("service: RemoveAllFollowers: ending the follows of account %d: %w", accountID, err)
	}
	return n, nil
}

// Block blocks the person with this username (D-12). The person must be one the
// caller can see (as Profile decides), so a block cannot be used to probe who
// has a private profile; blocking yourself is ErrPersonNotFound. The block row
// is written first, then the follows in both directions end with reason blocked
// through endFollowPair; FollowAccount refuses once the block row exists, so
// nothing can slip in between.
func (s *PeopleService) Block(ctx context.Context, blockerID int64, username string) error {
	name, ok := canonicalUsername(username)
	if !ok {
		return ErrPersonNotFound
	}
	person, err := s.q.GetPersonByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPersonNotFound
		}
		return fmt.Errorf("service: Block: reading the person: %w", err)
	}
	if person.ID == blockerID {
		return ErrPersonNotFound
	}
	rel, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: blockerID, OtherID: person.ID})
	if err != nil {
		return fmt.Errorf("service: Block: reading the relation: %w", err)
	}
	if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) {
		return ErrPersonNotFound
	}

	now := s.now().UTC()
	if err := s.q.BlockAccount(ctx, sqlcgen.BlockAccountParams{BlockerID: blockerID, BlockedID: person.ID, Now: now}); err != nil {
		return fmt.Errorf("service: Block: blocking: %w", err)
	}
	if _, err := endFollowPair(ctx, s.q, blockerID, person.ID, FollowEndBlocked, now); err != nil {
		return fmt.Errorf("service: Block: ending your follow: %w", err)
	}
	if _, err := endFollowPair(ctx, s.q, person.ID, blockerID, FollowEndBlocked, now); err != nil {
		return fmt.Errorf("service: Block: ending their follow: %w", err)
	}
	return nil
}

// Unblock takes a block back by username. It works for any live account, also
// one that went private since, and succeeds for a well formed username that
// matches nobody. Follows that the block ended stay ended.
func (s *PeopleService) Unblock(ctx context.Context, blockerID int64, username string) error {
	id, ok, err := s.idByUsername(ctx, username)
	if err != nil || !ok {
		return err
	}
	if _, err := s.q.UnblockAccount(ctx, sqlcgen.UnblockAccountParams{BlockerID: blockerID, BlockedID: id}); err != nil {
		return fmt.Errorf("service: Unblock: unblocking: %w", err)
	}
	return nil
}

// BlockedPeople lists the people the account blocked, newest first, at most
// BlockedListCap. Only the list owner sees it, so a person who went private
// since still appears and can be unblocked. A name appears only while its
// screen is clear (07.1 D-16): a blocked person whose new username is still
// pending is left out until it clears.
func (s *PeopleService) BlockedPeople(ctx context.Context, accountID int64) ([]PersonEntry, error) {
	rows, err := s.q.BlockedPeople(ctx, sqlcgen.BlockedPeopleParams{BlockerID: accountID, RowLimit: BlockedListCap})
	if err != nil {
		return nil, fmt.Errorf("service: BlockedPeople: listing: %w", err)
	}
	out := make([]PersonEntry, 0, len(rows))
	for _, r := range rows {
		if r.Username == nil || r.UsernameScreen == nil || *r.UsernameScreen != UsernameScreenClear {
			continue
		}
		out = append(out, PersonEntry{Username: *r.Username})
	}
	return out, nil
}

// Counts returns the three totals Settings shows: people you follow, people who
// follow you and people you blocked.
func (s *PeopleService) Counts(ctx context.Context, accountID int64) (PeopleCounts, error) {
	fc, err := s.q.FollowCounts(ctx, accountID)
	if err != nil {
		return PeopleCounts{}, fmt.Errorf("service: Counts: follow counts: %w", err)
	}
	blocked, err := s.q.CountBlocked(ctx, accountID)
	if err != nil {
		return PeopleCounts{}, fmt.Errorf("service: Counts: blocked count: %w", err)
	}
	return PeopleCounts{Following: int(fc.Following), Followers: int(fc.Followers), Blocked: int(blocked)}, nil
}

// listOwner resolves whose list is asked for. Your own list always opens; for
// anyone else the person must be visible to the viewer and not blocked by the
// viewer, otherwise ErrPersonNotFound (the same answer as Profile).
func (s *PeopleService) listOwner(ctx context.Context, viewerID int64, username string) (int64, error) {
	name, ok := canonicalUsername(username)
	if !ok {
		return 0, ErrPersonNotFound
	}
	person, err := s.q.GetPersonByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrPersonNotFound
		}
		return 0, fmt.Errorf("service: people list: reading the person: %w", err)
	}
	if person.ID == viewerID {
		return person.ID, nil
	}
	rel, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: viewerID, OtherID: person.ID})
	if err != nil {
		return 0, fmt.Errorf("service: people list: reading the relation: %w", err)
	}
	if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) || rel.ViewerBlocked {
		return 0, ErrPersonNotFound
	}
	return person.ID, nil
}

// Followers lists the named people who follow the person, newest first, 50 per
// page, after the opaque cursor (empty for the first page). The people who are
// not named (private, pending, deleted or blocked either way with the viewer)
// are counted in OthersCount (D-03).
func (s *PeopleService) Followers(ctx context.Context, viewerID int64, username, after string) (PeopleList, error) {
	return s.list(ctx, viewerID, username, after, true)
}

// Following lists the named people the person follows, the same rules as
// Followers.
func (s *PeopleService) Following(ctx context.Context, viewerID int64, username, after string) (PeopleList, error) {
	return s.list(ctx, viewerID, username, after, false)
}

// peopleCursorMessage is the refusal of a cursor that does not decode.
const peopleCursorMessage = "That page could not be read."

func (s *PeopleService) list(ctx context.Context, viewerID int64, username, after string, followers bool) (PeopleList, error) {
	ownerID, err := s.listOwner(ctx, viewerID, username)
	if err != nil {
		return PeopleList{}, err
	}
	afterAt, afterName, err := decodePeopleCursor(after)
	if err != nil {
		return PeopleList{}, err
	}

	// One row more than a page says whether a next page exists.
	limit := int32(PeopleListPageSize + 1)
	type entry struct {
		username string
		at       time.Time
	}
	var rows []entry
	var named int64
	if followers {
		got, err := s.q.FollowersPage(ctx, sqlcgen.FollowersPageParams{
			AccountID: ownerID, ViewerID: viewerID, AfterAt: afterAt, AfterUsername: afterName, RowLimit: limit,
		})
		if err != nil {
			return PeopleList{}, fmt.Errorf("service: Followers: listing: %w", err)
		}
		for _, r := range got {
			rows = append(rows, entry{r.Username, r.CreatedAt})
		}
		named, err = s.q.CountNamedFollowers(ctx, sqlcgen.CountNamedFollowersParams{AccountID: ownerID, ViewerID: viewerID})
		if err != nil {
			return PeopleList{}, fmt.Errorf("service: Followers: counting named: %w", err)
		}
	} else {
		got, err := s.q.FollowingPage(ctx, sqlcgen.FollowingPageParams{
			AccountID: ownerID, ViewerID: viewerID, AfterAt: afterAt, AfterUsername: afterName, RowLimit: limit,
		})
		if err != nil {
			return PeopleList{}, fmt.Errorf("service: Following: listing: %w", err)
		}
		for _, r := range got {
			rows = append(rows, entry{r.Username, r.CreatedAt})
		}
		named, err = s.q.CountNamedFollowing(ctx, sqlcgen.CountNamedFollowingParams{AccountID: ownerID, ViewerID: viewerID})
		if err != nil {
			return PeopleList{}, fmt.Errorf("service: Following: counting named: %w", err)
		}
	}
	totals, err := s.q.FollowCounts(ctx, ownerID)
	if err != nil {
		return PeopleList{}, fmt.Errorf("service: people list: counting: %w", err)
	}
	total := totals.Following
	if followers {
		total = totals.Followers
	}

	out := PeopleList{Entries: []PersonEntry{}}
	if len(rows) > PeopleListPageSize {
		rows = rows[:PeopleListPageSize]
		last := rows[len(rows)-1]
		out.Next = encodePeopleCursor(last.at, last.username)
	}
	for _, r := range rows {
		out.Entries = append(out.Entries, PersonEntry{Username: r.username})
	}
	others := int(total - named)
	if others < 0 {
		others = 0
	}
	out.OthersCount = others
	out.OthersText = OthersText(others, named > 0)
	return out, nil
}

// encodePeopleCursor builds the opaque keyset cursor: base64url of the
// created_at microseconds and the username.
func encodePeopleCursor(at time.Time, username string) string {
	raw := strconv.FormatInt(at.UnixMicro(), 10) + ":" + username
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodePeopleCursor reverses encodePeopleCursor. An empty cursor is the first
// page. Anything that does not decode to microseconds and a well formed
// username is a ValidationError on after.
func decodePeopleCursor(cursor string) (pgtype.Timestamptz, *string, error) {
	if cursor == "" {
		return pgtype.Timestamptz{}, nil, nil
	}
	bad := ValidationError{Field: "after", Message: peopleCursorMessage}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return pgtype.Timestamptz{}, nil, bad
	}
	micros, name, found := strings.Cut(string(raw), ":")
	if !found || !usernameShapeOK(name) {
		return pgtype.Timestamptz{}, nil, bad
	}
	us, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return pgtype.Timestamptz{}, nil, bad
	}
	return pgtype.Timestamptz{Time: time.UnixMicro(us).UTC(), Valid: true}, &name, nil
}

// logTagLost records an evaluate on touch end by ids and counts only (T-09-15).
func logTagLost(accountID, ended int64) {
	if ended > 0 {
		log.Printf("service: Profile: ended %d follows of account %d, the tag is no longer Reliable reporter", ended, accountID)
	}
}
