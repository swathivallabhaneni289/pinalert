-- name: ResolutionUndoCounts :many
-- For one account and a batch of report ids, how many take backs and how many
-- reopens that account has written, counted from the append only votes log.
-- Four points a later reader would otherwise "correct":
--
-- (a) What it feeds. The two undo limit of D-22: a person may take back a
-- mistaken Mark resolved twice per report. The count comes straight from the
-- votes table, which is append only, so no new table and no counter column is
-- needed (D-28). It counts every row, not the latest row per kind, because the
-- limit is about how many times the person did it, not what stands now.
--
-- (b) Why reopen is counted separately. Only the poster's reopen spends the
-- budget, and who the poster is comes from ReportVoteContext against the
-- verified account, which is Go's decision. The query returns both counts and
-- stays blind to that, so a non poster's earlier reopen rows are returned but
-- never added to the budget.
--
-- (c) The index. idx_votes_account_kind_report_created (migration 00006)
-- serves the equality on account_id and kind and the report id range. The
-- two values and the kind are parameters, never SQL literals, because the
-- closed vote vocabulary lives in Go (VotedReportsByAccount comment (b)).
--
-- (d) What it leaves out. It selects no voter cell and no other account's
-- rows: the filter is the caller's own account id, and the output is a report
-- id and two counts. A report id with no rows of that account returns no row.
SELECT
    report_id,
    COUNT(*) FILTER (WHERE value = sqlc.arg(unresolve_value))::bigint AS unresolves,
    COUNT(*) FILTER (WHERE value = sqlc.arg(reopen_value))::bigint AS reopens
FROM votes
WHERE account_id = sqlc.arg(account_id)
  AND kind = sqlc.arg(kind)
  AND report_id = ANY(sqlc.arg(report_ids)::bigint[])
GROUP BY report_id
ORDER BY report_id;
