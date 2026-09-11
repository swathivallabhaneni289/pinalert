package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestAllowsBurstThenRefuses is this limiter's core token-bucket contract: a
// fresh key gets exactly the configured burst, and the next request beyond
// it is refused.
func TestAllowsBurstThenRefuses(t *testing.T) {
	p := NewPerIP(time.Minute, 3)

	for i := 0; i < 3; i++ {
		if !p.Allow("1.2.3.4") {
			t.Fatalf("request %d within burst was refused", i+1)
		}
	}
	if p.Allow("1.2.3.4") {
		t.Fatal("request beyond burst was allowed")
	}
}

// TestRefillAfterInterval proves the bucket actually refills over time,
// rather than staying permanently exhausted once the burst is spent.
func TestRefillAfterInterval(t *testing.T) {
	const refill = 30 * time.Millisecond
	p := newPerIPWithIdleWindow(refill, 1, time.Hour)

	if !p.Allow("5.6.7.8") {
		t.Fatal("first request was refused")
	}
	if p.Allow("5.6.7.8") {
		t.Fatal("second request within the refill interval was allowed")
	}

	time.Sleep(refill * 3)

	if !p.Allow("5.6.7.8") {
		t.Fatal("request after the refill interval elapsed was refused")
	}
}

// TestIndependentKeysHaveIndependentBudgets asserts exhausting one key's
// budget never affects a different key's.
func TestIndependentKeysHaveIndependentBudgets(t *testing.T) {
	p := NewPerIP(time.Minute, 1)

	if !p.Allow("10.0.0.1") {
		t.Fatal("first key's first request was refused")
	}
	if p.Allow("10.0.0.1") {
		t.Fatal("first key's second request was allowed")
	}

	if !p.Allow("10.0.0.2") {
		t.Fatal("second key was refused even though it has never been used")
	}
}

// TestConcurrentAllowDoesNotRace drives many goroutines across many keys
// simultaneously. It makes no assertion about individual outcomes — its
// entire purpose is to be run under `-race` and prove Allow and the idle
// sweep never race each other or themselves.
func TestConcurrentAllowDoesNotRace(t *testing.T) {
	p := newPerIPWithIdleWindow(time.Millisecond, 5, 5*time.Millisecond)

	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "203.0.113." + string(rune('0'+id%10))
			for i := 0; i < 50; i++ {
				p.Allow(key)
			}
		}(g)
	}
	wg.Wait()
}

// TestIdleEntryIsEvicted exercises the sweep directly (bypassing the ticker
// goroutine's own timing) so the eviction behaviour is proven deterministically
// rather than left as an untested claim about a background goroutine.
func TestIdleEntryIsEvicted(t *testing.T) {
	p := newPerIPWithIdleWindow(time.Minute, 1, 10*time.Millisecond)

	p.Allow("198.51.100.1")
	if got := p.entryCount(); got != 1 {
		t.Fatalf("entryCount = %d, want 1 after a single Allow call", got)
	}

	// Sweep as if idleWindow has already elapsed, without waiting on the
	// real background ticker — deterministic and fast.
	p.sweep(time.Now().Add(20 * time.Millisecond))

	if got := p.entryCount(); got != 0 {
		t.Fatalf("entryCount = %d, want 0 after sweeping an idle entry", got)
	}
}

// TestBackgroundSweepEventuallyEvicts proves the constructor's own started
// goroutine — not just the sweep method in isolation — actually prunes an
// idle entry over real time, with a short idle window so the test stays
// fast.
func TestBackgroundSweepEventuallyEvicts(t *testing.T) {
	p := newPerIPWithIdleWindow(time.Minute, 1, 15*time.Millisecond)

	p.Allow("198.51.100.2")
	if got := p.entryCount(); got != 1 {
		t.Fatalf("entryCount = %d, want 1 immediately after Allow", got)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if p.entryCount() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background sweep never evicted the idle entry within 500ms")
}

// TestMiddlewareRefusesBeyondBurst drives the http.Handler shape end to end:
// a burst of requests succeeds, the next is refused with 429 and the exact
// generic envelope, and no field distinguishes this refusal from the
// per-email cooldown's own 429.
func TestMiddlewareRefusesBeyondBurst(t *testing.T) {
	p := NewPerIP(time.Minute, 2)
	handler := p.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/request-link", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		return req
	}

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newReq())
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newReq())
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	want := `{"error":{"field":"email","message":"Too many requests — try again in a minute."}}`
	if rec.Body.String() != want {
		t.Fatalf("body = %q, want %q", rec.Body.String(), want)
	}
}

// TestMiddlewareFallsBackToRawRemoteAddr covers RemoteAddr values that don't
// split cleanly into host:port (e.g. a test harness or unusual proxy that
// sets a bare IP) — the middleware must still key on something rather than
// panic or silently allow everything through one shared "" key.
func TestMiddlewareFallsBackToRawRemoteAddr(t *testing.T) {
	p := NewPerIP(time.Minute, 1)
	handler := p.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-link", nil)
	req.RemoteAddr = "no-port-here"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec2.Code)
	}
}
