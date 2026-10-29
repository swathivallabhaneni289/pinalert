// people_notify_e2e_test.go proves the Phase 9 plan 23 Notify me route end to
// end over the real router and a real Postgres (D-08, D-12, T-09-80, T-09-81,
// T-09-82): switching the bell on and off for a followed person, the profile
// field, the refusal without a follow, after a mute and over the cap of 50,
// the strict body, and the uniform 404 for a person the caller may not see. It
// reuses the plan 07 helpers by name. Every package level identifier added here
// carries the p923 prefix. Every person is made up, with an example.com address.
package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// p923Notify sends the Notify me POST with a raw body and returns the status and
// the body.
func p923Notify(t *testing.T, client *http.Client, base, username, body string) (int, string) {
	t.Helper()
	status, got, _ := p907Do(t, client, http.MethodPost, base+"/api/people/"+username+"/notify", body)
	return status, got
}

// TestNotifyRoutes covers the route: follow, notify on (200, profile notify
// true), notify off, notify on without following (409), after a mute (profile
// false, notify on 409, unmute then on again), the cap with 50 bells seeded, bad
// bodies (400) and an unseen person (the one 404).
func TestNotifyRoutes(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	me, meID := p907Person(t, srv, mailer, pool, "p923-me@example.com", "bell_me", false)
	_, _ = p907Reporter(t, srv, mailer, pool, "p923-ava@example.com", "bell_ava")
	_, _ = p907Reporter(t, srv, mailer, pool, "p923-bob@example.com", "bell_bob")
	_, _ = p907Person(t, srv, mailer, pool, "p923-closed@example.com", "bell_closed", false)

	notifyFlag := func(username string) any {
		t.Helper()
		_, profile := p907Get(t, me, srv, "/api/people/"+username, http.StatusOK)
		v, has := profile["notify"]
		if !has {
			t.Fatalf("the profile of %s has no notify key: %v", username, profile)
		}
		return v
	}
	wantRefusal := func(label string, status int, body, message string) {
		t.Helper()
		if status != http.StatusConflict {
			t.Fatalf("%s: status = %d, want 409, body %s", label, status, body)
		}
		field, got := p907ErrorOf(t, body)
		if field != "notify" || got != message {
			t.Fatalf("%s: field %q message %q, want notify and %q", label, field, got, message)
		}
	}

	// Before following: the profile says notify false and the bell is refused.
	if got := notifyFlag("bell_ava"); got != false {
		t.Fatalf("notify before following = %v, want false", got)
	}
	status, body := p923Notify(t, me, srv.URL, "bell_ava", `{"on":true}`)
	wantRefusal("notify on without following", status, body, "Couldn't change this. Try again.")

	// Follow, then the bell goes on and off, and the profile follows it.
	p907Post(t, me, srv, "/api/people/bell_ava/follow", http.StatusNoContent)
	if got := notifyFlag("bell_ava"); got != false {
		t.Fatalf("notify right after following = %v, want false (off by default)", got)
	}
	status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":true}`)
	if status != http.StatusOK || strings.ReplaceAll(body, " ", "") != "{\"notify\":true}\n" {
		t.Fatalf("notify on: status %d body %q, want 200 and {\"notify\":true}", status, body)
	}
	if got := notifyFlag("bell_ava"); got != true {
		t.Fatalf("notify after turning on = %v, want true", got)
	}
	if status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":true}`); status != http.StatusOK {
		t.Fatalf("a repeat notify on: status %d body %s, want 200", status, body)
	}
	status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":false}`)
	if status != http.StatusOK || strings.ReplaceAll(body, " ", "") != "{\"notify\":false}\n" {
		t.Fatalf("notify off: status %d body %q, want 200 and {\"notify\":false}", status, body)
	}
	if got := notifyFlag("bell_ava"); got != false {
		t.Fatalf("notify after turning off = %v, want false", got)
	}

	// A mute turns the bell off, hides it, and refuses it until Unmute.
	if status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":true}`); status != http.StatusOK {
		t.Fatalf("notify on before the mute: status %d body %s", status, body)
	}
	p907Post(t, me, srv, "/api/people/bell_ava/mute", http.StatusNoContent)
	if got := notifyFlag("bell_ava"); got != false {
		t.Fatalf("notify after muting = %v, want false", got)
	}
	status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":true}`)
	wantRefusal("notify on for a muted person", status, body, "Couldn't change this. Try again.")
	p907Post(t, me, srv, "/api/people/bell_ava/unmute", http.StatusNoContent)
	if got := notifyFlag("bell_ava"); got != false {
		t.Fatalf("notify after Unmute = %v, want false (the bell stays off)", got)
	}
	if status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":true}`); status != http.StatusOK {
		t.Fatalf("notify on after Unmute: status %d body %s, want 200", status, body)
	}

	// The cap: 49 more bells make 50 with bell_ava's, then a 51st is refused with
	// the sentence for the limit. Seeded with one statement.
	if _, err := pool.Exec(context.Background(),
		`WITH made AS (
		     INSERT INTO accounts (email, username, username_screen, profile_public, profile_public_at)
		     SELECT 'p923-bulk-' || g::text || '@example.com', 'bellbulk' || lpad(g::text, 3, '0'), 'clear', true, now()
		     FROM generate_series(1, 49) g
		     RETURNING id)
		 INSERT INTO account_follows (follower_id, followee_id, created_at, notify, notify_since)
		 SELECT $1::bigint, id, now(), true, now() FROM made`, meID); err != nil {
		t.Fatalf("seeding 49 more bells: %v", err)
	}
	p907Post(t, me, srv, "/api/people/bell_bob/follow", http.StatusNoContent)
	status, body = p923Notify(t, me, srv.URL, "bell_bob", `{"on":true}`)
	wantRefusal("the 51st bell", status, body, "You can turn on Notify me for up to 50 people.")
	if got := notifyFlag("bell_bob"); got != false {
		t.Fatalf("a refused bell shows notify = %v, want false", got)
	}
	// A repeat on for a bell already on is not a cap refusal, and turning one off
	// makes room.
	if status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":true}`); status != http.StatusOK {
		t.Fatalf("a repeat on at the cap: status %d body %s, want 200", status, body)
	}
	if status, body = p923Notify(t, me, srv.URL, "bell_ava", `{"on":false}`); status != http.StatusOK {
		t.Fatalf("turning one off at the cap: status %d body %s, want 200", status, body)
	}
	if status, body = p923Notify(t, me, srv.URL, "bell_bob", `{"on":true}`); status != http.StatusOK {
		t.Fatalf("a bell after one was turned off: status %d body %s, want 200", status, body)
	}

	// The body is strict: on is required and nothing else is accepted.
	for _, bad := range []string{``, `{}`, `{"on":"yes"}`, `{"on":null}`, `{"on":true,"extra":1}`, `[true]`, `not json`} {
		status, body := p923Notify(t, me, srv.URL, "bell_ava", bad)
		if status != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400, body %s", bad, status, body)
			continue
		}
		if field, _ := p907ErrorOf(t, body); field != "on" {
			t.Errorf("body %q: field = %q, want on", bad, field)
		}
	}

	// A person the caller may not see, and one nobody holds, answer the one 404,
	// byte for byte, so the route never says who opted in.
	statusClosed, bodyClosed := p923Notify(t, me, srv.URL, "bell_closed", `{"on":true}`)
	statusNobody, bodyNobody := p923Notify(t, me, srv.URL, "bell_nobody", `{"on":true}`)
	statusMalformed, bodyMalformed := p923Notify(t, me, srv.URL, "no", `{"on":true}`)
	if statusClosed != http.StatusNotFound || statusNobody != http.StatusNotFound || statusMalformed != http.StatusNotFound {
		t.Fatalf("statuses = %d, %d, %d, want 404 for a private, an unknown and a malformed name", statusClosed, statusNobody, statusMalformed)
	}
	if bodyClosed != bodyNobody || bodyClosed != bodyMalformed {
		t.Errorf("the 404 bodies differ: %q, %q, %q", bodyClosed, bodyNobody, bodyMalformed)
	}
}
