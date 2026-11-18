-- +goose Up
-- Quick 261118-saj (owner, 2026-11-18, 7.1 UAT): flags from three different
-- places never hide a serious report (severity critical, or the retired
-- rescue_needed category, ReportMeta.Serious). Such a report stays shown and
-- gets the Being checked mark instead, its phone alerts stop until a
-- moderator looks, and it goes to the top of the moderator's flagged list.
-- Non serious reports keep the review hide of 00026 exactly.
--
-- flag_check_at is when flags put the mark on the report. The mark is on while
-- flag_check_at IS NOT NULL AND review_kept_at IS NULL. A moderator's Keep
-- stamps review_kept_at, the one settled marker of 00026, which takes the mark
-- off and stops flags from marking or hiding the report again, and leaves
-- flag_check_at as it is: a settled report with flag_check_at set was kept
-- from the mark and alerts again, while a settled report without it was kept
-- after a hide or restored and stays silent, as before. A Remove, a late text
-- check removal and a Restore clear flag_check_at.
ALTER TABLE reports
    ADD COLUMN flag_check_at TIMESTAMPTZ;

-- Nothing serious stays hidden by flags: a serious report hidden for review
-- moves back to shown with the mark on, so a moderator still looks at it.
UPDATE reports
SET moderation_state = 'clear',
    flag_check_at = now()
WHERE moderation_state = 'review'
  AND (severity = 'critical' OR category = 'rescue_needed');

-- +goose Down
-- A marked report stays clear (shown); only the mark goes.
ALTER TABLE reports
    DROP COLUMN flag_check_at;
