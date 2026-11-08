// account_username.go holds the username reads of the account service (Phase
// 7.1, D-07): the live check that answers while a person types, and the batched
// read Phases 8 and 9 use to show names. The save path lives in profile.go
// (saveUsername) next to the D-16 decision.

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// The states of the live check, as the browser reads them.
const (
	UsernameStateAvailable = "available"
	UsernameStateYours     = "yours"
	UsernameStateTaken     = "taken"
	UsernameStateInvalid   = "invalid"
	UsernameStateReserved  = "reserved"
	UsernameStateWait      = "wait"
)

// The screen words stored with a username. A username is shown to other people
// only when its word is clear (D-16): pending means the screening provider
// could not judge it when it was saved.
const (
	UsernameScreenClear   = "clear"
	UsernameScreenPending = "pending"
)

// ErrUsernameRefused is returned by UpdateProfile when the screening provider
// rejected the username. Nothing was stored. The handler answers it with
// UsernameRefusedMessage.
var ErrUsernameRefused = errors.New("service: username rejected by the text screening")

// UsernameWaitUntil is the moment the caller's own rename wait ends, for the
// Settings page: the last change time plus UsernameRenameGap as an RFC 3339 UTC
// time, rounded up to the whole second so the page never opens the window before
// the server would take a change. It is empty outside the wait (no username, no
// recorded change, or the gap is over, decided by the same rule as
// usernameWaitDays). It reads the profile the page already has on the service
// clock and asks the store for nothing. The page works out the days left from it
// on each opening; the save path still decides (saveUsername). Quick 261108-fku
// (owner decision 2026-11-08; review fix WR-02: the lock follows the deadline).
func (s *AccountService) UsernameWaitUntil(p AccountProfile) string {
	if p.Username == nil || p.UsernameChangedAt == nil {
		return ""
	}
	changed := pgtype.Timestamptz{Time: *p.UsernameChangedAt, Valid: true}
	if _, wait := usernameWaitDays(changed, s.now().UTC()); !wait {
		return ""
	}
	until := p.UsernameChangedAt.Add(UsernameRenameGap).UTC()
	if until.Nanosecond() != 0 {
		until = until.Truncate(time.Second).Add(time.Second)
	}
	return until.Format(time.RFC3339)
}

// UsernameCheck is the answer of the live check. Name is the normalised name
// that was judged. Message is the sentence to show. Days is set only for the
// wait state.
type UsernameCheck struct {
	State   string
	Name    string
	Message string
	Days    int
}

// CheckUsername answers the live check for the caller's own account: invalid,
// reserved, yours, wait, taken or available. It judges format, reserved words
// and uniqueness only. It never calls the screening provider (research section
// 5): the provider is asked once, on save.
func (s *AccountService) CheckUsername(ctx context.Context, accountID int64, raw string) (UsernameCheck, error) {
	name, class := ClassifyUsername(raw)
	switch class {
	case UsernameInvalid:
		return UsernameCheck{State: UsernameStateInvalid, Name: name, Message: UsernameRuleMessage}, nil
	case UsernameIsReserved:
		return UsernameCheck{State: UsernameStateReserved, Name: name, Message: UsernameReservedMessage}, nil
	}

	row, err := s.q.GetAccountProfile(ctx, accountID)
	if err != nil {
		return UsernameCheck{}, fmt.Errorf("service: CheckUsername: reading the account: %w", err)
	}
	if row.Username != nil {
		if *row.Username == name {
			return UsernameCheck{State: UsernameStateYours, Name: name, Message: UsernameYoursMessage}, nil
		}
		if days, wait := usernameWaitDays(row.UsernameChangedAt, s.now().UTC()); wait {
			return UsernameCheck{State: UsernameStateWait, Name: name, Message: UsernameWaitMessage(days), Days: days}, nil
		}
	}

	_, err = s.q.GetAccountIDByUsername(ctx, name)
	switch {
	case err == nil:
		return UsernameCheck{State: UsernameStateTaken, Name: name, Message: UsernameTakenMessage(name)}, nil
	case errors.Is(err, pgx.ErrNoRows):
		return UsernameCheck{State: UsernameStateAvailable, Name: name, Message: UsernameAvailableMessage(name)}, nil
	default:
		return UsernameCheck{}, fmt.Errorf("service: CheckUsername: looking the name up: %w", err)
	}
}

// UsernameView is one account's username and its screen word.
type UsernameView struct {
	Username string
	Screen   string
}

// UsernamesFor reads the usernames of many accounts in one query, for Phase 8
// comments and Phase 9 profiles. The map holds only accounts that have a
// username and are not deleted. A caller shows a name only when Screen is
// UsernameScreenClear and re-checks a pending one first (D-16). An empty id
// list answers an empty map without a query.
func (s *AccountService) UsernamesFor(ctx context.Context, ids []int64) (map[int64]UsernameView, error) {
	out := make(map[int64]UsernameView, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q.UsernamesByAccountIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("service: UsernamesFor: %w", err)
	}
	for _, r := range rows {
		if r.Username == nil {
			continue
		}
		screen := UsernameScreenPending
		if r.UsernameScreen != nil && *r.UsernameScreen == UsernameScreenClear {
			screen = UsernameScreenClear
		}
		out[r.ID] = UsernameView{Username: *r.Username, Screen: screen}
	}
	return out, nil
}
