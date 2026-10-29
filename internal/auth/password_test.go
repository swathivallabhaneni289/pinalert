package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var phcDefault = regexp.MustCompile(`^\$argon2id\$v=19\$m=19456,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)

func TestHasherRoundTrip(t *testing.T) {
	h := NewHasher(TestParams())
	ctx := context.Background()

	a, err := h.Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, err := h.Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of one password are identical, the salt is not random")
	}

	ok, rehash, err := h.Verify(ctx, a, "correct horse battery")
	if err != nil || !ok || rehash {
		t.Fatalf("Verify same password: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	ok, _, err = h.Verify(ctx, a, "wrong horse battery")
	if err != nil || ok {
		t.Fatalf("Verify different password: ok=%v err=%v", ok, err)
	}
}

func TestHasherPHCFormat(t *testing.T) {
	h := NewHasher(DefaultParams)
	enc, err := h.Hash(context.Background(), "some long passphrase")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !phcDefault.MatchString(enc) {
		t.Fatalf("encoded hash does not match the PHC shape: %q", enc)
	}
	if !phcDefault.MatchString(h.DummyHash()) {
		t.Fatalf("dummy hash does not match the PHC shape: %q", h.DummyHash())
	}
}

func TestHasherRejectsMalformed(t *testing.T) {
	h := NewHasher(TestParams())
	good, err := h.Hash(context.Background(), "a fine password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	parts := strings.Split(good, "$")
	salt, key := parts[4], parts[5]

	cases := map[string]string{
		"empty":           "",
		"plain text":      "not a hash",
		"too few parts":   "$argon2id$v=19$m=64,t=1,p=1$" + salt,
		"too many parts":  good + "$extra",
		"first not empty": "x" + good,
		"unknown algo":    "$argon2i$v=19$m=64,t=1,p=1$" + salt + "$" + key,
		"bcrypt":          "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
		"wrong version":   "$argon2id$v=16$m=64,t=1,p=1$" + salt + "$" + key,
		"version text":    "$argon2id$v=abc$m=64,t=1,p=1$" + salt + "$" + key,
		"params text":     "$argon2id$v=19$m=abc,t=1,p=1$" + salt + "$" + key,
		"params missing":  "$argon2id$v=19$m=64$" + salt + "$" + key,
		"zero threads":    "$argon2id$v=19$m=64,t=1,p=0$" + salt + "$" + key,
		"zero time":       "$argon2id$v=19$m=64,t=0,p=1$" + salt + "$" + key,
		"bad salt":        "$argon2id$v=19$m=64,t=1,p=1$!!!$" + key,
		"bad key":         "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$!!!",
		"empty salt":      "$argon2id$v=19$m=64,t=1,p=1$$" + key,
		"empty key":       "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$",
	}
	for name, enc := range cases {
		t.Run(name, func(t *testing.T) {
			ok, rehash, err := h.Verify(context.Background(), enc, "a fine password")
			if !errors.Is(err, ErrMalformedHash) {
				t.Fatalf("err = %v, want ErrMalformedHash", err)
			}
			if ok || rehash {
				t.Fatalf("ok=%v rehash=%v on a malformed hash", ok, rehash)
			}
		})
	}
}

func TestHasherCapsParsedParams(t *testing.T) {
	h := NewHasher(TestParams())
	parts := strings.Split(mustHash(t, h, "x-password-1"), "$")
	salt, key := parts[4], parts[5]

	for name, params := range map[string]string{
		"memory bomb": "m=4194304,t=1,p=1",
		"just over":   "m=262145,t=1,p=1",
		"time bomb":   "m=64,t=11,p=1",
	} {
		t.Run(name, func(t *testing.T) {
			enc := "$argon2id$v=19$" + params + "$" + salt + "$" + key
			start := time.Now()
			ok, _, err := h.Verify(context.Background(), enc, "x-password-1")
			if time.Since(start) > 500*time.Millisecond {
				t.Fatalf("took %v, the cap did not stop the hash", time.Since(start))
			}
			if !errors.Is(err, ErrMalformedHash) || ok {
				t.Fatalf("ok=%v err=%v, want ErrMalformedHash", ok, err)
			}
		})
	}
}

func mustHash(t *testing.T, h *Hasher, pw string) string {
	t.Helper()
	enc, err := h.Hash(context.Background(), pw)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	return enc
}

func TestHasherNeedsRehash(t *testing.T) {
	weak := NewHasher(TestParams())
	strong := NewHasher(DefaultParams)

	enc := mustHash(t, weak, "needs a rehash")
	ok, rehash, err := strong.Verify(context.Background(), enc, "needs a rehash")
	if err != nil || !ok || !rehash {
		t.Fatalf("weak hash under strong hasher: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	ok, rehash, err = weak.Verify(context.Background(), enc, "needs a rehash")
	if err != nil || !ok || rehash {
		t.Fatalf("weak hash under weak hasher: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
}

func TestHasherSemaphoreBoundsConcurrency(t *testing.T) {
	h := NewHasher(TestParams())
	var inside, peak atomic.Int32
	h.slotHook = func() func() {
		n := inside.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		return func() { inside.Add(-1) }
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.Hash(context.Background(), "concurrent password"); err != nil {
				t.Errorf("Hash: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := peak.Load(); got > hashSlots {
		t.Fatalf("peak concurrency %d, want at most %d", got, hashSlots)
	}
	if got := peak.Load(); got < 2 {
		t.Fatalf("peak concurrency %d, the hook never saw callers overlap", got)
	}

	// A cancelled context while waiting for a slot gives ctx.Err().
	release := make(chan struct{})
	entered := make(chan struct{}, hashSlots)
	h.slotHook = func() func() {
		entered <- struct{}{}
		<-release
		return func() {}
	}
	for i := 0; i < hashSlots; i++ {
		go func() { _, _ = h.Hash(context.Background(), "holder password") }()
	}
	for i := 0; i < hashSlots; i++ {
		<-entered
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := h.Hash(ctx, "waiting password")
		done <- err
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled waiter did not return")
	}

	// W6: all slots held, a short wait gives ErrHasherBusy and never runs argon2.
	busy := NewHasher(TestParams(), WithSlotWait(50*time.Millisecond))
	var ran atomic.Int32
	hold := make(chan struct{})
	taken := make(chan struct{}, hashSlots)
	busy.slotHook = func() func() {
		ran.Add(1)
		taken <- struct{}{}
		<-hold
		return func() {}
	}
	for i := 0; i < hashSlots; i++ {
		go func() { _, _ = busy.Hash(context.Background(), "holder password") }()
	}
	for i := 0; i < hashSlots; i++ {
		<-taken
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	start := time.Now()
	_, err := busy.Hash(ctx2, "fourth caller")
	elapsed := time.Since(start)
	if !errors.Is(err, ErrHasherBusy) {
		t.Fatalf("Hash err = %v, want ErrHasherBusy", err)
	}
	if elapsed < 40*time.Millisecond || elapsed > time.Second {
		t.Fatalf("busy after %v, want about 50ms", elapsed)
	}
	_, _, err = busy.Verify(ctx2, busy.DummyHash(), "fourth caller")
	if !errors.Is(err, ErrHasherBusy) {
		t.Fatalf("Verify err = %v, want ErrHasherBusy", err)
	}
	if got := ran.Load(); got != hashSlots {
		t.Fatalf("argon2 slots entered %d times, want %d (the fourth must never run)", got, hashSlots)
	}
	close(hold)
	close(release)
}

func TestHasherDummyHashVerifiesFalse(t *testing.T) {
	h := NewHasher(TestParams())
	d := h.DummyHash()
	if d == "" {
		t.Fatal("empty dummy hash")
	}
	if d != h.DummyHash() {
		t.Fatal("dummy hash changes between calls, it must be made once")
	}
	for _, pw := range []string{"", "anything", "password", "dummy"} {
		ok, rehash, err := h.Verify(context.Background(), d, pw)
		if err != nil || ok || rehash {
			t.Fatalf("dummy verified %q: ok=%v rehash=%v err=%v", pw, ok, rehash, err)
		}
	}
}

func TestDefaultParamsFloor(t *testing.T) {
	p := DefaultParams
	primary := p.MemoryKiB >= 19456 && p.Time >= 2 && p.Threads >= 1
	alternate := p.MemoryKiB >= 12288 && p.Time >= 3 && p.Threads >= 1
	if !primary && !alternate {
		t.Fatalf("DefaultParams %+v is below the OWASP argon2id floor", p)
	}
	if TestParams() == DefaultParams {
		t.Fatal("TestParams equals DefaultParams, so a test cost could ship")
	}
	if h := NewHasher(DefaultParams); h.Params() != DefaultParams {
		t.Fatalf("Params() = %+v", h.Params())
	}
}
