-- The organisation is the top level. Everything else belongs to an organisation; so do plans (for example «Autumn 2026»).
CREATE TABLE organization (
    id         INTEGER PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- A plan is a period in an organisation. Dates, availability and assignments come under the plan in later phases.
CREATE TABLE plan (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    slug       TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (org_id, slug)
);

-- People belong to the organisation and are reused across plans. proposable: the planner may propose them.
CREATE TABLE person (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    active     INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    proposable INTEGER NOT NULL DEFAULT 1 CHECK (proposable IN (0, 1)),
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name)
);

-- Every change is logged in the same transaction as the change. actor is self-declared (there is no login).
CREATE TABLE change_log (
    id      INTEGER PRIMARY KEY,
    org_id  INTEGER NOT NULL REFERENCES organization(id),
    plan_id INTEGER REFERENCES plan(id),
    actor   TEXT NOT NULL,
    action  TEXT NOT NULL,
    detail  TEXT NOT NULL,
    at      TEXT NOT NULL
);
CREATE INDEX change_log_org ON change_log (org_id, id DESC);
