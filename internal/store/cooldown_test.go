package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestClaimEmailCooldownConcurrent proves exactly one of four simultaneous
// ClaimEmailCooldown calls for one address succeeds — email_cooldowns'
// primary key is what serialises the race (DEC-W, T-01-93), the same
// exactly-one-winner property TestConsumeTokenConcurrent proves for the
// token table's own atomic conditional UPDATE.
func TestClaimEmailCooldownConcurrent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	requestedAt := time.Now()
	cooldownCutoff := requestedAt.Add(-45 * time.Second)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	noRowsCount := 0

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
				Email:          "concurrent-claim@example.com",
				RequestedAt:    requestedAt,
				CooldownCutoff: cooldownCutoff,
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else if errors.Is(err, pgx.ErrNoRows) {
				noRowsCount++
			} else {
				t.Errorf("unexpected ClaimEmailCooldown error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("successCount = %d, want exactly 1", successCount)
	}
	if noRowsCount != 3 {
		t.Fatalf("noRowsCount = %d, want exactly 3", noRowsCount)
	}
}

// TestClaimEmailCooldownRefusalLeavesTimestampUntouched asserts DEC-J's "a
// refusal cannot extend its own cooldown" directly against the stored
// column, not merely against the claim's own return value: a second claim
// inside the window is refused, and the row's last_requested_at is
// byte-identical before and after that refusal.
func TestClaimEmailCooldownRefusalLeavesTimestampUntouched(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	firstRequestedAt := time.Now()
	if _, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
		Email:          "refusal-untouched@example.com",
		RequestedAt:    firstRequestedAt,
		CooldownCutoff: firstRequestedAt.Add(-45 * time.Second),
	}); err != nil {
		t.Fatalf("first claim: %v", err)
	}

	var before time.Time
	if err := pool.QueryRow(ctx,
		`SELECT last_requested_at FROM email_cooldowns WHERE email = $1`,
		"refusal-untouched@example.com").Scan(&before); err != nil {
		t.Fatalf("reading last_requested_at before refusal: %v", err)
	}

	secondRequestedAt := firstRequestedAt.Add(10 * time.Second)
	_, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
		Email:          "refusal-untouched@example.com",
		RequestedAt:    secondRequestedAt,
		CooldownCutoff: secondRequestedAt.Add(-45 * time.Second),
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second claim inside the window: err = %v, want pgx.ErrNoRows", err)
	}

	var after time.Time
	if err := pool.QueryRow(ctx,
		`SELECT last_requested_at FROM email_cooldowns WHERE email = $1`,
		"refusal-untouched@example.com").Scan(&after); err != nil {
		t.Fatalf("reading last_requested_at after refusal: %v", err)
	}

	if !before.Equal(after) {
		t.Fatalf("last_requested_at changed after a refused claim: before = %v, after = %v", before, after)
	}
}

// TestClaimEmailCooldownAllowsAfterWindow asserts a claim with a cutoff
// later than the stored timestamp succeeds and advances the stored value.
func TestClaimEmailCooldownAllowsAfterWindow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	firstRequestedAt := time.Now()
	if _, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
		Email:          "allowed-after-window@example.com",
		RequestedAt:    firstRequestedAt,
		CooldownCutoff: firstRequestedAt.Add(-45 * time.Second),
	}); err != nil {
		t.Fatalf("first claim: %v", err)
	}

	secondRequestedAt := firstRequestedAt.Add(time.Minute)
	got, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
		Email:       "allowed-after-window@example.com",
		RequestedAt: secondRequestedAt,
		// A cutoff later than firstRequestedAt means the stored value is no
		// longer within the window, so the guard passes.
		CooldownCutoff: firstRequestedAt.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("second claim after window elapsed: %v", err)
	}
	if !got.Equal(secondRequestedAt) {
		t.Fatalf("second claim returned %v, want %v", got, secondRequestedAt)
	}
}

// TestClaimEmailCooldownIsPerAddress asserts two different addresses never
// contend — each has its own row and its own clock.
func TestClaimEmailCooldownIsPerAddress(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	requestedAt := time.Now()
	cutoff := requestedAt.Add(-45 * time.Second)

	if _, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
		Email:          "per-address-one@example.com",
		RequestedAt:    requestedAt,
		CooldownCutoff: cutoff,
	}); err != nil {
		t.Fatalf("claim for first address: %v", err)
	}

	if _, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
		Email:          "per-address-two@example.com",
		RequestedAt:    requestedAt,
		CooldownCutoff: cutoff,
	}); err != nil {
		t.Fatalf("claim for second address inside the same window: %v", err)
	}
}
