-- Source: github.com/pressly/goose README [CITED]
-- +goose Up
-- Serves ReporterHistoryReports (D-10, D-13). The equality on session_id comes
-- first, then expires_at DESC, id DESC, mirroring the per session LATERAL's
-- ORDER BY r.expires_at DESC, r.id DESC, so each session scan reads at most
-- the row cap with no sort. The trailing id matches the query's tie break.
-- The column order and directions mirror the query and are not a style
-- choice.
--
-- The build runs inside goose's transaction, which is fine at this table
-- size. A large production table would want a non blocking build run outside
-- a transaction.
CREATE INDEX idx_reports_session_expires ON reports (session_id, expires_at DESC, id DESC);

-- +goose Down
DROP INDEX idx_reports_session_expires;
