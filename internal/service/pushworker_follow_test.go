// pushworker_follow_test.go proves the phone side of the follow note at send time
// (Phase 9 plan 26, PROF-03, D-09, D-24, D-25, research 5.2 items 3 and 4,
// Pitfall 4): the worker checks a queued follow note again before it sends it,
// leaves the distance rule out for it, replaces it in place when the report is
// confirmed, counts it toward the hourly cap and sends a Critical alert past a
// full hour. The database tests run the real dispatcher and the real worker with
// a fake push service, on the follow rig of plan 09-22, and skip without
// DATABASE_URL.
//
// Every person here is made up and uses example.com.

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"pinalert/internal/push"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// The fake store of the worker's unit tests (pwQuerier) gained the follow reads
// when PushWorkerQuerier did (plan 09-26, 2026-10-30). No unit test there has a
// follow row, so each answers "nothing holds"; the follow behaviour is proved on
// the real database below.

func (q *pwQuerier) ActiveFollowNotify(_ context.Context, _ sqlcgen.ActiveFollowNotifyParams) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("ActiveFollowNotify")
	return false, nil
}

func (q *pwQuerier) FolloweeStanding(_ context.Context, _ []int64) ([]sqlcgen.FolloweeStandingRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("FolloweeStanding")
	return nil, nil
}

func (q *pwQuerier) PersonRelation(_ context.Context, _ sqlcgen.PersonRelationParams) (sqlcgen.PersonRelationRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("PersonRelation")
	return sqlcgen.PersonRelationRow{}, nil
}

func (q *pwQuerier) PersonMuted(_ context.Context, _ sqlcgen.PersonMutedParams) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("PersonMuted")
	return false, nil
}

func (q *pwQuerier) ReporterHistoryReports(_ context.Context, _ sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rec("ReporterHistoryReports")
	return nil, nil
}

// pwFollowWorker is the worker of these tests: the rig's clock and the one
// ratings source, like main.go builds it.
func pwFollowWorker(r *followRig, sender push.Sender) *PushWorker {
	return NewPushWorker(r.q, sender,
		WithPushWorkerClock(func() time.Time { return r.cur }),
		WithPushWorkerJitter(func() float64 { return 0.5 }),
		WithPushWorkerSignals(NewRatingService(r.q)))
}

// followSession returns a session name for a device that is unique in the rig.
func (r *followRig) followSession(prefix string) string {
	r.seq2++
	return fmt.Sprintf("p9-26-sess-%s-%d", prefix, r.seq2)
}

// followerWithDevice makes a follower with no home area, no settings row and one
// Google device, who has the bell on for the person since 10 minutes after the
// rig's start.
func (r *followRig) followerWithDevice(name string, person int64) int64 {
	r.t.Helper()
	id := r.follower(name)
	r.device(id, r.followSession(name))
	r.bellOn(id, person, r.base.Add(10*time.Minute))
	return id
}

// postedByPerson posts a low severity report for a person's session and runs the
// dispatcher on it at the rig's clock, which is moved to 30 minutes after the
// start first. The follow notes it queues are due at once.
func (r *followRig) postedByPerson(session string) int64 {
	r.t.Helper()
	r.cur = r.base.Add(30 * time.Minute)
	report := r.report(session, SeverityLow, r.base.Add(20*time.Minute), "clear")
	r.eval(report, 0)
	return report
}

// followRowState reads the state and skip reason of a follower's follow row.
func (r *followRig) followRowState(follower, report int64) (state, skip string) {
	r.t.Helper()
	state, skip, _ = pwOutboxRow(r.t, r.pushDBRig, follower, report, "follow")
	return state, skip
}

// TestFollowPushNoHomeAreaStillSent: a follower with no home area gets the follow
// note, and so does one whose home area is far outside any alert radius; the
// words leave the distance out for the first and carry it for the second. A
// follower is not held back by distance (research Pitfall 4, UI-SPEC Defaults
// 19).
func TestFollowPushNoHomeAreaStillSent(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.person("w26.nohome")
	noHome := r.followerWithDevice("nohome", person)

	// A reader with a home area about 140 km away and the default 5 km radius.
	farID, _ := r.reader("far", 30, 0, true, false, false)
	r.bellOn(farID, person, r.base.Add(10*time.Minute))

	report := r.postedByPerson(session)
	rows := r.outbox(report)
	if !outboxKindsEqual(rows[noHome], "follow") || !outboxKindsEqual(rows[farID], "follow") {
		t.Fatalf("outbox = %v, want a follow row for both followers", rows)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := len(sender.messages()); got != 2 {
		t.Fatalf("messages = %d, want 2 (neither a missing home area nor the distance stops a follow note)", got)
	}
	for _, id := range []int64{noHome, farID} {
		if state, skip := r.followRowState(id, report); state != "sent" {
			t.Errorf("account %d: follow row is %q (%q), want sent", id, state, skip)
		}
	}

	var withoutDistance, withDistance string
	for _, m := range sender.messages() {
		if m.msg.Payload.Kind != "follow" || m.msg.Payload.Tag != fmt.Sprintf("report-%d", report) {
			t.Errorf("envelope = %+v, want kind follow under the report tag", m.msg.Payload)
		}
		if strings.Contains(m.msg.Payload.Body, "from your home area") {
			withDistance = m.msg.Payload.Body
		} else {
			withoutDistance = m.msg.Payload.Body
		}
	}
	if want := FromSomeoneYouFollowWords + " " + NotConfirmedYetWords; withoutDistance != want {
		t.Errorf("body without a home area = %q, want %q", withoutDistance, want)
	}
	if want := FromSomeoneYouFollowWords + " " + DistanceWords(r.distKm(r.cell(30))) + " from your home area. " + NotConfirmedYetWords; withDistance != want {
		t.Errorf("body with a far home area = %q, want %q", withDistance, want)
	}
}

// edgePerson makes a public reporter whose whole history sits at the far edge of
// the 90 day window: it counts at the rig's start plus 30 minutes and has fallen
// out of the window 20 minutes later. Time alone, with nothing else changed,
// takes the Reliable reporter tag away.
func (r *followRig) edgePerson(username string) (int64, string) {
	r.t.Helper()
	r.seq2++
	session := fmt.Sprintf("p9-26-sess-%s-%d", username, r.seq2)
	at := r.base.Add(-ReliabilityWindow + 100*time.Minute)
	id := testutil.SeedPublicReporter(r.t, r.pool, fmt.Sprintf("p9-26-%s-%d@example.com", username, r.seq2), username, session, at)
	return id, session
}

// TestFollowPushSendTimeRecheckDecay: a follow note queued while the reporter
// holds the Reliable reporter tag is skipped as not eligible once the clock has
// moved so the tag has fallen, with no sweep having run and the follow itself
// still active (D-24, research 5.2 item 3). The same note is sent when the clock
// has not moved.
func TestFollowPushSendTimeRecheckDecay(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.edgePerson("w26.decay")
	kept := r.followerWithDevice("kept", person)
	report := r.postedByPerson(session)
	if !outboxKindsEqual(r.outbox(report)[kept], "follow") {
		t.Fatalf("outbox = %v, want a queued follow row while the tag holds", r.outbox(report))
	}

	// 1. Still at the queue time: the tag holds, so the note goes out.
	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if state, skip := r.followRowState(kept, report); state != "sent" || len(sender.messages()) != 1 {
		t.Fatalf("follow row is %q (%q) with %d messages, want sent and 1 before the tag falls", state, skip, len(sender.messages()))
	}

	// 2. A second follower, a second report, and the clock 20 minutes on.
	second := r.followerWithDevice("second", person)
	r.cur = r.base.Add(30 * time.Minute)
	report2 := r.report(session, SeverityLow, r.base.Add(25*time.Minute), "clear")
	r.eval(report2, 0)
	if !outboxKindsEqual(r.outbox(report2)[second], "follow") {
		t.Fatalf("outbox = %v, want a queued follow row for the second report", r.outbox(report2))
	}
	r.cur = r.base.Add(50 * time.Minute)
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if state, skip := r.followRowState(second, report2); state != "skipped" || skip != "not_eligible" {
		t.Errorf("follow row is %q (%q), want skipped as not_eligible once the tag fell", state, skip)
	}
	if got := len(sender.messages()); got != 1 {
		t.Errorf("messages = %d, want still 1", got)
	}
	// No sweep ran: the follow is still active, so the skip came from the worker.
	if n := r.count(`SELECT COUNT(*) FROM account_follows WHERE follower_id = $1 AND followee_id = $2 AND ended_at IS NULL`, second, person); n != 1 {
		t.Errorf("active follows = %d, want 1 (only the send time check stopped the note)", n)
	}
}

// TestFollowPushSkipsMutedOrBlocked: a queued follow note is skipped as not
// eligible when, after it was queued, the follower muted or blocked the person,
// the person blocked the follower, the bell went off, the follow ended or the
// person went private. A follower with none of these still gets it.
func TestFollowPushSkipsMutedOrBlocked(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.person("w26.gone")

	names := []string{"control", "muted", "blocker", "blocked", "belloff", "unfollowed"}
	ids := map[string]int64{}
	for _, name := range names {
		ids[name] = r.followerWithDevice(name, person)
	}
	report := r.postedByPerson(session)
	for _, name := range names {
		if !outboxKindsEqual(r.outbox(report)[ids[name]], "follow") {
			t.Fatalf("%s holds %v, want a queued follow row", name, r.outbox(report)[ids[name]])
		}
	}

	testutil.SeedMute(t, r.pool, ids["muted"], person)
	testutil.SeedBlock(t, r.pool, ids["blocker"], person)
	testutil.SeedBlock(t, r.pool, person, ids["blocked"])
	r.setBell(ids["belloff"], person, false, r.cur)
	if _, err := r.q.EndFollowPair(ctx, sqlcgen.EndFollowPairParams{Now: r.cur, Reason: "unfollowed", FollowerID: ids["unfollowed"], FolloweeID: person}); err != nil {
		t.Fatal(err)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	for _, name := range names {
		state, skip := r.followRowState(ids[name], report)
		if name == "control" {
			if state != "sent" {
				t.Errorf("control: follow row is %q (%q), want sent", state, skip)
			}
			continue
		}
		if state != "skipped" || skip != "not_eligible" {
			t.Errorf("%s: follow row is %q (%q), want skipped as not_eligible", name, state, skip)
		}
	}
	if got := len(sender.messages()); got != 1 {
		t.Errorf("messages = %d, want 1 (the control only)", got)
	}

	// The person going private stops the note too.
	person2, session2 := r.person("w26.private")
	f2 := r.followerWithDevice("private", person2)
	report2 := r.postedByPerson(session2)
	testutil.MustExec(t, r.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, person2)
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if state, skip := r.followRowState(f2, report2); state != "skipped" || skip != "not_eligible" {
		t.Errorf("follow row after the person went private is %q (%q), want skipped as not_eligible", state, skip)
	}
}

// TestFollowPushReadsRatedTag: the worker reads the tag through the ratings
// source (review W8). A person whom raters pulled below the Reliable reporter
// line after the note was queued gets no note when the worker has the source,
// and would still get one from a worker without it, which is the drift the
// wiring test guards against.
func TestFollowPushReadsRatedTag(t *testing.T) {
	for _, tc := range []struct {
		name        string
		withSignals bool
		wantSent    bool
	}{
		{"with the ratings source", true, false},
		{"without the ratings source", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newFollowPushRig(t)
			ctx := context.Background()
			person, session := r.person("w26.rated")
			follower := r.followerWithDevice("rated", person)
			report := r.postedByPerson(session)
			if !outboxKindsEqual(r.outbox(report)[follower], "follow") {
				t.Fatalf("outbox = %v, want a queued follow row", r.outbox(report))
			}
			r.ratedDown(person, session)

			sender := &pwSender{answers: map[string]pwAnswer{}}
			worker := pwDBWorker(r.pushDBRig, sender)
			if tc.withSignals {
				worker = pwFollowWorker(r, sender)
			}
			if err := worker.SweepOnce(ctx); err != nil {
				t.Fatalf("sweep: %v", err)
			}
			state, skip := r.followRowState(follower, report)
			if tc.wantSent && state != "sent" {
				t.Errorf("follow row is %q (%q), want sent", state, skip)
			}
			if !tc.wantSent && (state != "skipped" || skip != "not_eligible") {
				t.Errorf("follow row is %q (%q), want skipped as not_eligible", state, skip)
			}
		})
	}
}

// confirmByNearbyPeople adds two confirming votes from two cells, so the report
// is confirmed by people nearby.
func (r *followRig) confirmByNearbyPeople(report int64, tag string) {
	r.t.Helper()
	v1 := testutil.SeedReporterAccount(r.t, r.pool, fmt.Sprintf("p9-26-%s-v1@example.com", tag), r.followSession(tag+"v1"))
	v2 := testutil.SeedReporterAccount(r.t, r.pool, fmt.Sprintf("p9-26-%s-v2@example.com", tag), r.followSession(tag+"v2"))
	r.vote(report, v1, "content", "confirm", "tdr1v8y", r.cur)
	r.vote(report, v2, "content", "confirm", "tdr1vc3", r.cur)
	r.eval(report, v1)
	r.eval(report, v2)
	r.eval(report, 0)
}

// TestFollowPushConfirmedInPlace: once people nearby confirm the report, the
// follow note is replaced in place under the same tag, with no second alert
// (normal urgency) and no count toward the hourly cap; the follow note itself
// counted. The replacement also goes through the send time check: when the
// person lost the tag before it was sent, nothing is sent.
func TestFollowPushConfirmedInPlace(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.person("w26.confirm")
	follower := r.followerWithDevice("confirm", person)
	report := r.postedByPerson(session)

	sender := &pwSender{answers: map[string]pwAnswer{}}
	worker := pwFollowWorker(r, sender)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if got := len(sender.messages()); got != 1 {
		t.Fatalf("messages after the first sweep = %d, want 1", got)
	}

	r.confirmByNearbyPeople(report, "confirm")
	if !outboxKindsEqual(r.outbox(report)[follower], "confirmed_update", "follow") {
		t.Fatalf("outbox = %v, want the follow row and a confirmed update", r.outbox(report)[follower])
	}
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	msgs := sender.messages()
	if len(msgs) != 2 {
		t.Fatalf("messages after the confirmation = %d, want 2", len(msgs))
	}
	first, update := msgs[0].msg, msgs[1].msg
	if update.Payload.Kind != "confirmed_update" || update.Payload.Tag != first.Payload.Tag {
		t.Errorf("update envelope = %+v, want kind confirmed_update under the tag %q", update.Payload, first.Payload.Tag)
	}
	if want := "Confirmed by people nearby."; update.Payload.Body != want {
		t.Errorf("update body = %q, want %q", update.Payload.Body, want)
	}
	if update.Urgency != push.UrgencyNormal {
		t.Errorf("update urgency = %q, want normal (no second alert)", update.Urgency)
	}
	if state, _, _ := pwOutboxRow(t, r.pushDBRig, follower, report, "confirmed_update"); state != "sent" {
		t.Errorf("the confirmed update is %q, want sent", state)
	}
	// Only the follow note counts toward the cap.
	n, err := r.q.CountAccountPushesInWindow(ctx, countParams(follower, r.cur.Add(-time.Hour)))
	if err != nil || n != 1 {
		t.Errorf("pushes counted = %d (err %v), want 1: the follow note counts, the replacement does not", n, err)
	}

	// The replacement is checked at send time too: a person who lost the tag
	// before it was sent gets nothing.
	person2, session2 := r.person("w26.confirm2")
	f2 := r.followerWithDevice("confirm2", person2)
	report2 := r.postedByPerson(session2)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("third sweep: %v", err)
	}
	r.confirmByNearbyPeople(report2, "confirm2")
	r.ratedDown(person2, session2)
	before := len(sender.messages())
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("fourth sweep: %v", err)
	}
	if state, skip, _ := pwOutboxRow(t, r.pushDBRig, f2, report2, "confirmed_update"); state != "skipped" || skip != "not_eligible" {
		t.Errorf("confirmed update after the tag fell is %q (%q), want skipped as not_eligible", state, skip)
	}
	if got := len(sender.messages()); got != before {
		t.Errorf("messages = %d, want still %d", got, before)
	}
}

// sentPushesInHour writes n sent follow rows for the account, each about its own
// report by another person, five minutes before the rig's clock.
func (r *followRig) sentPushesInHour(account int64, n int, tag string) {
	r.t.Helper()
	session := r.followSession(tag)
	testutil.SeedReporterAccount(r.t, r.pool, fmt.Sprintf("p9-26-%s@example.com", session), session)
	for i := 0; i < n; i++ {
		other := r.report(session, SeverityLow, r.base.Add(time.Duration(i+1)*time.Minute), "clear")
		testutil.MustExec(r.t, r.pool,
			`INSERT INTO push_outbox (account_id, report_id, kind, state, sent_at, expires_at)
			 VALUES ($1, $2, 'follow', 'sent', $3, $4)`,
			account, other, r.cur.Add(-5*time.Minute), r.cur.Add(time.Hour))
	}
}

// TestFollowPushHourlyCap: follow notes count toward PushPerAccountPerHour. With
// the hour full of sent follow notes a new one is held back as cap_account_hour
// and nothing is sent; with one fewer it is sent.
func TestFollowPushHourlyCap(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.person("w26.cap")
	full := r.followerWithDevice("full", person)
	room := r.followerWithDevice("room", person)
	r.cur = r.base.Add(30 * time.Minute)
	r.sentPushesInHour(full, PushPerAccountPerHour, "capfull")
	r.sentPushesInHour(room, PushPerAccountPerHour-1, "caproom")
	if n, err := r.q.CountAccountPushesInWindow(ctx, countParams(full, r.cur.Add(-time.Hour))); err != nil || n != PushPerAccountPerHour {
		t.Fatalf("sent follow rows counted = %d (err %v), want %d", n, err, PushPerAccountPerHour)
	}

	report := r.postedByPerson(session)
	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if state, skip := r.followRowState(full, report); state != "skipped" || skip != "cap_account_hour" {
		t.Errorf("full hour: follow row is %q (%q), want skipped as cap_account_hour", state, skip)
	}
	if state, skip := r.followRowState(room, report); state != "sent" {
		t.Errorf("one below the cap: follow row is %q (%q), want sent", state, skip)
	}
	if got := len(sender.messages()); got != 1 {
		t.Errorf("messages = %d, want 1", got)
	}
	// The held note stays in the bell.
	if n := r.notes(full, report); n != 1 {
		t.Errorf("bell rows for the held follower = %d, want 1", n)
	}
}

// TestFollowPushAppleFirstOnly: the one notification per report rule of the 3.3
// gap fix (D-25) reaches the follow kind through the same functions as every
// other kind. A follower with an Apple and a Google device, who already got a
// push about report R on both, gets the follow note about R on the Google device
// only, under the report tag, and the bell row stays; a follow note about a
// report with no earlier push reaches both devices.
//
// The earlier push about R is a sent changed row: the unique first alert index
// lets one critical, reliable or follow row exist per person and report, so an
// earlier Critical or Reliable push cannot sit beside a follow row for the same
// pair. The check counts a sent push of any kind (AccountPushSentAboutReport).
func TestFollowPushAppleFirstOnly(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.person("w26.apple")
	follower := r.follower("apple")
	const appleEndpoint = "https://web.push.apple.com/p9-26-apple-token"
	r.device(follower, r.followSession("apple-google"))
	testutil.SeedPushSubscription(t, r.pool, follower, r.followSession("apple-device"), appleEndpoint)
	var googleEndpoint string
	if err := r.pool.QueryRow(ctx,
		`SELECT endpoint FROM push_subscriptions WHERE account_id = $1 AND endpoint <> $2`,
		follower, appleEndpoint).Scan(&googleEndpoint); err != nil {
		t.Fatalf("reading the Google endpoint: %v", err)
	}
	r.bellOn(follower, person, r.base.Add(10*time.Minute))

	// Report R already got a push on this account; report S did not.
	reportR := r.postedByPerson(session)
	reportS := r.report(session, SeverityLow, r.base.Add(22*time.Minute), "clear")
	r.eval(reportS, 0)
	changedID := pwQueueChanged(t, r.pushDBRig, follower, reportR)
	testutil.MustExec(t, r.pool, `UPDATE push_outbox SET state = 'sent', sent_at = $2 WHERE id = $1`, changedID, r.cur.Add(-5*time.Minute))

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	toR, toS := map[string]bool{}, map[string]bool{}
	for _, m := range sender.messages() {
		switch m.msg.Payload.Tag {
		case fmt.Sprintf("report-%d", reportR):
			toR[m.sub.Endpoint] = true
		case fmt.Sprintf("report-%d", reportS):
			toS[m.sub.Endpoint] = true
		}
	}
	if len(toR) != 1 || !toR[googleEndpoint] {
		t.Errorf("the follow note about R went to %v, want the Google device only", toR)
	}
	if len(toS) != 2 || !toS[googleEndpoint] || !toS[appleEndpoint] {
		t.Errorf("the follow note about S went to %v, want both devices", toS)
	}
	if state, skip := r.followRowState(follower, reportR); state != "sent" {
		t.Errorf("follow row about R is %q (%q), want sent (to the Google device)", state, skip)
	}
	if n := r.notes(follower, reportR); n != 1 {
		t.Errorf("bell rows about R = %d, want 1 (the note stays in the bell)", n)
	}

	// Confirming S later sends no confirmed update to the Apple device either.
	r.confirmByNearbyPeople(reportS, "apple")
	before := len(sender.messages())
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	for _, m := range sender.messages()[before:] {
		if m.sub.Endpoint == appleEndpoint {
			t.Errorf("a confirmed update reached the Apple device: %+v", m.msg.Payload)
		}
	}
	if got := len(sender.messages()) - before; got != 1 {
		t.Errorf("confirmed updates sent = %d, want 1 (the Google device)", got)
	}
}

// TestFollowPushNeverHoldsBackCritical: follow notes use up the hour, the next
// one is held back, and a Critical alert near the home area is still sent in the
// same hour, because the cap never stops Critical (it counts it). D-25.
func TestFollowPushNeverHoldsBackCritical(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()
	person, session := r.person("w26.critical")
	// Critical on, radius 5 km, a home cell next to the report point.
	reader, _ := r.reader("crit", 1, 5, true, false, false)
	r.bellOn(reader, person, r.base.Add(10*time.Minute))
	r.cur = r.base.Add(30 * time.Minute)
	r.sentPushesInHour(reader, PushPerAccountPerHour, "critfull")

	report := r.postedByPerson(session)
	sender := &pwSender{answers: map[string]pwAnswer{}}
	worker := pwFollowWorker(r, sender)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if state, skip := r.followRowState(reader, report); state != "skipped" || skip != "cap_account_hour" {
		t.Fatalf("follow row is %q (%q), want skipped as cap_account_hour with the hour full", state, skip)
	}

	// A Critical report by someone else, near the home area.
	const otherSession = "p9-26-critical-other"
	testutil.SeedReporterAccount(t, r.pool, "p9-26-critical-other@example.com", otherSession)
	critical := r.report(otherSession, SeverityCritical, r.base.Add(25*time.Minute), "clear")
	testutil.MustExec(t, r.pool, `UPDATE reports SET happening = 'fire' WHERE id = $1`, critical)
	r.eval(critical, 0)
	if !outboxKindsEqual(r.outbox(critical)[reader], "critical") {
		t.Fatalf("outbox = %v, want a critical row for the reader", r.outbox(critical))
	}
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if state, skip, _ := pwOutboxRow(t, r.pushDBRig, reader, critical, "critical"); state != "sent" {
		t.Errorf("critical row is %q (%q), want sent although follow notes filled the hour", state, skip)
	}
	if got := len(sender.messages()); got != 1 {
		t.Errorf("messages = %d, want 1 (the Critical alert only)", got)
	}
	if n, err := r.q.CountAccountPushesInWindow(ctx, countParams(reader, r.cur.Add(-time.Hour))); err != nil || n != PushPerAccountPerHour+1 {
		t.Errorf("pushes counted = %d (err %v), want %d: the Critical alert counts", n, err, PushPerAccountPerHour+1)
	}
}
