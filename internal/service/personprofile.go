// personprofile.go is the profile header read (Phase 9, plan 09-04, D-03, D-13,
// D-15, SC5): one server written PersonProfile for the page, the options menu
// and the lists, with one uniform not found. Mute, notify and rating fields are
// added by later plans.

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// RatingReader is the part of the ratings service the profile read needs: whether
// the viewer may rate the person, and the viewer's own current rating (plan
// 09-14, D-02, D-23). *RatingService satisfies it.
type RatingReader interface {
	MayRate(ctx context.Context, raterID, reporterID int64) (bool, error)
	YourRating(ctx context.Context, raterID, reporterID int64) (int, bool, error)
}

var _ RatingReader = (*RatingService)(nil)

// WithPeopleRatings sets the rating reader the profile read fills may_rate and
// your_rating from. Without one both stay false and 0.
func WithPeopleRatings(r RatingReader) PeopleOption {
	return func(s *PeopleService) {
		s.ratings = r
	}
}

// PersonProfile is everything the profile header shows, built on the server
// (D-13): the order of the fields is the order of the header, then the home
// area (D-15). The browser draws these strings as they are and never formats a
// number or a date.
//
// StarsNumber is StarsText of the score, for example "3.6", and the template
// draws the word "stars" after it in its own weight; StarsLabel is the whole
// accessible sentence. Both are empty for a New reporter. FollowersWord is
// "Follower" or "Followers" and FollowingWord is "Following", drawn after the
// count in its own weight; FollowersText and FollowingText are the whole
// phrases for accessible names. The counts include people shown only as a
// number (D-03).
//
// For a person you blocked only Username and BlockedByYou are set (the Blocked
// by you face). The account id is kept unexported for server code
// (AccountID), so it never reaches JSON or a template.
type PersonProfile struct {
	Username     string
	Name         string
	HomeAreaName string
	Initial      string

	StarsNumber string
	StarsLabel  string
	// StarsFromReports and StarsFromRatings are the two parts of the stars total,
	// shown only from MinRatersForStars counted raters on (D-02, UI-SPEC 1c), for
	// example "3.6 from reports" and "4.0 from ratings by 12 people". Both are empty
	// below that, so a profile with fewer raters shows the total alone. The tapped
	// part is rounded to half stars and never exact (review W5).
	StarsFromReports string
	StarsFromRatings string
	TagWord          string
	TagSentence      string
	MemberSince      string

	FollowersWord  string
	FollowingWord  string
	FollowersText  string
	FollowingText  string
	FollowersCount int
	FollowingCount int

	// StateLine is the line under the buttons on your own page (UI-SPEC 1g)
	// and empty on anyone else's.
	StateLine string

	IsSelf        bool
	ProfilePublic bool
	Reliable      bool
	CanFollow     bool
	Following     bool
	FollowsYou    bool
	BlockedByYou  bool

	// Muted says the viewer muted this person (plan 09-18, D-12). It is the
	// viewer's own state, never the other way round: the muted person can never
	// learn they were muted, so it is false on your own page and on the profile
	// of someone who muted you.
	Muted bool

	// Notify says the viewer has the Notify me bell on for this person (plan
	// 09-23, D-08). It is true only while the viewer follows the person and has
	// not muted them, and false on your own page.
	Notify bool

	// MayRate says the viewer may rate this person now, and YourRating is the
	// viewer's own current rating, 1 to 5, or 0 for none (plan 09-14, D-02, D-23).
	// Both are false and 0 on your own page, when the viewer blocked the person,
	// and when no rating reader is configured.
	MayRate    bool
	YourRating int

	accountID int64
}

// AccountID is the profile owner's account id, for server code only.
func (p PersonProfile) AccountID() int64 { return p.accountID }

// initialOf is the first ASCII letter of the username in upper case. A username
// may start with a digit, an underscore or a full stop, so the first letter is
// searched for (every username has one).
func initialOf(username string) string {
	for i := 0; i < len(username); i++ {
		c := username[i]
		if c >= 'a' && c <= 'z' {
			return strings.ToUpper(string(c))
		}
	}
	if username == "" {
		return ""
	}
	return strings.ToUpper(username[:1])
}

// Profile reads the profile of the person with this username for the viewer.
//
// One error for four cases: an unknown, private, deleted or pending username,
// and a person who blocked the viewer, all return ErrPersonNotFound and nothing
// else, so the answer never tells who opted in (research Pitfall 9, T-09-10).
// The not found branches return before any count read and before any tag read
// for a person the viewer cannot see. A viewer who blocked the person gets the
// Blocked by you face; your own page always opens, whatever its switch or its
// username state.
//
// Evaluate on touch (SC5, D-24): when the profile is on, the tag is read now
// through LoadReporterSignals, and when it is not Reliable reporter every active
// follow of the person ends with reason tag_lost before the counts are read, so
// the page shows 0 Followers. The write inside a read is safe because it only
// soft ends follows that SC5 already ends; the sweep of plan 09-06 does the
// same for people nobody looks at. Muted (plan 09-18) is read for another
// person's visible profile only. Notify (plan 09-23) is the viewer's own bell, read
// only while the viewer follows and has not muted the person.
func (s *PeopleService) Profile(ctx context.Context, viewerID int64, username string) (PersonProfile, error) {
	name, ok := canonicalUsername(username)
	if !ok {
		return PersonProfile{}, ErrPersonNotFound
	}
	person, err := s.q.GetPersonByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PersonProfile{}, ErrPersonNotFound
		}
		return PersonProfile{}, fmt.Errorf("service: Profile: reading the person: %w", err)
	}

	self := person.ID == viewerID
	var rel sqlcgen.PersonRelationRow
	if !self {
		rel, err = s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: viewerID, OtherID: person.ID})
		if err != nil {
			return PersonProfile{}, fmt.Errorf("service: Profile: reading the relation: %w", err)
		}
		if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) {
			return PersonProfile{}, ErrPersonNotFound
		}
		if rel.ViewerBlocked {
			return PersonProfile{Username: *person.Username, BlockedByYou: true}, nil
		}
	}

	now := s.now().UTC()
	signals, err := LoadReporterSignals(ctx, s.q, s.signals, []int64{person.ID}, now)
	if err != nil {
		return PersonProfile{}, fmt.Errorf("service: Profile: reading the tag: %w", err)
	}
	sig := signals[person.ID]
	reliable := sig.Tag == ReporterReliable

	following := rel.ViewerFollows
	if person.ProfilePublic && !reliable {
		ended, err := endFollowsOf(ctx, s.q, person.ID, FollowEndTagLost, now)
		if err != nil {
			return PersonProfile{}, fmt.Errorf("service: Profile: ending the follows of a lost tag: %w", err)
		}
		logTagLost(person.ID, ended)
		following = false
	}

	counts, err := s.q.FollowCounts(ctx, person.ID)
	if err != nil {
		return PersonProfile{}, fmt.Errorf("service: Profile: counting: %w", err)
	}

	p := PersonProfile{
		MemberSince:    MemberSinceText(person.CreatedAt),
		TagWord:        sig.Label,
		TagSentence:    sig.Sentence,
		FollowersCount: int(counts.Followers),
		FollowingCount: int(counts.Following),
		FollowersWord:  FollowersWord(int(counts.Followers)),
		FollowingWord:  "Following",
		FollowersText:  FollowersCountText(int(counts.Followers)),
		FollowingText:  FollowingCountText(int(counts.Following)),
		IsSelf:         self,
		ProfilePublic:  person.ProfilePublic,
		Reliable:       reliable,
		Following:      following,
		FollowsYou:     rel.FollowsViewer,
		accountID:      person.ID,
	}
	if person.Username != nil {
		p.Username = *person.Username
		p.Initial = initialOf(p.Username)
	}
	if person.DisplayName != nil {
		p.Name = *person.DisplayName
	}
	if person.HomeAreaName != nil {
		p.HomeAreaName = *person.HomeAreaName
	}
	if sig.Score != nil {
		p.StarsNumber = StarsText(*sig.Score)
		p.StarsLabel = StarsGroupLabel(*sig.Score)
		// The two parts only when the tapped ratings applied to this signal: at
		// least MinRatersForStars counted raters (cells), so Raters is never
		// below five here (D-02, D-21).
		if sig.OutcomeScore != nil && sig.TappedHalves != nil && sig.Raters >= MinRatersForStars {
			p.StarsFromReports = StarsFromReportsText(*sig.OutcomeScore)
			p.StarsFromRatings = StarsFromRatingsText(*sig.TappedHalves, sig.Raters)
			p.StarsLabel = StarsGroupLabelParts(*sig.Score, p.StarsFromReports, p.StarsFromRatings)
		}
	}
	if self {
		p.StateLine = OwnStateLine(person.ProfilePublic, reliable)
	} else {
		p.CanFollow = reliable && !following
		muted, err := s.q.PersonMuted(ctx, sqlcgen.PersonMutedParams{MuterID: viewerID, MutedID: person.ID})
		if err != nil {
			return PersonProfile{}, fmt.Errorf("service: Profile: reading whether the viewer muted the person: %w", err)
		}
		p.Muted = muted
		if following && !muted {
			on, err := s.q.ActiveFollowNotify(ctx, sqlcgen.ActiveFollowNotifyParams{FollowerID: viewerID, FolloweeID: person.ID})
			if err != nil {
				return PersonProfile{}, fmt.Errorf("service: Profile: reading the viewer's bell: %w", err)
			}
			p.Notify = on
		}
		if s.ratings != nil {
			may, err := s.ratings.MayRate(ctx, viewerID, person.ID)
			if err != nil {
				return PersonProfile{}, fmt.Errorf("service: Profile: reading whether the viewer may rate: %w", err)
			}
			stars, _, err := s.ratings.YourRating(ctx, viewerID, person.ID)
			if err != nil {
				return PersonProfile{}, fmt.Errorf("service: Profile: reading the viewer's rating: %w", err)
			}
			p.MayRate = may
			p.YourRating = stars
		}
	}
	return p, nil
}

// MyUsername returns the caller's own username (the target of /me), or
// ErrPersonNotFound when the account has none or is gone.
func (s *PeopleService) MyUsername(ctx context.Context, accountID int64) (string, error) {
	row, err := s.q.GetPersonByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPersonNotFound
		}
		return "", fmt.Errorf("service: MyUsername: reading account %d: %w", accountID, err)
	}
	if row.Username == nil {
		return "", ErrPersonNotFound
	}
	return *row.Username, nil
}

// OwnProfileState is what the Settings page needs to draw the people rows: the
// caller's own username (empty when none is saved yet) and whether the public
// profile switch is on.
type OwnProfileState struct {
	Username      string
	ProfilePublic bool
}

// MyProfileState reads the caller's own username and switch state in one read
// (Phase 9 plan 12, D-04). An account with no username is not an error here:
// Settings still draws the rows, and turning the switch on is refused by
// SetProfilePublic with the Name or username sentence. A missing account is
// ErrPersonNotFound.
func (s *PeopleService) MyProfileState(ctx context.Context, accountID int64) (OwnProfileState, error) {
	row, err := s.q.GetPersonByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OwnProfileState{}, ErrPersonNotFound
		}
		return OwnProfileState{}, fmt.Errorf("service: MyProfileState: reading account %d: %w", accountID, err)
	}
	state := OwnProfileState{ProfilePublic: row.ProfilePublic}
	if row.Username != nil {
		state.Username = *row.Username
	}
	return state, nil
}
