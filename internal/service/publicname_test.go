package service_test

// publicname_test.go proves the Name checks of plan 09-04 (research gap G1):
// the reserved word fold, the screen outcomes and that a Name edit is screened
// while the public profile is on. Nothing here needs a database.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

func TestNameReserved(t *testing.T) {
	// Telugu letters written as code points so this file stays ASCII.
	telugu := string([]rune{0x0C30, 0x0C35, 0x0C3F, 0x0C15, 0x0C41, 0x0C2E, 0x0C3E, 0x0C30, 0x0C4D})
	cases := []struct {
		name string
		want bool
	}{
		{"NDMA Control Room", true},
		{"Police", true},
		{"Govt Office", true},
		{"IMD Hyderabad", true},
		{"N.D.M.A", true},
		{"  Hyd City Police  ", true},
		{"Ravi Kumar", false},
		{"Imdad Ali", false},
		{"Asha", false},
		{telugu, false},
		{"", false},
		{"   ", false},
	}
	for _, tc := range cases {
		if got := service.NameReserved(tc.name); got != tc.want {
			t.Errorf("NameReserved(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestScreenPublicName(t *testing.T) {
	ctx := context.Background()

	t.Run("reserved_never_calls_the_screener", func(t *testing.T) {
		fs := screenerWith(map[string]float64{})
		if got := service.ScreenPublicName(ctx, fs, "NDMA Control Room"); got != service.PublicNameRefused {
			t.Fatalf("got %v, want refused", got)
		}
		if fs.callCount() != 0 {
			t.Fatalf("screener called %d times for a reserved name, want 0", fs.callCount())
		}
	})
	t.Run("clear", func(t *testing.T) {
		fs := screenerWith(map[string]float64{"hate": 0.0})
		if got := service.ScreenPublicName(ctx, fs, "Ravi"); got != service.PublicNameClear {
			t.Fatalf("got %v, want clear (a one word Name is not gibberish)", got)
		}
		if fs.callCount() != 1 {
			t.Fatalf("calls %d, want 1", fs.callCount())
		}
	})
	t.Run("borderline_refuses", func(t *testing.T) {
		fs := screenerWith(map[string]float64{"hate": service.BorderlineScoreThreshold})
		if got := service.ScreenPublicName(ctx, fs, "Ravi Kumar"); got != service.PublicNameRefused {
			t.Fatalf("got %v, want refused", got)
		}
	})
	t.Run("rejected_refuses", func(t *testing.T) {
		fs := screenerWith(map[string]float64{"hate": service.RejectScoreThreshold})
		if got := service.ScreenPublicName(ctx, fs, "Ravi Kumar"); got != service.PublicNameRefused {
			t.Fatalf("got %v, want refused", got)
		}
	})
	t.Run("error_is_unfinished", func(t *testing.T) {
		fs := &fakeTextScreener{err: errors.New("provider down")}
		if got := service.ScreenPublicName(ctx, fs, "Ravi Kumar"); got != service.PublicNameUnfinished {
			t.Fatalf("got %v, want unfinished", got)
		}
	})
	t.Run("timeout_is_unfinished", func(t *testing.T) {
		fs := &fakeTextScreener{block: true}
		short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		if got := service.ScreenPublicName(short, fs, "Ravi Kumar"); got != service.PublicNameUnfinished {
			t.Fatalf("got %v, want unfinished", got)
		}
	})
	t.Run("nil_screener_passes_a_plain_name", func(t *testing.T) {
		if got := service.ScreenPublicName(ctx, nil, "Ravi Kumar"); got != service.PublicNameClear {
			t.Fatalf("got %v, want clear", got)
		}
		if got := service.ScreenPublicName(ctx, nil, "Police"); got != service.PublicNameRefused {
			t.Fatalf("a nil screener must still refuse a reserved name, got %v", got)
		}
	})
}

// nameEditQuerier is a ProfileQuerier that holds one stored Name. The embedded
// interface is nil: a method the test does not expect panics.
type nameEditQuerier struct {
	service.ProfileQuerier
	name   *string
	writes int
	last   *string
}

func (f *nameEditQuerier) GetAccountProfile(context.Context, int64) (sqlcgen.GetAccountProfileRow, error) {
	return sqlcgen.GetAccountProfileRow{DisplayName: f.name}, nil
}

func (f *nameEditQuerier) UpdateAccountProfile(_ context.Context, arg sqlcgen.UpdateAccountProfileParams) error {
	f.writes++
	f.last = arg.DisplayName
	f.name = arg.DisplayName
	return nil
}

// profileReader is a PublicProfileReader with one fixed answer.
type profileReader struct {
	row sqlcgen.GetPersonByIDRow
	err error
}

func (r profileReader) GetPersonByID(context.Context, int64) (sqlcgen.GetPersonByIDRow, error) {
	return r.row, r.err
}

func TestPublicNameEditWhilePublic(t *testing.T) {
	ctx := context.Background()
	stored := "Old Name"
	edit := func(name string) service.ProfileUpdate { return service.ProfileUpdate{DisplayName: &name} }
	reader := func(public bool) profileReader {
		return profileReader{row: sqlcgen.GetPersonByIDRow{ID: 1, DisplayName: &stored, ProfilePublic: public}}
	}
	fieldErr := func(t *testing.T, err error, msg string) {
		t.Helper()
		var ve service.ValidationError
		if !errors.As(err, &ve) || ve.Field != "display_name" || ve.Message != msg {
			t.Fatalf("got %v, want a display_name ValidationError %q", err, msg)
		}
	}

	t.Run("refused_name_writes_nothing", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		fs := screenerWith(map[string]float64{"hate": service.RejectScoreThreshold})
		svc := service.NewAccountService(q, service.WithAccountScreener(fs), service.WithPublicProfileReader(reader(true)))
		_, err := svc.UpdateProfile(ctx, 1, edit("Ravi Kumar"))
		fieldErr(t, err, service.PublicNameEditRefusedMessage)
		if q.writes != 0 {
			t.Fatalf("a refused name wrote %d times, want 0", q.writes)
		}
	})
	t.Run("reserved_name_refused_without_a_call", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		fs := screenerWith(map[string]float64{})
		svc := service.NewAccountService(q, service.WithAccountScreener(fs), service.WithPublicProfileReader(reader(true)))
		_, err := svc.UpdateProfile(ctx, 1, edit("NDMA Control Room"))
		fieldErr(t, err, service.PublicNameEditRefusedMessage)
		if fs.callCount() != 0 || q.writes != 0 {
			t.Fatalf("calls %d writes %d, want 0 and 0", fs.callCount(), q.writes)
		}
	})
	t.Run("unfinished_check_writes_nothing", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		fs := &fakeTextScreener{err: errors.New("provider down")}
		svc := service.NewAccountService(q, service.WithAccountScreener(fs), service.WithPublicProfileReader(reader(true)))
		_, err := svc.UpdateProfile(ctx, 1, edit("Ravi Kumar"))
		fieldErr(t, err, service.PublicNameUnfinishedMessage)
		if q.writes != 0 {
			t.Fatalf("an unfinished check wrote %d times, want 0", q.writes)
		}
	})
	t.Run("a_read_error_fails_closed", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		r := profileReader{err: errors.New("db down")}
		svc := service.NewAccountService(q, service.WithPublicProfileReader(r))
		if _, err := svc.UpdateProfile(ctx, 1, edit("Ravi Kumar")); err == nil || q.writes != 0 {
			t.Fatalf("err %v writes %d, want an error and no write", err, q.writes)
		}
	})
	t.Run("clear_name_is_saved", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		fs := screenerWith(map[string]float64{})
		svc := service.NewAccountService(q, service.WithAccountScreener(fs), service.WithPublicProfileReader(reader(true)))
		if _, err := svc.UpdateProfile(ctx, 1, edit("Ravi Kumar")); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if q.writes != 1 || q.last == nil || *q.last != "Ravi Kumar" {
			t.Fatalf("writes %d last %v, want one write of Ravi Kumar", q.writes, q.last)
		}
	})
	t.Run("profile_off_is_not_screened", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		fs := screenerWith(map[string]float64{"hate": service.RejectScoreThreshold})
		svc := service.NewAccountService(q, service.WithAccountScreener(fs), service.WithPublicProfileReader(reader(false)))
		if _, err := svc.UpdateProfile(ctx, 1, edit("NDMA Control Room")); err != nil {
			t.Fatalf("UpdateProfile with the profile off: %v", err)
		}
		if fs.callCount() != 0 || q.writes != 1 {
			t.Fatalf("calls %d writes %d, want 0 and 1", fs.callCount(), q.writes)
		}
	})
	t.Run("no_reader_is_not_screened", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		svc := service.NewAccountService(q)
		if _, err := svc.UpdateProfile(ctx, 1, edit("NDMA Control Room")); err != nil || q.writes != 1 {
			t.Fatalf("err %v writes %d, want today's behaviour (saved)", err, q.writes)
		}
	})
	t.Run("same_name_is_not_screened_again", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		fs := screenerWith(map[string]float64{"hate": service.RejectScoreThreshold})
		svc := service.NewAccountService(q, service.WithAccountScreener(fs), service.WithPublicProfileReader(reader(true)))
		if _, err := svc.UpdateProfile(ctx, 1, edit(stored)); err != nil {
			t.Fatalf("UpdateProfile with the same name: %v", err)
		}
		if fs.callCount() != 0 {
			t.Fatalf("screener called %d times for an unchanged name, want 0", fs.callCount())
		}
	})
	t.Run("a_deleted_account_is_not_screened", func(t *testing.T) {
		q := &nameEditQuerier{name: &stored}
		fs := screenerWith(map[string]float64{})
		svc := service.NewAccountService(q, service.WithAccountScreener(fs), service.WithPublicProfileReader(profileReader{err: pgx.ErrNoRows}))
		if _, err := svc.UpdateProfile(ctx, 1, edit("Ravi Kumar")); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if fs.callCount() != 0 {
			t.Fatalf("screener called %d times, want 0", fs.callCount())
		}
	})
}

// The refusal sentences carry no dash and no promise of a sound.
func TestPublicNameSentencesAreClean(t *testing.T) {
	for _, s := range []string{
		service.PublicNameRefusedMessage, service.PublicUsernameRefusedMessage,
		service.PublicNameUnfinishedMessage, service.PublicNameEditRefusedMessage,
	} {
		if strings.ContainsAny(s, string([]rune{0x2013, 0x2014})) {
			t.Errorf("sentence %q has a dash", s)
		}
	}
}
