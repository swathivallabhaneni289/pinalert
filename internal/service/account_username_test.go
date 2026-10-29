package service_test

// account_username_test.go proves the username save and read rules of Phase
// 7.1 plan 06 (D-07, D-16): the shape and reserved rules run before anything
// else, the screening outcome maps to clear, pending or a refusal, a lost race
// and the rename gap are answered in the plain sentences, the live check never
// calls the screening provider, and Phases 8 and 9 have a batched read. The
// unit cases use fakeProfileQ; the race and the batched read run on a real
// database and skip without DATABASE_URL.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

var usernameNow = time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)

func strp(s string) *string { return &s }

func usernameService(f *fakeProfileQ, screener service.TextScreener) *service.AccountService {
	opts := []service.AccountOption{service.WithAccountClock(func() time.Time { return usernameNow })}
	if screener != nil {
		opts = append(opts, service.WithAccountScreener(screener))
	}
	return service.NewAccountService(f, opts...)
}

func wantMessage(t *testing.T, err error, field, message string) {
	t.Helper()
	wantField(t, err, field)
	var ve service.ValidationError
	if errors.As(err, &ve) && ve.Message != message {
		t.Fatalf("message = %q, want %q", ve.Message, message)
	}
}

// TestUsernameSaveRules: a typed name is normalised and saved clear; a bad
// shape or a reserved name is refused with its sentence before the screener is
// asked and before any write.
func TestUsernameSaveRules(t *testing.T) {
	ctx := context.Background()

	t.Run("normalised_and_saved_clear", func(t *testing.T) {
		f := &fakeProfileQ{}
		fs := screenerWith(map[string]float64{})
		p, err := usernameService(f, fs).UpdateProfile(ctx, 7, service.ProfileUpdate{Username: strp("  @Ravi.K ")})
		if err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.usernameSets) != 1 || f.usernameSets[0].Username != "ravi.k" || f.usernameSets[0].UsernameScreen != "clear" {
			t.Fatalf("writes = %+v, want one write of ravi.k clear", f.usernameSets)
		}
		if f.usernameSets[0].ID != 7 {
			t.Errorf("write was for account %d, want 7", f.usernameSets[0].ID)
		}
		if p.Username == nil || *p.Username != "ravi.k" || p.UsernameScreen == nil || *p.UsernameScreen != "clear" {
			t.Errorf("profile username = %v / %v, want ravi.k / clear", p.Username, p.UsernameScreen)
		}
	})

	t.Run("bad_shape_and_reserved_stop_early", func(t *testing.T) {
		cases := []struct {
			typed   string
			message string
		}{
			{"ravi k", service.UsernameRuleMessage},
			{"", service.UsernameRuleMessage},
			{"ab", service.UsernameRuleMessage},
			{"p0lice", service.UsernameReservedMessage},
			{"ndma.india", service.UsernameReservedMessage},
		}
		for _, c := range cases {
			f := &fakeProfileQ{}
			fs := screenerWith(map[string]float64{})
			name := "Pat"
			_, err := usernameService(f, fs).UpdateProfile(ctx, 7, service.ProfileUpdate{
				Username: strp(c.typed), DisplayName: &name,
			})
			wantMessage(t, err, "username", c.message)
			if fs.callCount() != 0 {
				t.Errorf("%q: the screener was called %d times, want 0", c.typed, fs.callCount())
			}
			if len(f.usernameSets) != 0 || len(f.profiles) != 0 {
				t.Errorf("%q: wrote %d usernames and %d profiles, want none", c.typed, len(f.usernameSets), len(f.profiles))
			}
		}
	})

	t.Run("same_name_writes_nothing", func(t *testing.T) {
		f := &fakeProfileQ{profile: sqlcgen.GetAccountProfileRow{Username: strp("ravi.k"), UsernameScreen: strp("clear")}}
		fs := screenerWith(map[string]float64{})
		if _, err := usernameService(f, fs).UpdateProfile(ctx, 7, service.ProfileUpdate{Username: strp("Ravi.K")}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.usernameSets) != 0 || fs.callCount() != 0 {
			t.Errorf("same name wrote %d times and screened %d times, want none", len(f.usernameSets), fs.callCount())
		}
	})

	t.Run("a_username_error_leaves_name_and_home_unsaved", func(t *testing.T) {
		f := &fakeProfileQ{}
		name := "Pat"
		_, err := usernameService(f, nil).UpdateProfile(ctx, 7, service.ProfileUpdate{
			Username:    strp("p0lice"),
			DisplayName: &name,
			HomeArea: service.HomeAreaChange{
				Action: service.HomeAreaSet,
				Input:  service.HomeAreaInput{Name: "Indiranagar, Bengaluru", Lat: 12.97, Lon: 77.64},
			},
		})
		wantField(t, err, "username")
		if len(f.profiles) != 0 {
			t.Errorf("the name and home area were written (%d) after a username error", len(f.profiles))
		}

		// A lost race on a legal name also leaves them unsaved.
		g := &fakeProfileQ{usernameErr: &pgconn.PgError{Code: "23505"}}
		_, err = usernameService(g, nil).UpdateProfile(ctx, 7, service.ProfileUpdate{Username: strp("taken_one"), DisplayName: &name})
		wantMessage(t, err, "username", service.UsernameTakenMessage("taken_one"))
		if len(g.profiles) != 0 {
			t.Errorf("the name was written (%d) after a lost race", len(g.profiles))
		}
	})

	t.Run("username_then_name_in_one_request", func(t *testing.T) {
		f := &fakeProfileQ{}
		name := "Ravi Kumar"
		if _, err := usernameService(f, nil).UpdateProfile(ctx, 7, service.ProfileUpdate{Username: strp("ravi.k"), DisplayName: &name}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.usernameSets) != 1 || len(f.profiles) != 1 {
			t.Errorf("wrote %d usernames and %d profiles, want 1 and 1", len(f.usernameSets), len(f.profiles))
		}
	})
}

// TestUsernameSaveScreening is the D-16 table: clear stays clear, borderline
// and unfinished (a provider error, a stuck provider) are saved pending with no
// error, a rejected name is refused and nothing is written, a nil screener is
// clear, and the provider sees the name with separators turned into spaces.
func TestUsernameSaveScreening(t *testing.T) {
	ctx := context.Background()
	reject := map[string]float64{"hate": service.RejectScoreThreshold}
	borderline := map[string]float64{"hate": service.BorderlineScoreThreshold}

	type tc struct {
		name       string
		screener   func() *fakeTextScreener
		wantScreen string
		wantErr    error
	}
	cases := []tc{
		{"clear", func() *fakeTextScreener { return screenerWith(map[string]float64{}) }, "clear", nil},
		{"borderline_is_pending", func() *fakeTextScreener { return screenerWith(borderline) }, "pending", nil},
		{"provider_error_is_pending", func() *fakeTextScreener {
			return &fakeTextScreener{err: &moderation.CallError{Transient: true}}
		}, "pending", nil},
		{"plain_error_is_pending", func() *fakeTextScreener { return &fakeTextScreener{err: errors.New("down")} }, "pending", nil},
		{"rejected_is_refused", func() *fakeTextScreener { return screenerWith(reject) }, "", service.ErrUsernameRefused},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			captureLog(t)
			f := &fakeProfileQ{}
			fs := c.screener()
			_, err := usernameService(f, fs).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("ravi_k.123")})
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if fs.callCount() != 1 || fs.texts[0] != "ravi k 123" {
				t.Fatalf("screener calls %d, texts %q; want one call with %q", fs.callCount(), fs.texts, "ravi k 123")
			}
			if c.wantErr != nil {
				if len(f.usernameSets) != 0 {
					t.Fatalf("a refused name was written: %+v", f.usernameSets)
				}
				return
			}
			if len(f.usernameSets) != 1 || f.usernameSets[0].UsernameScreen != c.wantScreen || f.usernameSets[0].Username != "ravi_k.123" {
				t.Fatalf("writes = %+v, want ravi_k.123 with screen %q", f.usernameSets, c.wantScreen)
			}
		})
	}

	t.Run("nil_screener_is_clear", func(t *testing.T) {
		f := &fakeProfileQ{}
		if _, err := usernameService(f, nil).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("ravi.k")}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.usernameSets) != 1 || f.usernameSets[0].UsernameScreen != "clear" {
			t.Fatalf("writes = %+v, want clear", f.usernameSets)
		}
	})

	t.Run("stuck_provider_is_pending_inside_the_timeout", func(t *testing.T) {
		captureLog(t)
		f := &fakeProfileQ{}
		fs := &fakeTextScreener{block: true}
		start := time.Now()
		if _, err := usernameService(f, fs).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("ravi.k")}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if took := time.Since(start); took > service.InlineScreenTimeout+time.Second {
			t.Errorf("a stuck provider held the save for %v", took)
		}
		if len(f.usernameSets) != 1 || f.usernameSets[0].UsernameScreen != "pending" {
			t.Fatalf("writes = %+v, want pending", f.usernameSets)
		}
	})

	t.Run("a_single_word_is_not_gibberish", func(t *testing.T) {
		// ScreenAbuse, not CheckText: the gibberish prefilter must not refuse a
		// plain one word name (research Pitfall 1).
		f := &fakeProfileQ{}
		fs := screenerWith(map[string]float64{})
		if _, err := usernameService(f, fs).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("zzzzzzzzzzzz")}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if fs.callCount() != 1 {
			t.Errorf("screener calls = %d, want 1", fs.callCount())
		}
	})
}

// TestUsernameSaveRace23505: a name another account holds is answered with the
// taken sentence, on the fake with a 23505 and on a real database where the
// UNIQUE constraint decides.
func TestUsernameSaveRace23505(t *testing.T) {
	ctx := context.Background()

	t.Run("fake_23505", func(t *testing.T) {
		f := &fakeProfileQ{usernameErr: &pgconn.PgError{Code: "23505"}}
		_, err := usernameService(f, nil).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("Ravi.K")})
		wantMessage(t, err, "username", "ravi.k is taken. Try another.")
	})

	t.Run("other_database_errors_are_not_taken", func(t *testing.T) {
		f := &fakeProfileQ{usernameErr: &pgconn.PgError{Code: "40001"}}
		_, err := usernameService(f, nil).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("ravi.k")})
		if err == nil {
			t.Fatal("want an error")
		}
		var ve service.ValidationError
		if errors.As(err, &ve) {
			t.Fatalf("a serialisation failure was reported as a validation error: %v", err)
		}
	})

	t.Run("real_database", func(t *testing.T) {
		pool := testutil.NewTestDB(t)
		svc := service.NewAccountService(sqlcgen.New(pool))
		a := testutil.SeedReporterAccount(t, pool, "uname-race-a@example.com")
		b := testutil.SeedReporterAccount(t, pool, "uname-race-b@example.com")
		if _, err := svc.UpdateProfile(ctx, a, service.ProfileUpdate{Username: strp("Ravi.K")}); err != nil {
			t.Fatalf("first save: %v", err)
		}
		_, err := svc.UpdateProfile(ctx, b, service.ProfileUpdate{Username: strp("ravi.k")})
		wantMessage(t, err, "username", "ravi.k is taken. Try another.")
		p, err := svc.Profile(ctx, b)
		if err != nil {
			t.Fatalf("Profile: %v", err)
		}
		if p.Username != nil {
			t.Errorf("the loser holds %q, want no username", *p.Username)
		}
	})
}

// TestUsernameSaveRenameGap: the first set is free, a change inside seven days
// of the last one is refused with the days left rounded up and no provider
// call, a change after the gap is written, and the SQL's zero rows is the same
// sentence.
func TestUsernameSaveRenameGap(t *testing.T) {
	ctx := context.Background()
	day := 24 * time.Hour
	stored := func(changed *time.Time) sqlcgen.GetAccountProfileRow {
		row := sqlcgen.GetAccountProfileRow{Username: strp("old_name"), UsernameScreen: strp("clear")}
		if changed != nil {
			row.UsernameChangedAt = pgtype.Timestamptz{Time: *changed, Valid: true}
		}
		return row
	}

	t.Run("first_set_is_free", func(t *testing.T) {
		f := &fakeProfileQ{}
		if _, err := usernameService(f, nil).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("first_one")}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.usernameSets) != 1 {
			t.Fatalf("writes = %d, want 1", len(f.usernameSets))
		}
		cutoff := usernameNow.Add(-service.UsernameRenameGap)
		if !f.usernameSets[0].WaitCutoff.Equal(cutoff) || !f.usernameSets[0].Now.Equal(usernameNow) {
			t.Errorf("now %v cutoff %v, want %v and %v", f.usernameSets[0].Now, f.usernameSets[0].WaitCutoff, usernameNow, cutoff)
		}
	})

	t.Run("first_change_with_no_change_time_is_free", func(t *testing.T) {
		f := &fakeProfileQ{profile: stored(nil)}
		if _, err := usernameService(f, nil).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("new_name")}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.usernameSets) != 1 {
			t.Fatalf("writes = %d, want 1", len(f.usernameSets))
		}
	})

	t.Run("inside_the_gap_is_refused_with_days_rounded_up", func(t *testing.T) {
		cases := []struct {
			ago  time.Duration
			days int
		}{
			{1 * time.Hour, 7},
			{2*day + 3*time.Hour, 5},
			{6*day + 23*time.Hour, 1},
		}
		for _, c := range cases {
			changed := usernameNow.Add(-c.ago)
			f := &fakeProfileQ{profile: stored(&changed)}
			fs := screenerWith(map[string]float64{})
			_, err := usernameService(f, fs).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("new_name")})
			wantMessage(t, err, "username", service.UsernameWaitMessage(c.days))
			if fs.callCount() != 0 || len(f.usernameSets) != 0 {
				t.Errorf("%v ago: screened %d, wrote %d, want none", c.ago, fs.callCount(), len(f.usernameSets))
			}
		}
	})

	t.Run("after_the_gap_is_written", func(t *testing.T) {
		changed := usernameNow.Add(-service.UsernameRenameGap)
		f := &fakeProfileQ{profile: stored(&changed)}
		if _, err := usernameService(f, nil).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("new_name")}); err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if len(f.usernameSets) != 1 {
			t.Fatalf("writes = %d, want 1", len(f.usernameSets))
		}
	})

	t.Run("zero_rows_is_the_wait_sentence", func(t *testing.T) {
		f := &fakeProfileQ{usernameZeroRows: true}
		_, err := usernameService(f, nil).UpdateProfile(ctx, 3, service.ProfileUpdate{Username: strp("new_name")})
		wantMessage(t, err, "username", service.UsernameWaitMessage(7))
	})
}

// TestCheckUsernameStates: every state of the live check, and the screener is
// never asked.
func TestCheckUsernameStates(t *testing.T) {
	ctx := context.Background()
	recent := usernameNow.Add(-48 * time.Hour)

	f := &fakeProfileQ{usernameHolders: map[string]int64{"taken.one": 99}}
	fs := screenerWith(map[string]float64{})
	svc := usernameService(f, fs)

	check := func(f *fakeProfileQ, typed string) service.UsernameCheck {
		t.Helper()
		got, err := usernameService(f, fs).CheckUsername(ctx, 3, typed)
		if err != nil {
			t.Fatalf("CheckUsername(%q): %v", typed, err)
		}
		return got
	}

	t.Run("invalid", func(t *testing.T) {
		got := check(f, "no way")
		if got.State != service.UsernameStateInvalid || got.Message != service.UsernameRuleMessage {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("reserved", func(t *testing.T) {
		got := check(f, "P0lice")
		if got.State != service.UsernameStateReserved || got.Message != service.UsernameReservedMessage || got.Name != "p0lice" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("available", func(t *testing.T) {
		got := check(f, "@Fresh.Name")
		if got.State != service.UsernameStateAvailable || got.Name != "fresh.name" || got.Message != "fresh.name is available." {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("taken", func(t *testing.T) {
		got := check(f, "taken.one")
		if got.State != service.UsernameStateTaken || got.Message != "taken.one is taken. Try another." {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("yours", func(t *testing.T) {
		own := &fakeProfileQ{profile: sqlcgen.GetAccountProfileRow{Username: strp("my.name"), UsernameScreen: strp("clear")}, usernameHolders: map[string]int64{"my.name": 3}}
		got := check(own, "My.Name")
		if got.State != service.UsernameStateYours || got.Message != service.UsernameYoursMessage {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("wait", func(t *testing.T) {
		own := &fakeProfileQ{profile: sqlcgen.GetAccountProfileRow{
			Username: strp("my.name"), UsernameScreen: strp("clear"),
			UsernameChangedAt: pgtype.Timestamptz{Time: recent, Valid: true},
		}}
		got := check(own, "another.one")
		if got.State != service.UsernameStateWait || got.Days != 5 || got.Message != service.UsernameWaitMessage(5) {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("a_taken_name_inside_the_gap_is_wait_and_yours_is_still_yours", func(t *testing.T) {
		own := &fakeProfileQ{
			profile: sqlcgen.GetAccountProfileRow{
				Username: strp("my.name"), UsernameScreen: strp("clear"),
				UsernameChangedAt: pgtype.Timestamptz{Time: recent, Valid: true},
			},
			usernameHolders: map[string]int64{"taken.one": 99},
		}
		if got := check(own, "taken.one"); got.State != service.UsernameStateWait {
			t.Errorf("got %+v, want wait", got)
		}
		if got := check(own, "my.name"); got.State != service.UsernameStateYours {
			t.Errorf("got %+v, want yours", got)
		}
	})

	if fs.callCount() != 0 {
		t.Errorf("the live check called the screening provider %d times, want 0", fs.callCount())
	}
	_ = svc
	if strings.ContainsAny(service.UsernameWaitMessage(2), string(rune(0x2014))+string(rune(0x2013))) {
		t.Error("the wait sentence holds a dash")
	}
}

// TestUsernamesFor: the batched read keys by account id, carries the screen
// word, answers an empty list without a query, and (on a real database) leaves
// out accounts with no username and deleted accounts.
func TestUsernamesFor(t *testing.T) {
	ctx := context.Background()

	t.Run("empty_list_no_query", func(t *testing.T) {
		f := &fakeProfileQ{}
		got, err := usernameService(f, nil).UsernamesFor(ctx, nil)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %v, %v; want an empty map", got, err)
		}
		if f.usernameReads != 0 {
			t.Errorf("an empty list ran %d queries, want 0", f.usernameReads)
		}
	})

	t.Run("fake_rows", func(t *testing.T) {
		f := &fakeProfileQ{usernameRows: []sqlcgen.UsernamesByAccountIDsRow{
			{ID: 1, Username: strp("clear.one"), UsernameScreen: strp("clear")},
			{ID: 2, Username: strp("pending.one"), UsernameScreen: strp("pending")},
		}}
		got, err := usernameService(f, nil).UsernamesFor(ctx, []int64{1, 2, 3})
		if err != nil {
			t.Fatalf("UsernamesFor: %v", err)
		}
		if len(got) != 2 || got[1] != (service.UsernameView{Username: "clear.one", Screen: service.UsernameScreenClear}) ||
			got[2] != (service.UsernameView{Username: "pending.one", Screen: service.UsernameScreenPending}) {
			t.Errorf("got %+v", got)
		}
		if f.usernameReads != 1 {
			t.Errorf("queries = %d, want 1", f.usernameReads)
		}
	})

	t.Run("real_database", func(t *testing.T) {
		pool := testutil.NewTestDB(t)
		svc := service.NewAccountService(sqlcgen.New(pool))
		a := testutil.SeedReporterAccount(t, pool, "unames-a@example.com")
		b := testutil.SeedReporterAccount(t, pool, "unames-b@example.com")
		c := testutil.SeedReporterAccount(t, pool, "unames-c@example.com")
		d := testutil.SeedReporterAccount(t, pool, "unames-d@example.com")
		for id, name := range map[int64]string{a: "name.a", b: "name.b", d: "name.d"} {
			if _, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{Username: strp(name)}); err != nil {
				t.Fatalf("save %s: %v", name, err)
			}
		}
		testutil.MustExec(t, pool, `UPDATE accounts SET username_screen = 'pending' WHERE id = $1`, b)
		testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, d)

		got, err := svc.UsernamesFor(ctx, []int64{a, b, c, d})
		if err != nil {
			t.Fatalf("UsernamesFor: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %+v, want 2 entries (no username and deleted left out)", got)
		}
		if got[a] != (service.UsernameView{Username: "name.a", Screen: "clear"}) || got[b] != (service.UsernameView{Username: "name.b", Screen: "pending"}) {
			t.Errorf("got %+v", got)
		}
	})
}
