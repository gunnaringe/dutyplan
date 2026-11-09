-- Groups: a named set of people (team, choir, couple). No rules of their own: a group is just a candidate or a cell
-- that makes all the members busy. «Can be proposed» (default yes) governs whether the planner itself may pick the group.
-- grp instead of group because group is a word in SQL. Composite foreign keys as elsewhere: no crossing between organisations.
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
    level    TEXT NOT NULL CHECK (level IN ('darlig', 'noytral', 'godt')),
    PRIMARY KEY (group_id, task_id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id)
);
