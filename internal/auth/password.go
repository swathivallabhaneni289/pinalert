package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// Params are the argon2id cost parameters. They are written into every stored
// hash, so a stored hash always carries the cost it was made with and Verify
// can tell when it is older than the current cost (needsRehash).
type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
}

// DefaultParams is the OWASP primary argon2id setting: 19 MiB, 2 passes, one
// lane (D-02, IDENT-06, threat T-07.1-12). TestDefaultParamsFloor pins it, so a
// cheaper setting cannot ship by accident. The equal-strength alternative is
// m=12288, t=3, which research Pitfall 14 names as the first thing to try if
// the host reports out-of-memory kills.
var DefaultParams = Params{MemoryKiB: 19456, Time: 2, Threads: 1}

// TestParams is a deliberately cheap setting so the unit tests and the e2e
// harness do not spend 40 ms and 19 MiB on every hash. It is below the OWASP
// floor and must never be passed to a production Hasher: no non test file other
// than this one may call it (checked by this plan's acceptance grep).
func TestParams() Params {
	return Params{MemoryKiB: 64, Time: 1, Threads: 1}
}

const (
	saltBytes = 16
	keyBytes  = 32

	// maxParsedMemoryKiB and maxParsedTime cap what a stored hash may ask of
	// the verifier. The stored string is data read back from the database; a
	// corrupted or crafted row must not become a memory bomb (T-07.1-11).
	maxParsedMemoryKiB = 262144 // 256 MiB
	maxParsedTime      = 10
	maxParsedBytes     = 64 // longest salt or key a stored hash may carry

	// hashSlots is how many argon2 runs may be in flight at once. Each costs
	// about 19 MiB, and the free host is small (research Pitfall 14, A2).
	hashSlots = 3

	// defaultSlotWait is how long a caller waits for a free slot before it
	// gets ErrHasherBusy (revision W6). It is well under the 10 second write
	// deadline of the production server.
	defaultSlotWait = 3 * time.Second
)

var (
	// ErrMalformedHash means a stored hash is not a well formed argon2id PHC
	// string within the parameter caps. Callers treat it as "does not match"
	// and log it; they never show it to the visitor.
	ErrMalformedHash = errors.New("auth: malformed password hash")

	// ErrHasherBusy means every hash slot stayed taken for the whole slot
	// wait. The API answers 503 "Something went wrong. Try again." so nothing
	// queues without bound (Pitfall 14, W6).
	ErrHasherBusy = errors.New("auth: password hasher is busy")
)

// Hasher hashes and verifies passwords with argon2id and a bounded number of
// concurrent runs. Construct it once at start up and share it.
type Hasher struct {
	p        Params
	slots    chan struct{}
	slotWait time.Duration
	dummy    string

	// slotHook is for tests only. It runs once a slot is held and just before
	// argon2 starts, and the function it returns runs when argon2 is done.
	slotHook func() func()
}

// HasherOption changes a Hasher at construction.
type HasherOption func(*Hasher)

// WithSlotWait sets how long a caller waits for a free slot (default 3 s).
func WithSlotWait(d time.Duration) HasherOption {
	return func(h *Hasher) { h.slotWait = d }
}

// NewHasher builds a Hasher with the given cost. It computes the dummy hash
// once, here, from a random throwaway password, with the same parameters as
// real hashes, so verifying an unknown address costs the same work as
// verifying a known one (D-02, T-07.1-04). If the system random source fails
// it panics at start up, like session.Manager.NewID: that is a broken host, not
// a request that can fail cleanly.
func NewHasher(p Params, opts ...HasherOption) *Hasher {
	h := &Hasher{
		p:        p,
		slots:    make(chan struct{}, hashSlots),
		slotWait: defaultSlotWait,
	}
	for _, o := range opts {
		o(h)
	}
	throwaway := make([]byte, 24)
	if _, err := rand.Read(throwaway); err != nil {
		panic("auth: no random source for the dummy hash: " + err.Error())
	}
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		panic("auth: no random source for the dummy hash: " + err.Error())
	}
	h.dummy = encodePHC(p, salt, argon2.IDKey(throwaway, salt, p.Time, p.MemoryKiB, p.Threads, keyBytes))
	return h
}

// Params returns the cost this Hasher hashes with.
func (h *Hasher) Params() Params { return h.p }

// DummyHash returns a valid PHC string made once at construction. Sign in
// verifies it when the address is unknown, so timing does not reveal which
// addresses have accounts. No real password ever matches it.
func (h *Hasher) DummyHash() string { return h.dummy }

// acquire takes a hash slot. A caller whose context ends gets ctx.Err(); a
// caller that waits the whole slot wait gets ErrHasherBusy.
func (h *Hasher) acquire(ctx context.Context) (release func(), err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timer := time.NewTimer(h.slotWait)
	defer timer.Stop()
	select {
	case h.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, ErrHasherBusy
	}
	done := func() {}
	if h.slotHook != nil {
		done = h.slotHook()
	}
	return func() {
		done()
		<-h.slots
	}, nil
}

// Hash returns the argon2id PHC string for password: a fresh 16 byte salt from
// crypto/rand, a 32 byte key, RawStdEncoding for both. The caller passes the
// NormalizePassword form of what the visitor typed.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	release, err := h.acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	key := argon2.IDKey([]byte(password), salt, h.p.Time, h.p.MemoryKiB, h.p.Threads, keyBytes)
	return encodePHC(h.p, salt, key), nil
}

// Verify checks password against a stored PHC string. The stored parameters are
// parsed and capped before any hashing (a bad row returns ErrMalformedHash and
// costs nothing), the key is recomputed with the stored cost under a slot, and
// compared in constant time. needsRehash is true when the stored cost differs
// from this Hasher's, so the caller can upgrade the stored hash after a good
// sign in.
func (h *Hasher) Verify(ctx context.Context, encoded, password string) (ok bool, needsRehash bool, err error) {
	p, salt, want, err := parsePHC(encoded)
	if err != nil {
		return false, false, err
	}
	release, err := h.acquire(ctx)
	if err != nil {
		return false, false, err
	}
	defer release()
	got := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	return true, p != h.p, nil
}

func encodePHC(p Params, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		p.MemoryKiB, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// parsePHC reads an argon2id PHC string. It refuses anything outside the shape
// this package writes, and any cost above the caps, before the caller hashes.
func parsePHC(encoded string) (p Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var version int
	if n, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || n != 1 || version != argon2.Version {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var m, t uint32
	var threads uint8
	if n, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &threads); err != nil || n != 3 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	if m == 0 || m > maxParsedMemoryKiB || t == 0 || t > maxParsedTime || threads == 0 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	salt, err = base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) == 0 || len(salt) > maxParsedBytes {
		return Params{}, nil, nil, ErrMalformedHash
	}
	key, err = base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(key) == 0 || len(key) > maxParsedBytes {
		return Params{}, nil, nil, ErrMalformedHash
	}
	p = Params{MemoryKiB: m, Time: t, Threads: threads}
	// Sscanf ignores trailing text and leading zeros; only the exact string
	// this package writes is accepted.
	if encodePHC(p, salt, key) != encoded {
		return Params{}, nil, nil, ErrMalformedHash
	}
	return p, salt, key, nil
}
