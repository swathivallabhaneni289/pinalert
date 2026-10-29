-- +goose Up
-- Phase 8 plan 08 (PAGE-02, D-02, D-12, D-14, D-15): comments on a report, the
-- likes on a comment, the flags on a comment and the moderator's Keep and
-- Remove trail for a comment.
--
-- Comments never feed any trust number (D-02, research section 5). They have
-- their own tables and no foreign key runs from votes, reports or any trust
-- read to them, so a comment or a like can never change a confirmation count,
-- a currency stage, a reliability score or whether a report is visible.
--
-- The held check lives on the comment row itself. moderation_checks is keyed
-- and locked by report, and three race tests pin that shape, so a comment
-- cannot reuse it (research section 5). The check columns below are copies of
-- the moderation_checks ones, and the worker of a later plan leases and
-- settles a comment row the same way it does a report row.
--
-- parent_id is always the thread root, never a reply, so threads are exactly
-- one level deep (D-12). reply_to_id remembers which comment was actually
-- answered, so a later plan can tell that comment's author and show the
-- "replying to" name. Both are set together or both are NULL.
--
-- A deleted comment keeps its row with empty words (D-14): its likes, flags and
-- audit rows stay valid, and a reply under it can still be read in context.
-- Who it was by is never shown for such a row. Likes are one per account per
-- comment, by the primary key. No ON DELETE and no CONCURRENTLY (repo
-- convention; goose runs this in a transaction).

-- One comment. moderation_state follows the report rule: clear is visible to
-- everyone, pending is visible to its author only until the text check
-- settles, removed is a moderator's decision.
CREATE TABLE comments (
    id               BIGSERIAL PRIMARY KEY,
    report_id        BIGINT NOT NULL REFERENCES reports(id),
    account_id       BIGINT NOT NULL REFERENCES accounts(id),
    parent_id        BIGINT REFERENCES comments(id),
    reply_to_id      BIGINT REFERENCES comments(id),
    body             TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    moderation_state TEXT NOT NULL DEFAULT 'clear',
    needs_review     BOOLEAN NOT NULL DEFAULT false,
    check_attempts   INTEGER NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ,
    lease_until      TIMESTAMPTZ,
    -- An error class such as a status code, never comment text.
    last_error       TEXT,
    verdict          TEXT,
    scores           JSONB,
    model            TEXT,
    checked_at       TIMESTAMPTZ,
    CONSTRAINT comments_state_valid CHECK (moderation_state IN ('clear', 'pending', 'removed')),
    -- A live comment holds 1 to 280 characters; a deleted one holds nothing.
    CONSTRAINT comments_body_shape CHECK (
        (deleted_at IS NULL AND char_length(body) BETWEEN 1 AND 280)
        OR (deleted_at IS NOT NULL AND body = '')
    ),
    CONSTRAINT comments_thread_shape CHECK ((parent_id IS NULL) = (reply_to_id IS NULL)),
    CONSTRAINT comments_parent_not_self CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT comments_attempts_valid CHECK (check_attempts >= 0),
    CONSTRAINT comments_pending_due CHECK (moderation_state <> 'pending' OR next_attempt_at IS NOT NULL),
    CONSTRAINT comments_verdict_valid CHECK (verdict IS NULL OR verdict IN ('pass', 'borderline', 'reject'))
);

-- Serves the list of one report's comments, in id order, and the count.
CREATE INDEX idx_comments_report ON comments (report_id, id);

-- Serves the replies of a thread root.
CREATE INDEX idx_comments_parent ON comments (parent_id) WHERE parent_id IS NOT NULL;

-- Serves the worker claim: only comments still waiting for the text check,
-- soonest due first.
CREATE INDEX idx_comments_due ON comments (next_attempt_at) WHERE moderation_state = 'pending';

-- Serves the moderator review list: only comments a moderator has yet to see.
CREATE INDEX idx_comments_review ON comments (id) WHERE needs_review;

-- Serves one account's own comments (viewer_commented, account deletion).
CREATE INDEX idx_comments_account ON comments (account_id);

-- One like per account per comment, by the primary key. The like is a personal
-- row and never read by any trust query.
CREATE TABLE comment_likes (
    comment_id  BIGINT NOT NULL REFERENCES comments(id),
    account_id  BIGINT NOT NULL REFERENCES accounts(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, account_id)
);

-- Serves the likes an account gave, for its deletion.
CREATE INDEX idx_comment_likes_account ON comment_likes (account_id);

-- A flag on a comment. Like a report flag it hides nothing by itself. The
-- reasons rule is the report_flags_reasons_valid body and reuses its helper
-- function report_flag_reasons_distinct, so no second function exists.
CREATE TABLE comment_flags (
    id            BIGSERIAL PRIMARY KEY,
    comment_id    BIGINT NOT NULL REFERENCES comments(id),
    account_id    BIGINT NOT NULL REFERENCES accounts(id),
    reasons       TEXT[] NOT NULL,
    note          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dismissed_at  TIMESTAMPTZ,
    CONSTRAINT comment_flags_one_per_account UNIQUE (comment_id, account_id),
    CONSTRAINT comment_flags_reasons_valid CHECK (
        cardinality(reasons) >= 1
        AND array_ndims(reasons) = 1
        AND array_position(reasons, NULL) IS NULL
        AND reasons <@ ARRAY['spam', 'abusive', 'false', 'dangerous', 'other']::TEXT[]
        AND report_flag_reasons_distinct(reasons)
    ),
    CONSTRAINT comment_flags_note_length CHECK (note IS NULL OR char_length(note) BETWEEN 1 AND 300)
);

-- Serves the open flags of one comment and the moderator's flagged list.
CREATE INDEX idx_comment_flags_open ON comment_flags (comment_id) WHERE dismissed_at IS NULL;

-- Serves the flags an account sent, for its deletion.
CREATE INDEX idx_comment_flags_account ON comment_flags (account_id);

-- An audit trail of Keep and Remove for comments, holding no comment text.
CREATE TABLE comment_moderation_actions (
    id                    BIGSERIAL PRIMARY KEY,
    comment_id            BIGINT NOT NULL REFERENCES comments(id),
    moderator_account_id  BIGINT NOT NULL REFERENCES accounts(id),
    action                TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT comment_moderation_actions_action_valid CHECK (action IN ('keep', 'remove'))
);

-- +goose Down
-- Every comment, like, comment flag and comment moderation row is lost with
-- the tables. report_flag_reasons_distinct belongs to migration 00010 and stays.
DROP TABLE comment_moderation_actions;
DROP TABLE comment_flags;
DROP TABLE comment_likes;
DROP TABLE comments;
