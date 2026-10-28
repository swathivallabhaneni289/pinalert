package demo

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// The store keeps every live Demo world in memory and bounds what it holds:
// at most MaxWorlds worlds, a world idle for WorldIdleTTL dropped, the least
// recently used world evicted first when a new visit arrives at the ceiling,
// and, inside each world, the report and vote ceilings of memquerier.go.
//
// A Demo visitor has no account, so there is no identity to rate limit by, and
// per address limits are weak behind carrier shared addresses (research G5,
// gotcha 7). What must be protected is memory, so memory is what is capped.
//
// There is no goroutine and no timer per world and none for the store: idle
// worlds are dropped lazily, inside Create and Get, under the store's lock.

const (
	// MaxWorlds is the most worlds the store keeps at once.
	MaxWorlds = 100
	// WorldIdleTTL is how long a world may go unused before it is dropped.
	WorldIdleTTL = 30 * time.Minute
	// VisitIDBytes is the random length of a visit id, 128 bits.
	VisitIDBytes = 16
)

// visitIDLen is the length of VisitIDBytes bytes in unpadded base64url.
const visitIDLen = 22

// Store holds the live worlds by visit id.
type Store struct {
	mu     sync.Mutex
	now    func() time.Time
	setup  func(*World)
	worlds map[string]*World
}

// StoreOption configures a Store at construction.
type StoreOption func(*Store)

// WithStoreClock sets the clock the store judges idle time with and the wall
// clock its worlds are offset from. It exists for tests.
func WithStoreClock(now func() time.Time) StoreOption {
	return func(s *Store) { s.now = now }
}

// WithWorldSetup sets a function that runs once on each new world before the
// store keeps it. The sample data of a Demo visit is written this way.
func WithWorldSetup(fn func(*World)) StoreOption {
	return func(s *Store) { s.setup = fn }
}

// NewStore builds an empty store.
func NewStore(opts ...StoreOption) *Store {
	s := &Store{now: time.Now, worlds: make(map[string]*World)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ValidVisitID reports whether id has the shape of an id the store hands out:
// exactly 22 characters from A to Z, a to z, 0 to 9, minus and underscore.
func ValidVisitID(id string) bool {
	if len(id) != visitIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// newVisitID returns VisitIDBytes bytes from the system's cryptographic source
// in unpadded base64url. The id is not a secret, since it guards only a sandbox
// with nothing in it, but it must not be guessable in sequence.
func newVisitID() (string, error) {
	b := make([]byte, VisitIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create starts a new world for a new visit. It drops idle worlds first and,
// when the store is still at the ceiling, evicts the least recently used one.
func (s *Store) Create() (*World, error) {
	id, err := newVisitID()
	if err != nil {
		return nil, err
	}
	w := NewWorld(id, s.now)
	if s.setup != nil {
		s.setup(w)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.sweepLocked(now)
	if _, taken := s.worlds[id]; taken {
		return nil, errors.New("demo: visit id collision")
	}
	for len(s.worlds) >= MaxWorlds {
		s.evictOldestLocked()
	}
	w.created = now
	w.lastUsed = now
	s.worlds[id] = w
	return w, nil
}

// Get returns the world of a visit and marks it used. A malformed, unknown or
// idle world gives false.
func (s *Store) Get(id string) (*World, bool) {
	if !ValidVisitID(id) {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.sweepLocked(now)
	w, ok := s.worlds[id]
	if !ok {
		return nil, false
	}
	w.lastUsed = now
	return w, true
}

// Len is the number of worlds held now.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.worlds)
}

// sweepLocked drops every world idle for WorldIdleTTL or longer.
func (s *Store) sweepLocked(now time.Time) {
	for id, w := range s.worlds {
		if now.Sub(w.lastUsed) >= WorldIdleTTL {
			delete(s.worlds, id)
		}
	}
}

// evictOldestLocked drops the least recently used world; a tie goes to the
// older world, then to the lower id, so the choice does not depend on map order.
func (s *Store) evictOldestLocked() {
	var victim *World
	for _, w := range s.worlds {
		if victim == nil ||
			w.lastUsed.Before(victim.lastUsed) ||
			(w.lastUsed.Equal(victim.lastUsed) && w.created.Before(victim.created)) ||
			(w.lastUsed.Equal(victim.lastUsed) && w.created.Equal(victim.created) && w.ID < victim.ID) {
			victim = w
		}
	}
	if victim != nil {
		delete(s.worlds, victim.ID)
	}
}
