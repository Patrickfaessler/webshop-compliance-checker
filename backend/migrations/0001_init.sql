-- 0001: scans and their results. Spec: specs/07-architecture.md (data model), ADR-1 (no user data), ADR-5.
-- Not yet here: pages, suggestions, rate_limits (added with their features).

CREATE TABLE scans (
    id               text PRIMARY KEY,               -- random 128-bit id (F11)
    host             text        NOT NULL,
    start_url        text        NOT NULL,
    areas            text[]      NOT NULL,
    status           text        NOT NULL CHECK (status IN ('queued', 'running', 'done', 'incomplete', 'failed')),
    pages_done       integer     NOT NULL DEFAULT 0,
    pages_total      integer     NOT NULL DEFAULT 0,
    score            integer     CHECK (score BETWEEN 0 AND 100),
    error_de         text        NOT NULL DEFAULT '',
    previous_scan_id text        NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL,
    finished_at      timestamptz,
    expires_at       timestamptz NOT NULL
);
CREATE INDEX scans_host_created_idx ON scans (host, created_at DESC);
CREATE INDEX scans_expires_idx ON scans (expires_at);

CREATE TABLE rule_results (
    scan_id        text    NOT NULL REFERENCES scans (id) ON DELETE CASCADE,
    position       integer NOT NULL,
    rule_id        text    NOT NULL,
    state          text    NOT NULL CHECK (state IN ('passed', 'failed', 'not_applicable', 'not_assessable')),
    decided_by     text    NOT NULL DEFAULT '',
    pages_affected integer NOT NULL DEFAULT 0,
    PRIMARY KEY (scan_id, rule_id)
);

CREATE TABLE findings (
    id         text    PRIMARY KEY,
    scan_id    text    NOT NULL REFERENCES scans (id) ON DELETE CASCADE,
    position   integer NOT NULL,
    rule_id    text    NOT NULL,
    severity   text    NOT NULL CHECK (severity IN ('critical', 'warning', 'notice')),
    url        text    NOT NULL DEFAULT '',
    selector   text    NOT NULL DEFAULT '',
    snippet    text    NOT NULL DEFAULT '',
    evidence   text    NOT NULL DEFAULT '',
    sub        text    NOT NULL DEFAULT '',
    summary_de text    NOT NULL DEFAULT ''
);
CREATE INDEX findings_scan_idx ON findings (scan_id, position);
