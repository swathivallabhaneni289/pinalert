// profile.go holds the account profile rules of Phase 3.4 (D-24b, D-27): the
// consent record, the optional name, the home area and the first-time flow
// flag. Every rule lives here; the handlers only decode and encode.
//
// The home area keeps a place name and a geohash cell of precision 5, computed
// here from the picked point. The point itself is never stored. The cell is
// coarse (about 5 km). The name is the place name the person picked, kept as
// sent after the checks below; the home area helper line asks for a town or an
// area, but nothing here checks the granularity of the name, so a person who
// picks a building or a street name has that name stored next to the cell.

package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// CurrentTermsVersion names the version of the Disclaimer and Policy text a
// person agrees to in the first-time flow. It must change whenever that text
// changes, so everyone is asked again for the new text (D-27). It is the day
// the Policy was last updated in YYYY-MM-DD form and must equal the "Last
// updated" date the Policy shows (TestTermsVersionFollowsPolicyDate).
const CurrentTermsVersion = "2026-10-17"

// HomeAreaCellPrecision is the geohash precision of the stored home area cell.
// Five characters is a cell of about 5 km, the same grid the project uses for
// its diversity weighting, and Phase 3.3 compares these cells.
const HomeAreaCellPrecision = 5

// DisplayNameMaxRunes and HomeAreaNameMaxRunes match the CHECK constraints of
// migration 00008.
const (
	DisplayNameMaxRunes  = 60
	HomeAreaNameMaxRunes = 120
)

// ErrTermsNotAgreed is returned by MarkFirstRunDone when the account has not
// agreed to CurrentTermsVersion: finishing the flow without consent would let
// the onboarding redirect be skipped.
var ErrTermsNotAgreed = errors.New("service: the current terms are not agreed to")

// TermsNotAgreedMessage is the plain sentence shown for ErrTermsNotAgreed.
const TermsNotAgreedMessage = "Agree to the terms first."

// HomeAreaAction says what an update does to the home area.
type HomeAreaAction int

const (
	// HomeAreaUnchanged leaves the stored home area as it is. It is the zero
	// value, so an update that says nothing about the home area changes nothing.
	HomeAreaUnchanged HomeAreaAction = iota
	// HomeAreaClear removes the name and the cell together.
	HomeAreaClear
	// HomeAreaSet stores the place name and the cell computed from the point.
	HomeAreaSet
)

// HomeAreaInput is the picked place as the caller sent it. The point is used
// once to compute the cell and is then discarded.
type HomeAreaInput struct {
	Name string
	Lat  float64
	Lon  float64
}

// HomeAreaChange is the three state home area part of an update.
type HomeAreaChange struct {
	Action HomeAreaAction
	Input  HomeAreaInput
}

// ProfileUpdate is one edit of the optional rows. A nil DisplayName leaves the
// name alone; an empty one after trimming clears it.
//
// Reserved word screening of names is Phase 3.2's work (03.2 D-19). Until then
// the name is shown to its owner only, and html/template escapes it.
type ProfileUpdate struct {
	DisplayName *string
	HomeArea    HomeAreaChange
}

// ValidatedProfile is a ProfileUpdate after validation: trimmed text and the
// cell. Name nil with SetName true means clear; HomeName and HomeCell are nil
// together when HomeAction is HomeAreaClear.
type ValidatedProfile struct {
	SetName    bool
	Name       *string
	HomeAction HomeAreaAction
	HomeName   *string
	HomeCell   *string
}

// AccountProfile is what the service reads back about one account.
type AccountProfile struct {
	DisplayName  *string
	HomeAreaName *string
	TermsVersion string
	FirstRunDone bool
}

// cleanText trims text and reports whether it is free of hidden characters:
// control characters (category Cc) and format characters (category Cf), which
// cover the zero width space, joiner and non joiner, the bidi embedding,
// override and isolate characters and the byte order mark. None of them has a
// place in a name that other people will read.
func cleanText(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return s, false
		}
	}
	return s, true
}

// hasLetterOrDigit reports whether s holds at least one letter or digit, so a
// name made only of spaces, dots and symbols is not a name.
func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// DisplayNameRuleMessage is the sentence shown when a name breaks the
// character rule: it names what the check really does.
const DisplayNameRuleMessage = "Use at least one letter or number in your name, and no hidden characters."

// ValidateProfileUpdate checks one update and returns the values to store.
// Nothing is written here; a failure names the field the caller sent.
func ValidateProfileUpdate(u ProfileUpdate) (ValidatedProfile, error) {
	var v ValidatedProfile

	if u.DisplayName != nil {
		name, ok := cleanText(*u.DisplayName)
		if !ok || (name != "" && !hasLetterOrDigit(name)) {
			return v, ValidationError{Field: "display_name", Message: DisplayNameRuleMessage}
		}
		if utf8.RuneCountInString(name) > DisplayNameMaxRunes {
			return v, ValidationError{Field: "display_name", Message: fmt.Sprintf("Keep your name to %d characters or fewer.", DisplayNameMaxRunes)}
		}
		v.SetName = true
		if name != "" {
			v.Name = &name
		}
	}

	switch u.HomeArea.Action {
	case HomeAreaUnchanged:
		// Nothing to do.
	case HomeAreaClear:
		v.HomeAction = HomeAreaClear
	case HomeAreaSet:
		in := u.HomeArea.Input
		name, ok := cleanText(in.Name)
		switch {
		case !ok || name == "":
			return v, ValidationError{Field: "home_area", Message: "Pick a place for your home area."}
		case utf8.RuneCountInString(name) > HomeAreaNameMaxRunes:
			return v, ValidationError{Field: "home_area", Message: fmt.Sprintf("Keep the place name to %d characters or fewer.", HomeAreaNameMaxRunes)}
		case !finiteIn(in.Lat, -90, 90) || !finiteIn(in.Lon, -180, 180):
			return v, ValidationError{Field: "home_area", Message: "That place has no usable position. Pick another."}
		}
		cell := geohash.EncodeWithPrecision(in.Lat, in.Lon, HomeAreaCellPrecision)
		v.HomeAction = HomeAreaSet
		v.HomeName = &name
		v.HomeCell = &cell
	default:
		return v, ValidationError{Field: "home_area", Message: "Say whether to keep, remove or save the home area."}
	}
	return v, nil
}

// finiteIn reports whether x is a real number inside [lo, hi].
func finiteIn(x, lo, hi float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= lo && x <= hi
}

// ProfileQuerier is the part of the generated queries AccountService uses.
type ProfileQuerier interface {
	AcceptTerms(ctx context.Context, arg sqlcgen.AcceptTermsParams) error
	MarkFirstRunDone(ctx context.Context, id int64) error
	UpdateAccountProfile(ctx context.Context, arg sqlcgen.UpdateAccountProfileParams) error
	GetAccountProfile(ctx context.Context, id int64) (sqlcgen.GetAccountProfileRow, error)
}

// AccountService reads and writes the caller's own profile rows. Every method
// takes the account id the gate resolved, never an id from a request body.
type AccountService struct {
	q ProfileQuerier
}

// NewAccountService stores the querier. It never dereferences it, so a router
// built with a nil sessions store still constructs.
func NewAccountService(q ProfileQuerier) *AccountService {
	return &AccountService{q: q}
}

// Profile reads one account's profile rows and onboarding state.
func (s *AccountService) Profile(ctx context.Context, accountID int64) (AccountProfile, error) {
	row, err := s.q.GetAccountProfile(ctx, accountID)
	if err != nil {
		return AccountProfile{}, fmt.Errorf("service: reading account profile: %w", err)
	}
	p := AccountProfile{
		DisplayName:  row.DisplayName,
		HomeAreaName: row.HomeAreaName,
		FirstRunDone: row.FirstRunDoneAt.Valid,
	}
	if row.TermsVersion != nil {
		p.TermsVersion = *row.TermsVersion
	}
	return p, nil
}

// AcceptTerms records agreement to CurrentTermsVersion. A repeat call for the
// same version keeps the first agreed time.
func (s *AccountService) AcceptTerms(ctx context.Context, accountID int64) error {
	err := s.q.AcceptTerms(ctx, sqlcgen.AcceptTermsParams{TermsVersion: CurrentTermsVersion, ID: accountID})
	if err != nil {
		return fmt.Errorf("service: recording terms agreement: %w", err)
	}
	return nil
}

// MarkFirstRunDone records that the first-time flow was finished. It refuses
// with ErrTermsNotAgreed unless the account agreed to the current version.
func (s *AccountService) MarkFirstRunDone(ctx context.Context, accountID int64) error {
	p, err := s.Profile(ctx, accountID)
	if err != nil {
		return err
	}
	if p.TermsVersion != CurrentTermsVersion {
		return ErrTermsNotAgreed
	}
	if err := s.q.MarkFirstRunDone(ctx, accountID); err != nil {
		return fmt.Errorf("service: marking first run done: %w", err)
	}
	return nil
}

// UpdateProfile validates the update and then writes the rows it names. The
// whole update is validated before any write, so a refusal changes nothing.
// The name and the home area are written by one statement, so a database
// failure changes neither. The answer is the profile as stored afterwards.
func (s *AccountService) UpdateProfile(ctx context.Context, accountID int64, u ProfileUpdate) (AccountProfile, error) {
	v, err := ValidateProfileUpdate(u)
	if err != nil {
		return AccountProfile{}, err
	}
	setHome := v.HomeAction != HomeAreaUnchanged
	if v.SetName || setHome {
		// Clear leaves the home name and cell nil, which writes NULL to both
		// columns together.
		err := s.q.UpdateAccountProfile(ctx, sqlcgen.UpdateAccountProfileParams{
			SetName:      v.SetName,
			DisplayName:  v.Name,
			SetHomeArea:  setHome,
			HomeAreaName: v.HomeName,
			HomeAreaCell: v.HomeCell,
			ID:           accountID,
		})
		if err != nil {
			return AccountProfile{}, fmt.Errorf("service: saving profile: %w", err)
		}
	}
	return s.Profile(ctx, accountID)
}
