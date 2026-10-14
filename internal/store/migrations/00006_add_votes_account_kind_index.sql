-- Source: github.com/pressly/goose README [CITED]
-- +goose Up
-- Serves VotedReportsByAccount, the Activity page's My votes and My disputes
-- read. The equality on account_id and kind comes first, then report_id,
-- created_at DESC, id DESC, mirroring the query's DISTINCT ON (report_id) and
-- its ORDER BY report_id, created_at DESC, id DESC, so the planner reads each
-- report's newest vote first with no sort. The column order and directions
-- mirror the query and are not a style choice.
--
-- The build runs inside goose's transaction, which is fine at this table
-- size. A large production table would want a non blocking build run outside
-- a transaction.
CREATE INDEX idx_votes_account_kind_report_created ON votes (account_id, kind, report_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX idx_votes_account_kind_report_created;
