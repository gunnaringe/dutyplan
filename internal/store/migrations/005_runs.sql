-- En planleggingskjøring: hva som ble sendt til planleggeren, hva den svarte, og et øyeblikksbilde av navnene så
-- resultatet kan vises riktig også etter at noe er endret. input_hash er hash av dataene (ikke frø og tidsgrense):
-- endres dataene etterpå, er kjøringen «utdatert». Sammensatt fremmednøkkel hindrer en kjøring på feil organisasjon.
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
