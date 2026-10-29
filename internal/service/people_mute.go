// people_mute.go is person mute (Phase 9, plan 09-18, D-12, research G4): a
// reader can mute a person so that notes about that person stop. A mute is
// quiet and one way: the muted person is never told, and it never hides a
// report from any feed, map, list or profile, because hiding emergency
// information is never safe. It stops person level notes only (today the
// Reliable reporter alert, later the follow note), not Critical alerts near the
// home area and not changes to reports the reader posted or voted on, because
// those are about the place and the reader's own reports, not about the person.
//
// Person mute is its own table (person_mutes). It is separate from report_mutes,
// which mutes one report (Phase 3.2); the two never read each other.

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// MutedPeopleListCap is how many muted people the Muted people list shows. The
// plan called it MutedListCap, but that name already bounds muted reports
// (updates.go, Phase 3.2), so the person constant carries People in its name.
const MutedPeopleListCap = 500

// Mute mutes the person with this username (D-12). The person must be one the
// caller can see (as Profile decides), so a mute cannot be used to probe who
// has a private profile; muting yourself is ErrPersonNotFound. A repeat keeps
// one row. Nothing else changes: no follow ends and no report is hidden.
func (s *PeopleService) Mute(ctx context.Context, muterID int64, username string) error {
	name, ok := canonicalUsername(username)
	if !ok {
		return ErrPersonNotFound
	}
	person, err := s.q.GetPersonByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPersonNotFound
		}
		return fmt.Errorf("service: Mute: reading the person: %w", err)
	}
	if person.ID == muterID {
		return ErrPersonNotFound
	}
	rel, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: muterID, OtherID: person.ID})
	if err != nil {
		return fmt.Errorf("service: Mute: reading the relation: %w", err)
	}
	if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) {
		return ErrPersonNotFound
	}
	if err := s.q.MutePerson(ctx, sqlcgen.MutePersonParams{MuterID: muterID, MutedID: person.ID, Now: s.now().UTC()}); err != nil {
		return fmt.Errorf("service: Mute: muting: %w", err)
	}
	return nil
}

// Unmute takes a mute back by username. It works for any live account, also one
// that went private since, and succeeds for a well formed username that matches
// nobody; only a malformed username is ErrPersonNotFound (the same rule as
// Unfollow and Unblock, review W7).
func (s *PeopleService) Unmute(ctx context.Context, muterID int64, username string) error {
	id, ok, err := s.idByUsername(ctx, username)
	if err != nil || !ok {
		return err
	}
	if _, err := s.q.UnmutePerson(ctx, sqlcgen.UnmutePersonParams{MuterID: muterID, MutedID: id}); err != nil {
		return fmt.Errorf("service: Unmute: unmuting: %w", err)
	}
	return nil
}

// MutedPeople lists the people the account muted, newest first, at most
// MutedPeopleListCap. Only the list owner sees it, so a person who went private since
// still appears and can be unmuted. A name appears only while its screen is
// clear (07.1 D-16): a muted person whose new username is still pending is left
// out until it clears.
func (s *PeopleService) MutedPeople(ctx context.Context, accountID int64) ([]PersonEntry, error) {
	rows, err := s.q.MutedPeople(ctx, sqlcgen.MutedPeopleParams{MuterID: accountID, RowLimit: MutedPeopleListCap})
	if err != nil {
		return nil, fmt.Errorf("service: MutedPeople: listing: %w", err)
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

// MutedPeopleCount counts the people the account muted, for the Settings row.
func (s *PeopleService) MutedPeopleCount(ctx context.Context, accountID int64) (int, error) {
	n, err := s.q.CountMutedPeople(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("service: MutedPeopleCount: counting: %w", err)
	}
	return int(n), nil
}
