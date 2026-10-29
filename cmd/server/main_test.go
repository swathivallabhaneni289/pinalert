package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/mailer"
)

// TestSearchBudgetsMatchTheRouterDefaults keeps the two search budgets this
// program sets equal to the router's own defaults (quick 261016-sne), so the
// numbers a reader finds beside the defaults are the ones that run.
func TestSearchBudgetsMatchTheRouterDefaults(t *testing.T) {
	geocode := api.GeocodeRateLimit{Burst: geocodeRateLimitBurst, Every: geocodeRateLimitEvery}
	if geocode != api.GeocodeRateLimitDefault {
		t.Errorf("report search budget = %+v, want %+v", geocode, api.GeocodeRateLimitDefault)
	}
	place := api.PlaceRateLimit{Burst: placeRateLimitBurst, Every: placeRateLimitEvery}
	if place != api.PlaceRateLimitDefault {
		t.Errorf("place search budget = %+v, want %+v", place, api.PlaceRateLimitDefault)
	}
}

// TestAuthLimitsMatchTheRouterDefaults keeps the auth bucket block of this
// program equal to api.AuthLimitsDefault (Phase 7.1, plan 07), so the numbers a
// reader finds beside the defaults are the ones that run.
func TestAuthLimitsMatchTheRouterDefaults(t *testing.T) {
	if got := authLimits(); got != api.AuthLimitsDefault {
		t.Errorf("auth limits = %+v, want %+v", got, api.AuthLimitsDefault)
	}
}

// TestCeilingLimitsMatchTheMailerDefaults keeps the send ceiling block of this
// program equal to mailer.DefaultCeilingLimits (review CR-S03), and checks that
// main wires the paced ceiling, not the plain daily one.
func TestCeilingLimitsMatchTheMailerDefaults(t *testing.T) {
	if got := ceilingLimits(); got != mailer.DefaultCeilingLimits {
		t.Errorf("ceiling limits = %+v, want %+v", got, mailer.DefaultCeilingLimits)
	}
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	if !strings.Contains(string(raw), "mailer.NewSendCeiling(mail, ceilingLimits(), time.Now)") {
		t.Error("main must wrap the mailer with mailer.NewSendCeiling(mail, ceilingLimits(), time.Now)")
	}
}

// TestOnboardingGateIsSwitchedOn pins that the real program turns the first-time
// flow gate on (Phase 3.4 plan 10, D-27, orchestrator decision 5). The Deps
// literal is built inside main, so the check reads the source: a refactor that
// drops the field would leave every account unasked for consent.
func TestOnboardingGateIsSwitchedOn(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	if n := strings.Count(string(raw), "OnboardingGate: true,"); n != 1 {
		t.Errorf("main.go sets OnboardingGate: true %d times, want exactly 1", n)
	}
}

// fakeServer stands in for http.Server in the serveUntilDone tests: serve
// blocks until shutdown is called, like ListenAndServe, and then returns
// http.ErrServerClosed.
type fakeServer struct {
	closed   chan struct{}
	shutdown atomic.Int32
}

func newFakeServer() *fakeServer { return &fakeServer{closed: make(chan struct{})} }

func (s *fakeServer) serve() error {
	<-s.closed
	return http.ErrServerClosed
}

func (s *fakeServer) stop(context.Context) error {
	if s.shutdown.Add(1) == 1 {
		close(s.closed)
	}
	return nil
}

// runUntilDone runs serveUntilDone in the background and returns its result
// channel.
func runUntilDone(ctx context.Context, s *fakeServer, grace time.Duration, runners ...func(context.Context)) <-chan error {
	out := make(chan error, 1)
	go func() { out <- serveUntilDone(ctx, s.serve, s.stop, grace, runners...) }()
	return out
}

// TestServeUntilDoneWaitsForTheWorkerMark (review S-05, 2026-10-25): on SIGINT
// or SIGTERM the push worker's context ends, and the process waits for the
// worker to return, so a row whose send a push service accepted is still
// marked on its WithoutCancel context instead of being sent again after a
// redeploy. Before the fix main ran the workers on context.Background and had
// no shutdown at all.
func TestServeUntilDoneWaitsForTheWorkerMark(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newFakeServer()
	var marked atomic.Bool
	worker := func(ctx context.Context) {
		<-ctx.Done()
		// The in flight row settles after its context ended.
		time.Sleep(50 * time.Millisecond)
		marked.Store(true)
	}
	out := runUntilDone(ctx, s, 5*time.Second, worker)
	cancel()
	select {
	case err := <-out:
		if err != nil {
			t.Fatalf("serveUntilDone: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveUntilDone did not return")
	}
	if !marked.Load() {
		t.Error("serveUntilDone returned before the worker finished its row")
	}
	if n := s.shutdown.Load(); n != 1 {
		t.Errorf("Shutdown called %d times, want 1", n)
	}
}

// TestServeUntilDoneWithoutPushRunners: with VAPID unset there are no push
// runners, and the server still starts and stops normally.
func TestServeUntilDoneWithoutPushRunners(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newFakeServer()
	out := runUntilDone(ctx, s, 5*time.Second)
	cancel()
	select {
	case err := <-out:
		if err != nil {
			t.Fatalf("serveUntilDone: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveUntilDone did not return")
	}
	if n := s.shutdown.Load(); n != 1 {
		t.Errorf("Shutdown called %d times, want 1", n)
	}
}

// TestServeUntilDoneBoundsTheWait: a runner that never returns cannot hold the
// process past the grace period.
func TestServeUntilDoneBoundsTheWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newFakeServer()
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	out := runUntilDone(ctx, s, 50*time.Millisecond, func(context.Context) { <-stuck })
	cancel()
	select {
	case err := <-out:
		if err == nil {
			t.Error("want an error naming the runners that did not stop")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveUntilDone waited past its grace period")
	}
}

// TestServeUntilDoneReturnsAServeError: a server that fails on its own (a busy
// port) returns its error, and the runners are told to stop.
func TestServeUntilDoneReturnsAServeError(t *testing.T) {
	boom := errors.New("listen failed")
	var stopped atomic.Bool
	runner := func(ctx context.Context) { <-ctx.Done(); stopped.Store(true) }
	err := serveUntilDone(context.Background(), func() error { return boom },
		func(context.Context) error { return nil }, 5*time.Second, runner)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if !stopped.Load() {
		t.Error("the runners were not stopped after the server failed")
	}
}

// TestMainStopsOnSignals pins the wiring that the tests above cannot reach,
// the same way TestOnboardingGateIsSwitchedOn reads the source: main listens
// for SIGINT and SIGTERM and hands that context to both push loops.
func TestMainStopsOnSignals(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	src := string(raw)
	for _, want := range []string{
		"signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)",
		"pushDispatcher.Run,",
		"pushWorker.Run,",
		"serveUntilDone(runCtx,",
		// IN-01: the signal hook is released once the first signal arrives, so a
		// second Ctrl-C during the grace period is not swallowed.
		"context.AfterFunc(runCtx, stopSignals)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("main.go does not contain %q", want)
		}
	}
	if strings.Contains(src, "go pushWorker.Run(context.Background())") || strings.Contains(src, "go pushDispatcher.Run(context.Background())") {
		t.Error("a push loop still runs on context.Background, which no signal can end")
	}
}

// TestClientIPHeaderRequiredOutsideDevelopment is SK-01 of the Phase 7.1 review
// fix skeptic: without CLIENT_IP_HEADER every request with a valid cookie is
// its own mail sender, so a script that fetches a fresh cookie before each
// Forgot password can spend the reset reserve. Outside ENV=development the
// server therefore refuses to start without the header, with one plain line
// that says to run the README canary check and set it. Development keeps it
// optional, and a value that is not a plain header name is refused everywhere.
func TestClientIPHeaderRequiredOutsideDevelopment(t *testing.T) {
	for _, env := range []string{"", "production", "staging"} {
		got, err := clientIPHeaderFor(env, "  ")
		if err == nil {
			t.Errorf("ENV=%q with no CLIENT_IP_HEADER: got %q and no error, want a refusal", env, got)
			continue
		}
		msg := err.Error()
		if strings.Contains(msg, "\n") {
			t.Errorf("ENV=%q: the refusal is not one line: %q", env, msg)
		}
		for _, want := range []string{"CLIENT_IP_HEADER", "canary", "README"} {
			if !strings.Contains(msg, want) {
				t.Errorf("ENV=%q: the refusal %q does not name %q", env, msg, want)
			}
		}
	}
	if got, err := clientIPHeaderFor("development", ""); err != nil || got != "" {
		t.Errorf("ENV=development with no header = %q, %v; want it optional", got, err)
	}
	for _, env := range []string{"development", "production"} {
		if got, err := clientIPHeaderFor(env, " CF-Connecting-IP "); err != nil || got != "CF-Connecting-IP" {
			t.Errorf("ENV=%q with a plain header = %q, %v; want CF-Connecting-IP", env, got, err)
		}
		if _, err := clientIPHeaderFor(env, "X-Real-IP: 1"); err == nil {
			t.Errorf("ENV=%q: a header value that is not a plain name was accepted", env)
		}
	}

	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	if !strings.Contains(string(raw), `clientIPHeaderFor(env, os.Getenv("CLIENT_IP_HEADER"))`) {
		t.Error(`main must read the header through clientIPHeaderFor(env, os.Getenv("CLIENT_IP_HEADER"))`)
	}
}
