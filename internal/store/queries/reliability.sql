-- name: ReporterHistoryReports :many
-- A bounded read of the latest expired reports of a batch of reporter
-- accounts, one query for the whole batch. Six choices a later reader would
-- otherwise "correct":
--
-- (a) Keyed to the verified account. The rows are found through
-- sessions.account_id across every session an account holds, never by
-- session_id, because one account can hold several sessions and one person
-- must not be able to split a record across devices (D-10, T-03-10). A
-- session with no bound account never matches the account list, so it
-- contributes nothing.
--
-- (b) Why a per session LATERAL with a LIMIT, then a window function per
-- account. The LATERAL forces an equality on session_id, so the planner uses
-- idx_reports_session_expires (migration 00005) and each session scan stops
-- at the row cap. The other shape that was tried drove from
-- idx_reports_expires_at and read far more rows. An account's latest N
-- reports are always among the union of its sessions' latest N, so the
-- window function can then pick the account's own latest N across sessions.
--
-- (c) The window. A report is judged once expires_at is at or before now, so
-- the upper edge is inclusive (R-05). Only the last 90 days count, measured
-- from expires_at, and the lower edge is exclusive (D-13).
--
-- (d) Every bound is a parameter. now is captured once per request in Go, so
-- an injected clock reaches the query. The window start is now minus the
-- reliability window and the row cap is the reliability row cap. The
-- statement therefore holds no clock call and no literal bound.
--
-- (e) The cap counts the latest expired reports inside the window, not
-- judged reports. Neutral classification of critical and rescue needed
-- reports that expired unconfirmed (D-12) needs vote data, so it happens in
-- Go. The caller therefore passes a read cap larger than the judged report
-- cap (100 against 30, WR-05): Go drops the neutral rows and keeps the
-- latest 30 judged ones. More than 100 neutral rows in a row can still push
-- an old record out of this read.
--
-- (f) No session id, voter cell or text in the output, continuing T-01-02.
-- Independence is never aggregated in SQL. The votes for these report ids
-- are read separately by one CurrentVotesForReports call (plan 03-12).
SELECT t.account_id::bigint AS account_id, t.report_id, t.severity, t.category, t.expires_at
FROM (
    SELECT x.account_id, rr.id AS report_id, rr.severity, rr.category, rr.expires_at,
           row_number() OVER (PARTITION BY x.account_id ORDER BY rr.expires_at DESC, rr.id DESC) AS rn
    FROM sessions x
    CROSS JOIN LATERAL (
        SELECT r.id, r.severity, r.category, r.expires_at
        FROM reports r
        WHERE r.session_id = x.session_id
          AND r.expires_at <= sqlc.arg(now)::timestamptz
          AND r.expires_at > sqlc.arg(window_start)::timestamptz
        ORDER BY r.expires_at DESC, r.id DESC
        LIMIT sqlc.arg(row_cap)::int
    ) rr
    WHERE x.account_id = ANY(sqlc.arg(account_ids)::bigint[])
) t
WHERE t.rn <= sqlc.arg(row_cap)::int
ORDER BY t.account_id, t.expires_at DESC, t.report_id DESC;
