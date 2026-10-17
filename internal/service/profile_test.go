package service_test

// profile_test.go proves the account profile rules of plan 03.4-01 (D-24b,
// D-27): the validation of the optional name and the home area, the precision 5
// cell the server computes, and a round trip through a real database.

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// The generated Queries value must satisfy the narrow profile querier, because
// NewRouter passes deps.Sessions to NewAccountService.
var _ service.ProfileQuerier = (*sqlcgen.Queries)(nil)

func strPtr(s string) *string { return &s }

func TestValidateProfileUpdate(t *testing.T) {
	good := service.HomeAreaInput{Name: "Indiranagar, Bengaluru", Lat: 12.9716, Lon: 77.5946}

	t.Run("a name with surrounding spaces is trimmed", func(t *testing.T) {
		v, err := service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr("  Asha  ")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !v.SetName || v.Name == nil || *v.Name != "Asha" {
			t.Fatalf("name = %+v, want Asha", v)
		}
	})

	t.Run("60 runes pass and 61 fail", func(t *testing.T) {
		if _, err := service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr(strings.Repeat("é", 60))}); err != nil {
			t.Fatalf("60 runes must pass: %v", err)
		}
		_, err := service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr(strings.Repeat("é", 61))})
		wantField(t, err, "display_name")
	})

	t.Run("a control character fails", func(t *testing.T) {
		_, err := service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr("As\x00ha")})
		wantField(t, err, "display_name")
		_, err = service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr("As\nha")})
		wantField(t, err, "display_name")
	})

	t.Run("an empty name after trimming means clear", func(t *testing.T) {
		v, err := service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr("   ")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !v.SetName || v.Name != nil {
			t.Fatalf("an empty name must clear: %+v", v)
		}
	})

	t.Run("no name means unchanged", func(t *testing.T) {
		v, err := service.ValidateProfileUpdate(service.ProfileUpdate{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.SetName || v.HomeAction != service.HomeAreaUnchanged {
			t.Fatalf("an empty update must change nothing: %+v", v)
		}
	})

	bad := map[string]service.HomeAreaInput{
		"blank name":     {Name: "   ", Lat: 12.9, Lon: 77.5},
		"long name":      {Name: strings.Repeat("a", 121), Lat: 12.9, Lon: 77.5},
		"control char":   {Name: "Indira\x07nagar", Lat: 12.9, Lon: 77.5},
		"lat too high":   {Name: "x", Lat: 90.5, Lon: 77.5},
		"lat too low":    {Name: "x", Lat: -90.5, Lon: 77.5},
		"lon too high":   {Name: "x", Lat: 12.9, Lon: 180.5},
		"lon too low":    {Name: "x", Lat: 12.9, Lon: -180.5},
		"NaN lat":        {Name: "x", Lat: math.NaN(), Lon: 77.5},
		"infinite lon":   {Name: "x", Lat: 12.9, Lon: math.Inf(1)},
		"neg infinite":   {Name: "x", Lat: math.Inf(-1), Lon: 77.5},
		"NaN lon":        {Name: "x", Lat: 12.9, Lon: math.NaN()},
		"infinite lat":   {Name: "x", Lat: math.Inf(1), Lon: 77.5},
		"neg inf lon":    {Name: "x", Lat: 12.9, Lon: math.Inf(-1)},
		"name of spaces": {Name: " \t ", Lat: 12.9, Lon: 77.5},
	}
	for label, in := range bad {
		t.Run("home area "+label, func(t *testing.T) {
			_, err := service.ValidateProfileUpdate(service.ProfileUpdate{
				HomeArea: service.HomeAreaChange{Action: service.HomeAreaSet, Input: in},
			})
			wantField(t, err, "home_area")
		})
	}

	t.Run("a valid home area yields the precision 5 cell", func(t *testing.T) {
		v, err := service.ValidateProfileUpdate(service.ProfileUpdate{
			HomeArea: service.HomeAreaChange{Action: service.HomeAreaSet, Input: good},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := geohash.EncodeWithPrecision(good.Lat, good.Lon, 5)
		if v.HomeAction != service.HomeAreaSet || v.HomeName == nil || v.HomeCell == nil {
			t.Fatalf("home area not set: %+v", v)
		}
		if len(*v.HomeCell) != 5 || *v.HomeCell != want {
			t.Fatalf("cell = %q, want %q", *v.HomeCell, want)
		}
		if *v.HomeName != "Indiranagar, Bengaluru" {
			t.Fatalf("name = %q", *v.HomeName)
		}
	})

	t.Run("clear keeps no name and no cell", func(t *testing.T) {
		v, err := service.ValidateProfileUpdate(service.ProfileUpdate{
			HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.HomeAction != service.HomeAreaClear || v.HomeName != nil || v.HomeCell != nil {
			t.Fatalf("clear must carry nothing: %+v", v)
		}
	})
}

func wantField(t *testing.T, err error, field string) {
	t.Helper()
	var ve service.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want a ValidationError on %q, got %v", field, err)
	}
	if ve.Field != field {
		t.Fatalf("error field = %q, want %q (%v)", ve.Field, field, err)
	}
	if strings.ContainsAny(ve.Message, "\u2014\u2013") {
		t.Fatalf("message must not hold a dash: %q", ve.Message)
	}
}

// TestAccountServiceProfileRoundTrip runs the service over a real database and
// skips without DATABASE_URL (NewTestDB).
func TestAccountServiceProfileRoundTrip(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	svc := service.NewAccountService(q)
	id := testutil.SeedReporterAccount(t, pool, "profile-rt@example.com")

	// Before consent: not agreed, not done, and Done is refused.
	p, err := svc.Profile(ctx, id)
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if p.TermsVersion != "" || p.FirstRunDone || p.DisplayName != nil || p.HomeAreaName != nil {
		t.Fatalf("a new account must have an empty profile: %+v", p)
	}
	if err := svc.MarkFirstRunDone(ctx, id); !errors.Is(err, service.ErrTermsNotAgreed) {
		t.Fatalf("MarkFirstRunDone before consent: want ErrTermsNotAgreed, got %v", err)
	}

	// Consent stores the current version and keeps the first agreed time.
	if err := svc.AcceptTerms(ctx, id); err != nil {
		t.Fatalf("AcceptTerms: %v", err)
	}
	var first string
	if err := pool.QueryRow(ctx, `SELECT terms_agreed_at::text FROM accounts WHERE id = $1`, id).Scan(&first); err != nil {
		t.Fatalf("reading terms_agreed_at: %v", err)
	}
	if err := svc.AcceptTerms(ctx, id); err != nil {
		t.Fatalf("second AcceptTerms: %v", err)
	}
	var second string
	if err := pool.QueryRow(ctx, `SELECT terms_agreed_at::text FROM accounts WHERE id = $1`, id).Scan(&second); err != nil {
		t.Fatalf("reading terms_agreed_at again: %v", err)
	}
	if first != second {
		t.Fatalf("a repeat AcceptTerms moved the agreed time: %s then %s", first, second)
	}
	p, _ = svc.Profile(ctx, id)
	if p.TermsVersion != service.CurrentTermsVersion {
		t.Fatalf("terms version = %q, want %q", p.TermsVersion, service.CurrentTermsVersion)
	}

	// An older stored version is replaced and restamped.
	if _, err := pool.Exec(ctx, `UPDATE accounts SET terms_version = 'old', terms_agreed_at = now() - interval '1 day' WHERE id = $1`, id); err != nil {
		t.Fatalf("aging the stored version: %v", err)
	}
	if err := svc.MarkFirstRunDone(ctx, id); !errors.Is(err, service.ErrTermsNotAgreed) {
		t.Fatalf("MarkFirstRunDone with an old version: want ErrTermsNotAgreed, got %v", err)
	}
	if err := svc.AcceptTerms(ctx, id); err != nil {
		t.Fatalf("AcceptTerms over an old version: %v", err)
	}
	var restamped bool
	if err := pool.QueryRow(ctx, `SELECT terms_agreed_at > now() - interval '1 hour' FROM accounts WHERE id = $1`, id).Scan(&restamped); err != nil || !restamped {
		t.Fatalf("a new version must be restamped: %v %v", restamped, err)
	}

	// Done succeeds after consent and keeps the first time.
	if err := svc.MarkFirstRunDone(ctx, id); err != nil {
		t.Fatalf("MarkFirstRunDone after consent: %v", err)
	}
	p, _ = svc.Profile(ctx, id)
	if !p.FirstRunDone {
		t.Fatalf("FirstRunDone must be true: %+v", p)
	}

	// A name alone leaves the home area alone, and the other way round.
	p, err = svc.UpdateProfile(ctx, id, service.ProfileUpdate{DisplayName: strPtr(" Asha ")})
	if err != nil {
		t.Fatalf("UpdateProfile name: %v", err)
	}
	if p.DisplayName == nil || *p.DisplayName != "Asha" || p.HomeAreaName != nil {
		t.Fatalf("name only: %+v", p)
	}
	area := service.HomeAreaInput{Name: "Indiranagar", Lat: 12.9784, Lon: 77.6408}
	p, err = svc.UpdateProfile(ctx, id, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaSet, Input: area}})
	if err != nil {
		t.Fatalf("UpdateProfile home area: %v", err)
	}
	if p.DisplayName == nil || *p.DisplayName != "Asha" || p.HomeAreaName == nil || *p.HomeAreaName != "Indiranagar" {
		t.Fatalf("home area only: %+v", p)
	}

	// The stored row holds the place name and the cell, never the point.
	var name, cell string
	if err := pool.QueryRow(ctx, `SELECT home_area_name, home_area_cell FROM accounts WHERE id = $1`, id).Scan(&name, &cell); err != nil {
		t.Fatalf("reading the stored home area: %v", err)
	}
	if name != "Indiranagar" {
		t.Fatalf("stored name = %q", name)
	}
	if want := geohash.EncodeWithPrecision(area.Lat, area.Lon, service.HomeAreaCellPrecision); cell != want || len(cell) != 5 {
		t.Fatalf("stored cell = %q, want %q", cell, want)
	}
	// The table has no place to keep a point at all.
	var pointColumns int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'accounts'
		AND (column_name ILIKE '%lat%' OR column_name ILIKE '%lon%')`).Scan(&pointColumns); err != nil || pointColumns != 0 {
		t.Fatalf("accounts must hold no latitude or longitude column: %d %v", pointColumns, err)
	}

	// An empty name clears the name only.
	p, err = svc.UpdateProfile(ctx, id, service.ProfileUpdate{DisplayName: strPtr("")})
	if err != nil {
		t.Fatalf("UpdateProfile clear name: %v", err)
	}
	if p.DisplayName != nil || p.HomeAreaName == nil {
		t.Fatalf("clearing the name must keep the home area: %+v", p)
	}

	// Removing the home area clears both columns.
	p, err = svc.UpdateProfile(ctx, id, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}})
	if err != nil {
		t.Fatalf("UpdateProfile clear home area: %v", err)
	}
	if p.HomeAreaName != nil {
		t.Fatalf("home area name not cleared: %+v", p)
	}
	var cellIsNull bool
	if err := pool.QueryRow(ctx, `SELECT home_area_cell IS NULL AND home_area_name IS NULL FROM accounts WHERE id = $1`, id).Scan(&cellIsNull); err != nil || !cellIsNull {
		t.Fatalf("name and cell must clear together: %v %v", cellIsNull, err)
	}

	// An invalid update changes nothing.
	if _, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{DisplayName: strPtr(strings.Repeat("a", 61))}); err == nil {
		t.Fatal("a 61 character name must be refused")
	}
}

// TestDisplayNameFormatCharacters proves code review part 1 WR-02: a name that
// is invisible, or holds a Unicode format character (category Cf), is refused,
// and a name needs at least one letter or digit. The characters are written as
// code points, never as raw invisible characters.
func TestDisplayNameFormatCharacters(t *testing.T) {
	hidden := func(points ...rune) string { return string(points) }
	refused := map[string]string{
		"lone zero width space":            hidden(0x200b),
		"zero width joiner and non joiner": hidden(0x200d, 0x200c),
		"right to left override inside":    "a" + hidden(0x202e) + "b",
		"byte order mark before a name":    hidden(0xfeff) + "Ravi",
		"isolates between spaces":          " " + hidden(0x2066) + " ",
		"isolate pair around a name":       hidden(0x2066) + "Ravi" + hidden(0x2069),
		"zero width space inside a name":   "Ra" + hidden(0x200b) + "vi",
		"only dots":                        "...",
		"only punctuation and spaces":      " - . , ",
		"only an emoji":                    "\U0001F600",
	}
	for label, name := range refused {
		t.Run("refused: "+label, func(t *testing.T) {
			_, err := service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr(name)})
			wantField(t, err, "display_name")
			var ve service.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want a ValidationError, got %v", err)
			}
			if !strings.Contains(ve.Message, "letter") || !strings.Contains(ve.Message, "hidden") {
				t.Fatalf("the message must say what the rule is (a letter or number, no hidden characters): %q", ve.Message)
			}
		})
	}

	accepted := []string{"Ravi Kumar", string([]rune{0x0930, 0x0935, 0x093f}), "Ravi 2", "42", "A.", "Anne-Marie O'Neil"}
	for _, name := range accepted {
		t.Run("accepted: "+name, func(t *testing.T) {
			v, err := service.ValidateProfileUpdate(service.ProfileUpdate{DisplayName: strPtr(name)})
			if err != nil {
				t.Fatalf("%q must be accepted: %v", name, err)
			}
			if v.Name == nil || *v.Name != name {
				t.Fatalf("name = %v, want %q", v.Name, name)
			}
		})
	}

	// The format character refusal applies to the home area name too, but the
	// letter or digit rule does not: its other rules stay as they were.
	t.Run("home area name with a format character is refused", func(t *testing.T) {
		for _, name := range []string{hidden(0x200b), "Indira" + hidden(0x202e) + "nagar", hidden(0xfeff) + "Indiranagar"} {
			_, err := service.ValidateProfileUpdate(service.ProfileUpdate{
				HomeArea: service.HomeAreaChange{Action: service.HomeAreaSet, Input: service.HomeAreaInput{Name: name, Lat: 12.9, Lon: 77.5}},
			})
			wantField(t, err, "home_area")
		}
	})
	t.Run("home area name keeps its rules apart from format characters", func(t *testing.T) {
		v, err := service.ValidateProfileUpdate(service.ProfileUpdate{
			HomeArea: service.HomeAreaChange{Action: service.HomeAreaSet, Input: service.HomeAreaInput{Name: "...", Lat: 12.9, Lon: 77.5}},
		})
		if err != nil || v.HomeName == nil || *v.HomeName != "..." {
			t.Fatalf("the home area name has no letter rule: %+v %v", v, err)
		}
	})
}

// TestUpdateProfileIsAllOrNothing proves code review part 1 WR-03: an update
// that changes the name and the home area writes both or neither. A trigger on
// the disposable test database makes the home area part fail for one marker
// place name, the way a dropped connection or a timeout would.
func TestUpdateProfileIsAllOrNothing(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	svc := service.NewAccountService(sqlcgen.New(pool))
	id := testutil.SeedReporterAccount(t, pool, "profile-atomic@example.com")

	if _, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{
		DisplayName: strPtr("Before"),
		HomeArea:    service.HomeAreaChange{Action: service.HomeAreaSet, Input: service.HomeAreaInput{Name: "Old place", Lat: 12.9784, Lon: 77.6408}},
	}); err != nil {
		t.Fatalf("setting the starting profile: %v", err)
	}

	const marker = "Forced failure place"
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION test_fail_home_area() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.home_area_name = '`+marker+`' THEN
				RAISE EXCEPTION 'forced home area failure';
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER test_fail_home_area BEFORE UPDATE ON accounts
			FOR EACH ROW EXECUTE FUNCTION test_fail_home_area();`); err != nil {
		t.Fatalf("installing the failure trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_fail_home_area ON accounts; DROP FUNCTION IF EXISTS test_fail_home_area()`)
	})

	_, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{
		DisplayName: strPtr("After"),
		HomeArea:    service.HomeAreaChange{Action: service.HomeAreaSet, Input: service.HomeAreaInput{Name: marker, Lat: 13.0827, Lon: 80.2707}},
	})
	if err == nil {
		t.Fatal("the forced home area failure must surface as an error")
	}
	var ve service.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("a database failure is not a validation error: %v", err)
	}

	var name, area string
	if err := pool.QueryRow(ctx, `SELECT display_name, home_area_name FROM accounts WHERE id = $1`, id).Scan(&name, &area); err != nil {
		t.Fatalf("reading the stored profile: %v", err)
	}
	if name != "Before" || area != "Old place" {
		t.Fatalf("a failed save must change neither part: name %q, home area %q", name, area)
	}

	// With the failure gone the same kind of update lands whole.
	p, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{
		DisplayName: strPtr("After"),
		HomeArea:    service.HomeAreaChange{Action: service.HomeAreaSet, Input: service.HomeAreaInput{Name: "New place", Lat: 13.0827, Lon: 80.2707}},
	})
	if err != nil {
		t.Fatalf("a working save: %v", err)
	}
	if p.DisplayName == nil || *p.DisplayName != "After" || p.HomeAreaName == nil || *p.HomeAreaName != "New place" {
		t.Fatalf("both parts must be stored: %+v", p)
	}
}
