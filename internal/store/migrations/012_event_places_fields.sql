-- The table on the plan page sets everything about an event in one place.
--   event_task:  places for one task on one event, instead of what the day type says (0 = not on this event). The planner
--                uses them, so they are part of a run's input. A deleted event or task takes its overrides with it.
--   plan_field:  an info row of a plan (for example «Theme»): a name, in the order they were made.
--   event_field: the value of an info row for one event. Like the note, the planner never sees it.
CREATE TABLE event_task (
    org_id   INTEGER NOT NULL,
    plan_id  INTEGER NOT NULL,
    event_id INTEGER NOT NULL,
    task_id  INTEGER NOT NULL,
    slots    INTEGER NOT NULL CHECK (slots BETWEEN 0 AND 50),
    PRIMARY KEY (event_id, task_id),
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id) ON DELETE CASCADE
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
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, plan_id, field_id) REFERENCES plan_field (org_id, plan_id, id) ON DELETE CASCADE
);
CREATE INDEX event_field_plan ON event_field (plan_id, field_id);
