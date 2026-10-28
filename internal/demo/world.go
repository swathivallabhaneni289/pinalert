package demo

import (
	"strconv"
	"sync/atomic"
	"time"

	"pinalert/internal/service"
)

// Synthetic account ids. They exist only inside a World's memory and are never
// written to any table. They sit far above any id the accounts sequence will
// reach, so a synthetic id can never be mistaken for a real account.
const (
	// VisitorAccountID is the one account the visitor acts as in a Demo.
	VisitorAccountID int64 = 1_000_000_001
	// NeighbourAccountBase is the first of the pretend neighbours who confirm.
	NeighbourAccountBase int64 = 1_000_000_100
	// SeedReporterAccountBase is the first of the accounts that posted the sample reports.
	SeedReporterAccountBase int64 = 1_000_000_200
	// SeedVoterAccountBase is the first of the accounts that voted on the sample reports.
	SeedVoterAccountBase int64 = 1_000_000_300
)

// VisitorSessionPrefix starts every session id a world uses. The session of an
// account is the prefix plus its decimal id (SessionFor), so it can never equal
// the random session id of a real visitor.
const VisitorSessionPrefix = "demo-"

// World is one visit's sandbox. Reports and Votes are the real services, built
// on the in memory querier with no text screener and no change notifier: the
// practice text is checked by the local prefilter only and goes to no vendor,
// and no phone alert is ever queued from a Demo.
type World struct {
	ID      string
	Reports *service.ReportService
	Votes   *service.VotingService

	q      *memQuerier
	wall   func() time.Time
	offset atomic.Int64 // nanoseconds added to the wall clock

	// created and lastUsed are kept by the Store under its own lock.
	created  time.Time
	lastUsed time.Time
}

// NewWorld builds an empty world. wallNow is the clock the world is offset
// from; nil means time.Now.
func NewWorld(id string, wallNow func() time.Time) *World {
	if wallNow == nil {
		wallNow = time.Now
	}
	w := &World{ID: id, wall: wallNow}
	w.q = newMemQuerier(w.Now)
	w.Reports = service.NewReportService(w.q, service.WithReportClock(w.Now))
	w.Votes = service.NewVotingService(w.q, service.WithVotingClock(w.Now))
	return w
}

// Now is the world clock: the wall clock plus the offset. It takes no lock the
// querier holds, so the querier may call it while locked.
func (w *World) Now() time.Time {
	return w.wall().Add(time.Duration(w.offset.Load()))
}

// SetClockOffset moves the world clock to the wall clock plus d. Seeding a
// world sets it to the past while the sample is written ("20 minutes ago"), then
// back to zero.
func (w *World) SetClockOffset(d time.Duration) {
	w.offset.Store(int64(d))
}

// SessionFor returns the session id the world uses for accountID and binds it
// to that account, so a report posted under it has that reporter. Calling it
// again for the same account gives the same session.
func (w *World) SessionFor(accountID int64) string {
	session := VisitorSessionPrefix + strconv.FormatInt(accountID, 10)
	w.q.BindSession(session, accountID)
	return session
}
