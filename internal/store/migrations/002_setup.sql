-- Setup: tasks, day types and combinations belong to the organisation; dates belong to a plan.
-- Every child table has org_id and composite foreign keys to (org_id, id): the database itself stops a row
-- from pointing at something in another organisation, in addition to the checks in the code.
CREATE UNIQUE INDEX plan_org_id ON plan (org_id, id);

CREATE TABLE task (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    planned    INTEGER NOT NULL DEFAULT 1 CHECK (planned IN (0, 1)),  -- 0: only for manual cells and combinations
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name),
    UNIQUE (org_id, id)
);

CREATE TABLE day_type (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name),
    UNIQUE (org_id, id)
);

-- Which tasks a day type has, and how many places. Deleting a task that is in use here is refused.
CREATE TABLE day_type_task (
    org_id      INTEGER NOT NULL,
    day_type_id INTEGER NOT NULL,
    task_id     INTEGER NOT NULL,
    slots       INTEGER NOT NULL CHECK (slots BETWEEN 1 AND 50),
    PRIMARY KEY (day_type_id, task_id),
    FOREIGN KEY (org_id, day_type_id) REFERENCES day_type (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id)
);

-- The shared setting: what can be combined on the same day. A pair is stored once (task_a < task_b); «standard» is no row.
CREATE TABLE task_combination (
    org_id INTEGER NOT NULL,
    task_a INTEGER NOT NULL,
    task_b INTEGER NOT NULL,
    level  TEXT NOT NULL CHECK (level IN ('kan_ikke', 'upraktisk', 'praktisk', 'bor')),
    PRIMARY KEY (task_a, task_b),
    CHECK (task_a < task_b),
    FOREIGN KEY (org_id, task_a) REFERENCES task (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_b) REFERENCES task (org_id, id) ON DELETE CASCADE
);

-- A date in a plan, with a day type. UNIQUE (plan_id, date) is a deliberate limit for now (two events on the same day are not
-- supported); the API uses the id, so it can be loosened later without changing the interface.
CREATE TABLE slot_date (
    id          INTEGER PRIMARY KEY,
    org_id      INTEGER NOT NULL,
    plan_id     INTEGER NOT NULL,
    date        TEXT NOT NULL CHECK (date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    day_type_id INTEGER NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    UNIQUE (plan_id, date),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, day_type_id) REFERENCES day_type (org_id, id)
);
CREATE INDEX slot_date_plan ON slot_date (plan_id, date);
