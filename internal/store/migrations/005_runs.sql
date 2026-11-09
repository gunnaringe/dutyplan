-- A planning run: what was sent to the planner, what it answered, and a snapshot of the names so that
-- the result can be shown correctly even after something has changed. input_hash is the hash of the data (not seed and time limit):
-- if the data changes afterwards, the run is «out of date». A composite foreign key stops a run on the wrong organisation.
CREATE TABLE plan_run (
    id            INTEGER PRIMARY KEY,
    org_id        INTEGER NOT NULL,
    plan_id       INTEGER NOT NULL,
    status        TEXT NOT NULL CHECK (status IN ('running', 'done', 'failed')),
    seed          INTEGER NOT NULL,
    time_limit    INTEGER NOT NULL,
    actor         TEXT NOT NULL,
    input_hash    TEXT NOT NULL,
    input_json    TEXT NOT NULL,
    snapshot_json TEXT NOT NULL,
    output_json   TEXT,
    error         TEXT,
    started_at    TEXT NOT NULL,
    finished_at   TEXT,
    UNIQUE (org_id, plan_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id)
);
CREATE INDEX plan_run_plan ON plan_run (plan_id, id DESC);
