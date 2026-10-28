package official

// Who is told on their phone when an official alert arrives (plan 05-11, owner
// decisions D-11 and D-16). PushEnqueueHook is a VersionHook: it runs inside the
// transaction that stores each alert version, so the queued rows commit or roll
// back with the version. A crash can neither lose a push nor double one (research
// G7 point 10), and the unique index on (account, alert, cap identifier) is the
// second guard.
//
// The D-16 rule in plain words. A version is queued only when it is a real Alert
// or Update, Severe or Extreme, sent within the last hour and ending more than
// ten minutes from now. If it is placed on districts, the people whose home area
// touches one of those districts are told. If it is placed on a whole state, it
// is pushed only when it is Extreme, and the people whose home is in that state
// are told. A state level Severe alert queues nobody; it is on the Nearby banner
// and on the alerts page only. An unplaced alert is never pushed. A cancelled
// alert, an alert a stored message replaces, and an alert whose issuer is outside
// the closed table (IMD, CWC or a state SDMA) are never pushed either.
//
// A later version of an alert queues a new push only when it escalates: it
// raises the severity to Severe or Extreme, or it adds a district. A wording only
// Update queues nothing, so there is never a stack of near duplicate
// notifications. The version is judged against the earlier version under its own
// guid or, for an Update under a new guid, against the stored alerts its
// references name. An escalated row is still queued; plan 05-16 sends it only to
// devices that replace a notification by its tag, and an Apple endpoint gets only
// the first official push about an alert series and learns of the raise from the
// Nearby banner and the alerts page (owner rule of 2026-10-28, few notifications
// that never pile up).
//
// Recipients are chosen by their stored home cell, never by a position: the cell
// is turned into districts at its centre and four corners through areas, the
// same rule the reader view uses. Nobody's cell is returned, logged or kept. At
// most OfficialPushRecipientsPerAlert people are queued for one version.
//
// Logs hold the alert id, counts and one closed class word, never a home cell, a
// place name or alert text. The feed's headline, description and instruction are
// not read here at all: the lock screen words are built later from closed tables.

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"pinalert/internal/areas"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// The reasons a push is queued. They are the two values the table accepts.
const (
	pushReasonNew       = "new"
	pushReasonEscalated = "escalated"
)

// recipientCap is the most people one version is queued for. A package variable
// only so a test can lower it; production code never changes it.
var recipientCap = service.OfficialPushRecipientsPerAlert

// candidateFactor is how many candidates are read for each recipient wanted: most
// accounts with a home area live elsewhere, so the read is wider than the cap.
const candidateFactor = 4

// PushEnqueueHook returns the hook that queues official pushes inside the
// transaction of each stored alert version. It returns nil for everything that is
// not a database error (a version that does not qualify, an issuer outside the
// table, nobody to tell), because an error rolls the version back and the refresh
// would count it as failed.
func PushEnqueueHook(ix *areas.Index) VersionHook {
	if ix == nil {
		return func(context.Context, *sqlcgen.Queries, SavedVersion, *SavedVersion, time.Time) error { return nil }
	}
	return func(ctx context.Context, q *sqlcgen.Queries, saved SavedVersion, prev *SavedVersion, now time.Time) error {
		// Most versions end here, before the database is touched.
		if _, ok := qualifies(saved, nil, now); !ok {
			return nil
		}
		if service.OfficialIssuer(saved.Version.Sender, ix) == "" {
			log.Printf("official: push alert=%d not queued class=issuer", saved.ID)
			return nil
		}

		row, err := q.OfficialAlertForPush(ctx, saved.ID)
		if err != nil {
			return fmt.Errorf("official: reading the stored alert for the push queue: %w", err)
		}
		if row.CancelledAt.Valid || row.SupersededAt.Valid {
			log.Printf("official: push alert=%d not queued class=withdrawn", saved.ID)
			return nil
		}
		gone, err := q.OfficialAlertWithdrawnByStored(ctx, sqlcgen.OfficialAlertWithdrawnByStoredParams{
			Guid: saved.Version.GUID, CapIdentifier: saved.Version.CapIdentifier})
		if err != nil {
			return fmt.Errorf("official: checking whether the alert was withdrawn: %w", err)
		}
		if gone {
			log.Printf("official: push alert=%d not queued class=withdrawn", saved.ID)
			return nil
		}

		if prev == nil {
			if prev, err = predecessorOf(ctx, q, saved); err != nil {
				return err
			}
		}
		reason, ok := qualifies(saved, prev, now)
		if !ok {
			return nil
		}
		scoped := recipientScope(saved, prev, reason)

		candidates, err := q.OfficialPushCandidates(ctx, int32(recipientCap*candidateFactor))
		if err != nil {
			return fmt.Errorf("official: reading the push candidates: %w", err)
		}
		expires := saved.Version.ExpiresAt
		if limit := now.Add(service.OfficialPushTTL); limit.Before(expires) {
			expires = limit
		}
		cells := map[string][]areas.District{} // each distinct home cell is decoded once
		queued := 0
		for _, c := range candidates {
			if queued >= recipientCap {
				break
			}
			ds, seen := cells[c.HomeAreaCell]
			if !seen {
				ds = ix.CellDistricts(c.HomeAreaCell)
				cells[c.HomeAreaCell] = ds
			}
			if !matchDistricts(ds, scoped) {
				continue
			}
			queued++
			if _, err := q.EnqueueOfficialPush(ctx, sqlcgen.EnqueueOfficialPushParams{
				AccountID:     c.AccountID,
				AlertID:       saved.ID,
				CapIdentifier: saved.Version.CapIdentifier,
				Reason:        reason,
				ExpiresAt:     expires,
				Now:           now,
			}); err != nil {
				return fmt.Errorf("official: queuing a push: %w", err)
			}
		}
		log.Printf("official: push alert=%d queued reason=%s recipients=%d", saved.ID, reason, queued)
		return nil
	}
}

// pushSeverityRank orders the severities a push cares about: Severe is 1, Extreme is
// 2, and everything below Severe is 0.
func pushSeverityRank(s Severity) int {
	switch s {
	case SeverityExtreme:
		return 2
	case SeveritySevere:
		return 1
	}
	return 0
}

// qualifies is the D-16 rule for one version. prev is the version it replaced, or
// nil for a series seen the first time. It returns the reason the version is
// queued, "new" or "escalated", and false when it queues nobody. It looks at no
// feed text and reads no clock but the one passed in.
func qualifies(saved SavedVersion, prev *SavedVersion, now time.Time) (string, bool) {
	v := saved.Version
	if v.MsgType != MsgAlert && v.MsgType != MsgUpdate {
		return "", false
	}
	rank := pushSeverityRank(v.Severity)
	if rank == 0 {
		return "", false
	}
	switch saved.Placement.Tier {
	case TierDistrict:
		if len(saved.Placement.DistrictCodes) == 0 {
			return "", false
		}
	case TierState:
		// A state level alert reaches a phone only when it is Extreme (D-16).
		if v.Severity != SeverityExtreme || len(saved.Placement.StateCodes) == 0 {
			return "", false
		}
	default:
		return "", false
	}
	if now.Sub(v.SentAt) > service.OfficialPushSentWindow {
		return "", false
	}
	if v.ExpiresAt.Sub(now) <= service.OfficialPushExpiryMargin {
		return "", false
	}

	if prev == nil || prev.Version.MsgType == MsgCancel {
		return pushReasonNew, true
	}
	if rank > pushSeverityRank(prev.Version.Severity) {
		return pushReasonEscalated, true
	}
	if saved.Placement.Tier == TierDistrict && len(newlyCovered(prev, saved)) > 0 {
		return pushReasonEscalated, true
	}
	return "", false
}

// newlyCovered returns the districts of saved that prev did not name, sorted. A
// nil prev covers nothing, so every district of saved is new. It is nil when
// nothing was added.
func newlyCovered(prev *SavedVersion, saved SavedVersion) []int {
	had := map[int]bool{}
	if prev != nil {
		for _, c := range prev.Placement.DistrictCodes {
			had[c] = true
		}
	}
	var added []int
	for _, c := range saved.Placement.DistrictCodes {
		if !had[c] {
			added = append(added, c)
		}
	}
	sort.Ints(added)
	return added
}

// recipientScope narrows a version to the districts whose people are told. A new
// alert, or a later version that raised the severity, reaches everyone its
// placement names. A later version that only added districts reaches only the
// people in the added ones.
func recipientScope(saved SavedVersion, prev *SavedVersion, reason string) SavedVersion {
	if reason != pushReasonEscalated || prev == nil || prev.Version.MsgType == MsgCancel ||
		pushSeverityRank(saved.Version.Severity) > pushSeverityRank(prev.Version.Severity) {
		return saved
	}
	out := saved
	out.Placement = Placement{
		Tier:          saved.Placement.Tier,
		DistrictCodes: newlyCovered(prev, saved),
		StateCodes:    saved.Placement.StateCodes,
	}
	return out
}

// homeMatches reports whether a stored home cell falls in the places the version
// names: one of its districts for a district placed alert, its state for a state
// placed one, never for an unplaced one. A district placed alert also carries the
// states of its districts, and those do not widen the match. A cell that is not
// a cell matches nothing.
func homeMatches(cell string, saved SavedVersion, ix *areas.Index) bool {
	return matchDistricts(ix.CellDistricts(cell), saved)
}

// matchDistricts is homeMatches for the districts of a cell already decoded.
func matchDistricts(ds []areas.District, saved SavedVersion) bool {
	switch saved.Placement.Tier {
	case TierDistrict:
		for _, d := range ds {
			for _, c := range saved.Placement.DistrictCodes {
				if d.Code == c {
					return true
				}
			}
		}
	case TierState:
		for _, d := range ds {
			for _, c := range saved.Placement.StateCodes {
				if d.StateCode == c {
					return true
				}
			}
		}
	}
	return false
}

// predecessorOf stands in for the earlier version of an Update that arrives under
// a new guid: the stored alerts its references name, merged into one version with
// the highest severity and the union of their districts and states. It is nil for
// anything that is not an Update with references, and when no referenced alert is
// stored.
func predecessorOf(ctx context.Context, q *sqlcgen.Queries, saved SavedVersion) (*SavedVersion, error) {
	v := saved.Version
	if v.MsgType != MsgUpdate || len(v.References) == 0 {
		return nil, nil
	}
	rows, err := q.OfficialPredecessorsForPush(ctx, sqlcgen.OfficialPredecessorsForPushParams{
		Identifiers: v.References, Guid: v.GUID})
	if err != nil {
		return nil, fmt.Errorf("official: reading the alerts an update replaces: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	merged := &SavedVersion{
		Version:   Version{MsgType: MsgAlert, Severity: SeverityUnknown},
		Placement: Placement{Tier: TierUnplaced},
	}
	districts, states := map[int]bool{}, map[int]bool{}
	for _, r := range rows {
		if sev := ParseSeverity(r.Severity); pushSeverityRank(sev) > pushSeverityRank(merged.Version.Severity) {
			merged.Version.Severity = sev
		}
		switch Tier(r.PlacementTier) {
		case TierDistrict:
			merged.Placement.Tier = TierDistrict
		case TierState:
			if merged.Placement.Tier == TierUnplaced {
				merged.Placement.Tier = TierState
			}
		}
		for _, c := range r.DistrictCodes {
			districts[int(c)] = true
		}
		for _, c := range r.StateCodes {
			states[int(c)] = true
		}
	}
	merged.Placement.DistrictCodes = sortedKeys(districts)
	merged.Placement.StateCodes = sortedKeys(states)
	return merged, nil
}
