-- The whole schema. Everything belongs to an organisation, and every child table has org_id with composite foreign keys
-- to (org_id, id): the database itself stops a row from pointing at something in another organisation, in addition to
-- the checks in the code. Wire codes are English: levels never|poor|neutral|good, combinations
-- cannot|impractical|practical|preferred.

-- The organisation is the top level; it has plans (for example «Autumn 2026»).
CREATE TABLE organization (
    id         INTEGER PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- A plan is a period in an organisation, with its events. weights: the planner's weighting, only what differs from the
-- default, as JSON ({"w_fair": 12, "w_task.good": 3}); the keys and limits are validated in the code (store.WeightSpecs).
CREATE TABLE plan (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    slug       TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    weights    TEXT NOT NULL DEFAULT '{}',
    UNIQUE (org_id, slug)
);
CREATE UNIQUE INDEX plan_org_id ON plan (org_id, id);

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
CREATE UNIQUE INDEX person_org_id ON person (org_id, id);

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

-- The shared setting: what can be combined on the same event. A pair is stored once (task_a < task_b); «standard» is no row.
CREATE TABLE task_combination (
    org_id INTEGER NOT NULL,
    task_a INTEGER NOT NULL,
    task_b INTEGER NOT NULL,
    level  TEXT NOT NULL CHECK (level IN ('cannot', 'impractical', 'practical', 'preferred')),
    PRIMARY KEY (task_a, task_b),
    CHECK (task_a < task_b),
    FOREIGN KEY (org_id, task_a) REFERENCES task (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_b) REFERENCES task (org_id, id) ON DELETE CASCADE
);

-- An event is the self-contained unit in a plan, not the date: two events can fall on the same day, and everything about
-- «can I», combinations and busyness applies to the event. start_time and end_time are 'HH:MM' on a 24-hour clock (the
-- local time of the event, no time zone) or '' when not set; an end time needs a start time and comes after it on the same
-- day. Both are validated in the code, and the checks here are a second line of defence. description is free text.
CREATE TABLE event (
    id          INTEGER PRIMARY KEY,
    org_id      INTEGER NOT NULL,
    plan_id     INTEGER NOT NULL,
    date        TEXT NOT NULL CHECK (date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    day_type_id INTEGER NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    start_time  TEXT NOT NULL DEFAULT '' CHECK (start_time = '' OR start_time GLOB '[0-2][0-9]:[0-5][0-9]'),
    end_time    TEXT NOT NULL DEFAULT '' CHECK (end_time = '' OR (end_time GLOB '[0-2][0-9]:[0-5][0-9]' AND start_time <> '' AND end_time > start_time)),
    description TEXT NOT NULL DEFAULT '',
    UNIQUE (org_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, day_type_id) REFERENCES day_type (org_id, id)
);
CREATE INDEX event_plan ON event (plan_id, date, id);
CREATE UNIQUE INDEX event_org_plan_id ON event (org_id, plan_id, id);

-- A person's own answers. person_task is at organisation level (which tasks I take part in, and how willingly);
-- person_event is per plan (can I make this event). No row = not taking part in the task / not answered on the event.
-- (org_id, plan_id, event_id) makes sure the event really belongs to the plan.
CREATE TABLE person_task (
    org_id    INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    task_id   INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('never', 'poor', 'neutral', 'good')),
    PRIMARY KEY (person_id, task_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id) ON DELETE CASCADE
);

CREATE TABLE person_event (
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    event_id  INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('never', 'poor', 'good')),
    PRIMARY KEY (person_id, event_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE -- answers follow their event
);

-- A planning run: what was sent to the planner, what it answered, and a snapshot of the names so that the result can be
-- shown correctly even after something has changed. input_hash is the hash of the data (not seed and time limit): if the
-- data changes afterwards, the run is «out of date». One finished run per plan can be pinned as its current plan (the
-- partial unique index allows one pin per plan); pinned runs are never pruned.
CREATE TABLE plan_run (
    id            INTEGER PRIMARY KEY,
    org_id        INTEGER NOT NULL,
    plan_id       INTEGER NOT NULL,
    status        TEXT NOT NULL CHECK (status IN ('running', 'done', 'failed')),
    seed          INTEGER NOT NULL,
    time_limit    INTEGER NOT NULL,
    input_hash    TEXT NOT NULL,
    input_json    TEXT NOT NULL,
    snapshot_json TEXT NOT NULL,
    output_json   TEXT,
    error         TEXT,
    started_at    TEXT NOT NULL,
    finished_at   TEXT,
    pinned        INTEGER NOT NULL DEFAULT 0 CHECK (pinned IN (0, 1)),
    UNIQUE (org_id, plan_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id)
);
CREATE INDEX plan_run_plan ON plan_run (plan_id, id DESC);
CREATE UNIQUE INDEX plan_run_pinned ON plan_run (plan_id) WHERE pinned = 1;

-- Groups: a named set of people (team, choir, couple). No rules of their own: a group is just a candidate or a cell
-- that makes all the members busy. «Can be proposed» (default yes) governs whether the planner itself may pick the group.
-- grp instead of group because group is a word in SQL.
CREATE TABLE grp (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    proposable INTEGER NOT NULL DEFAULT 1 CHECK (proposable IN (0, 1)),
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name),
    UNIQUE (org_id, id)
);

CREATE TABLE grp_member (
    org_id    INTEGER NOT NULL,
    group_id  INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    PRIMARY KEY (group_id, person_id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id)
);

-- The tasks the planner may propose the group for: places the group fills, and how willingly («never» is the same as no row).
-- Deleting a task that is in use here is refused.
CREATE TABLE grp_task (
    org_id   INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    task_id  INTEGER NOT NULL,
    fills    INTEGER NOT NULL CHECK (fills BETWEEN 1 AND 50),
    level    TEXT NOT NULL CHECK (level IN ('poor', 'neutral', 'good')),
    PRIMARY KEY (group_id, task_id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id)
);

-- Manual cells and selections, per plan.
--   manual_cell: the admin enters a person, a group or a text (for example «Outside guest») for a task on an event.
--                The planner never moves it, and it counts as a filled place (fills for a group and a text).
--   pool_member: a selection, «only these can be picked» for a task on an event. If there are rows, there is a selection.
-- If the event is deleted, the cells and the selection go with it. A group, task or person in use here cannot be deleted.
CREATE TABLE manual_cell (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL,
    plan_id    INTEGER NOT NULL,
    event_id   INTEGER NOT NULL,
    task_id    INTEGER NOT NULL,
    person_id  INTEGER,
    group_id   INTEGER,
    label      TEXT,
    fills      INTEGER NOT NULL DEFAULT 1 CHECK (fills BETWEEN 1 AND 50),
    created_at TEXT NOT NULL,
    CHECK ((person_id IS NOT NULL) + (group_id IS NOT NULL) + (label IS NOT NULL) = 1),
    CHECK (person_id IS NULL OR fills = 1),
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE, -- cells follow their event
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id)
);
CREATE UNIQUE INDEX manual_cell_person ON manual_cell (event_id, task_id, person_id) WHERE person_id IS NOT NULL;
CREATE UNIQUE INDEX manual_cell_group ON manual_cell (event_id, task_id, group_id) WHERE group_id IS NOT NULL;
CREATE UNIQUE INDEX manual_cell_label ON manual_cell (event_id, task_id, label) WHERE label IS NOT NULL;
CREATE INDEX manual_cell_plan ON manual_cell (plan_id, event_id, task_id);

CREATE TABLE pool_member (
    id        INTEGER PRIMARY KEY,
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER NOT NULL,
    event_id  INTEGER NOT NULL,
    task_id   INTEGER NOT NULL,
    person_id INTEGER,
    group_id  INTEGER,
    CHECK ((person_id IS NOT NULL) + (group_id IS NOT NULL) = 1),
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE, -- selections follow their event
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id)
);
CREATE UNIQUE INDEX pool_member_person ON pool_member (event_id, task_id, person_id) WHERE person_id IS NOT NULL;
CREATE UNIQUE INDEX pool_member_group ON pool_member (event_id, task_id, group_id) WHERE group_id IS NOT NULL;
CREATE INDEX pool_member_plan ON pool_member (plan_id, event_id, task_id);

-- The table on the plan page sets everything about an event in one place.
--   event_task:  places for one task on one event, instead of what the day type says (0 = not on this event). The planner
--                uses them, so they are part of a run's input. A deleted event or task takes its places with it.
--   plan_field:  an info row of a plan (for example «Theme»): a name, in the order they were made.
--   event_field: the value of an info row for one event. The planner never sees it.
CREATE TABLE event_task (
    org_id   INTEGER NOT NULL,
    plan_id  INTEGER NOT NULL,
    event_id INTEGER NOT NULL,
    task_id  INTEGER NOT NULL,
    slots    INTEGER NOT NULL CHECK (slots BETWEEN 0 AND 50),
    PRIMARY KEY (event_id, task_id),
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE, -- places follow their event
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id) ON DELETE CASCADE -- and their task
);
CREATE INDEX event_task_plan ON event_task (plan_id, event_id);

CREATE TABLE plan_field (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL,
    plan_id    INTEGER NOT NULL,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (plan_id, name),
    UNIQUE (org_id, plan_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id)
);

CREATE TABLE event_field (
    org_id   INTEGER NOT NULL,
    plan_id  INTEGER NOT NULL,
    event_id INTEGER NOT NULL,
    field_id INTEGER NOT NULL,
    value    TEXT NOT NULL CHECK (value <> ''),
    PRIMARY KEY (event_id, field_id),
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE, -- values follow their event
    FOREIGN KEY (org_id, plan_id, field_id) REFERENCES plan_field (org_id, plan_id, id) ON DELETE CASCADE -- and their row
);
CREATE INDEX event_field_plan ON event_field (plan_id, field_id);
