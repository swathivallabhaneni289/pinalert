// historyread.go holds the history reads of the report service (plan 03.1-06):
// the viewer fields every read path fills, and the reads behind the Past tab,
// the empty Active tab counts and a report link. Membership of every view comes
// from Resolve and ViewOf only. There is no second visibility authority here
// and no SQL predicate behind it (T-02-04, D-03).

package service

import (
	"context"
	"time"
)

// attachUndos fills UndosLeft on every view and reads the undo counts only
// when they can matter (D-22). A view is a candidate when it has not expired
// and the viewer either has a standing resolution row on it or posted it, which
// are the only two cases where a take back or a poster's reopen can exist. The
// common page has no candidate, so it returns at once and costs zero reads.
// Every other view reads UndoLimit, because nothing the viewer did can have
// spent the budget. The viewer comes from the gate, never from a query value
// (T-31-27); 0 means no identified viewer and never reads.
func (s *ReportService) attachUndos(ctx context.Context, viewerAccountID int64, views []ReportView, now time.Time) error {
	for i := range views {
		views[i].UndosLeft = UndoLimit
	}
	if viewerAccountID == 0 {
		return nil
	}
	var ids []int64
	for _, v := range views {
		if !v.ExpiresAt.After(now) {
			continue
		}
		if v.ViewerResolution == "" && !v.IsOwnReport {
			continue
		}
		ids = append(ids, v.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ResolutionUndoCounts(ctx, undoCountParams(viewerAccountID, ids))
	if err != nil {
		return err
	}
	for i := range views {
		for _, row := range rows {
			if row.ReportID != views[i].ID {
				continue
			}
			views[i].UndosLeft = UndosLeft(UndosUsed(row, views[i].IsOwnReport))
			break
		}
	}
	return nil
}
