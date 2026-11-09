-- Manual cells and selections, per plan.
--   manual_cell: the admin enters a person, a group or a text (for example «Outside guest») for a task on an event.
--                The cell is locked: the planner never moves it, and it counts as a filled place (fills for a group and a text).
--   pool_member: a selection, «only these can be picked» for a task on an event. If there are rows, there is a selection.
-- Composite foreign keys: the event belongs to the plan (org_id, plan_id, id), the rest to the organisation. If
-- the event is deleted, the cells and the selection go with it. A group, task or person in use here cannot be deleted.
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
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE,
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
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id)
);
CREATE UNIQUE INDEX pool_member_person ON pool_member (event_id, task_id, person_id) WHERE person_id IS NOT NULL;
CREATE UNIQUE INDEX pool_member_group ON pool_member (event_id, task_id, group_id) WHERE group_id IS NOT NULL;
CREATE INDEX pool_member_plan ON pool_member (plan_id, event_id, task_id);
