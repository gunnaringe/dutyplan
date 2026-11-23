-- foreign_keys: off
-- Everything a plan is made of is per plan (2026-11-23, the user: "Task preferences for a name in a plan should be for that
-- plan only", "Task is also something that lives in the plan", "keep people per plan as well"): people, tasks, their
-- sections, combinations and the tasks people take. The organisation keeps its members, links and plans.
--
-- Each organisation's first plan keeps the rows (and ids) it had. Every other plan gets its own copy with new ids, and its
-- answers, notes, places, cells, selections, combinations and the tasks people take are moved to the copies. The ids in its
-- runs are rewritten by the Go step (remapRuns012), from map_person and map_task. An organisation without plans keeps its
-- rows with no plan (plan_id NULL); its first plan takes them (store.CreatePlan).
CREATE TEMP TABLE first_plan AS SELECT org_id, MIN(id) AS plan_id FROM plan GROUP BY org_id;
CREATE TEMP TABLE other_plan AS
    SELECT p.org_id, p.id AS plan_id FROM plan p JOIN first_plan f ON f.org_id = p.org_id WHERE p.id != f.plan_id;

CREATE TEMP TABLE map_person_section AS
    SELECT s.id AS old, o.plan_id AS plan, (SELECT COALESCE(MAX(id), 0) FROM person_section) + ROW_NUMBER() OVER (ORDER BY o.plan_id, s.id) AS new
    FROM person_section s JOIN other_plan o ON o.org_id = s.org_id;
CREATE TEMP TABLE map_person AS
    SELECT x.id AS old, o.plan_id AS plan, (SELECT COALESCE(MAX(id), 0) FROM person) + ROW_NUMBER() OVER (ORDER BY o.plan_id, x.id) AS new
    FROM person x JOIN other_plan o ON o.org_id = x.org_id;
CREATE TEMP TABLE map_task_section AS
    SELECT s.id AS old, o.plan_id AS plan, (SELECT COALESCE(MAX(id), 0) FROM task_section) + ROW_NUMBER() OVER (ORDER BY o.plan_id, s.id) AS new
    FROM task_section s JOIN other_plan o ON o.org_id = s.org_id;
CREATE TEMP TABLE map_task AS
    SELECT x.id AS old, o.plan_id AS plan, (SELECT COALESCE(MAX(id), 0) FROM task) + ROW_NUMBER() OVER (ORDER BY o.plan_id, x.id) AS new
    FROM task x JOIN other_plan o ON o.org_id = x.org_id;

-- Sections of people: a heading in Availability, only placement.
CREATE TABLE person_section_new (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    plan_id    INTEGER,           -- NULL only in an organisation that has no plan yet
    name       TEXT NOT NULL,
    position   INTEGER NOT NULL,  -- the order of the headings (0, 1, ...); people without a section come first
    created_at TEXT NOT NULL,
    UNIQUE (plan_id, name),
    UNIQUE (org_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id)
);
INSERT INTO person_section_new
    SELECT s.id, s.org_id, f.plan_id, s.name, s.position, s.created_at FROM person_section s LEFT JOIN first_plan f ON f.org_id = s.org_id;
INSERT INTO person_section_new
    SELECT m.new, s.org_id, m.plan, s.name, s.position, s.created_at FROM map_person_section m JOIN person_section s ON s.id = m.old;
DROP TABLE person_section;
ALTER TABLE person_section_new RENAME TO person_section;

-- People are rows in a plan, not accounts. proposable: the planner may propose them.
CREATE TABLE person_new (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    plan_id    INTEGER,           -- NULL only in an organisation that has no plan yet
    name       TEXT NOT NULL,
    proposable INTEGER NOT NULL DEFAULT 1 CHECK (proposable IN (0, 1)),
    section_id INTEGER,           -- NULL: no section
    position   INTEGER NOT NULL,  -- the order within the section (0, 1, ...)
    created_at TEXT NOT NULL,
    UNIQUE (plan_id, name),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, section_id) REFERENCES person_section (org_id, id)
);
INSERT INTO person_new
    SELECT x.id, x.org_id, f.plan_id, x.name, x.proposable, x.section_id, x.position, x.created_at
    FROM person x LEFT JOIN first_plan f ON f.org_id = x.org_id;
INSERT INTO person_new
    SELECT m.new, x.org_id, m.plan, x.name, x.proposable,
           (SELECT s.new FROM map_person_section s WHERE s.old = x.section_id AND s.plan = m.plan), x.position, x.created_at
    FROM map_person m JOIN person x ON x.id = m.old;
DROP TABLE person;
ALTER TABLE person_new RENAME TO person;
CREATE UNIQUE INDEX person_org_id ON person (org_id, id);
CREATE INDEX person_plan ON person (plan_id);

-- Sections of tasks: headings in the Tasks part of the table (and the draft), only placement.
CREATE TABLE task_section_new (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    plan_id    INTEGER,           -- NULL only in an organisation that has no plan yet
    name       TEXT NOT NULL,
    position   INTEGER NOT NULL,  -- the order of the headings (0, 1, ...); tasks without a section come first
    created_at TEXT NOT NULL,
    UNIQUE (plan_id, name),
    UNIQUE (org_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id)
);
INSERT INTO task_section_new
    SELECT s.id, s.org_id, f.plan_id, s.name, s.position, s.created_at FROM task_section s LEFT JOIN first_plan f ON f.org_id = s.org_id;
INSERT INTO task_section_new
    SELECT m.new, s.org_id, m.plan, s.name, s.position, s.created_at FROM map_task_section m JOIN task_section s ON s.id = m.old;
DROP TABLE task_section;
ALTER TABLE task_section_new RENAME TO task_section;

CREATE TABLE task_new (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    plan_id    INTEGER,           -- NULL only in an organisation that has no plan yet
    name       TEXT NOT NULL,
    planned    INTEGER NOT NULL DEFAULT 1 CHECK (planned IN (0, 1)),  -- 0: only for manual cells and combinations
    position   INTEGER NOT NULL,  -- the order within its section (0, 1, ... from the top)
    created_at TEXT NOT NULL,
    section_id INTEGER,           -- NULL: no section
    UNIQUE (plan_id, name),
    UNIQUE (org_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, section_id) REFERENCES task_section (org_id, id)
);
INSERT INTO task_new
    SELECT x.id, x.org_id, f.plan_id, x.name, x.planned, x.position, x.created_at, x.section_id
    FROM task x LEFT JOIN first_plan f ON f.org_id = x.org_id;
INSERT INTO task_new
    SELECT m.new, x.org_id, m.plan, x.name, x.planned, x.position, x.created_at,
           (SELECT s.new FROM map_task_section s WHERE s.old = x.section_id AND s.plan = m.plan)
    FROM map_task m JOIN task x ON x.id = m.old;
DROP TABLE task;
ALTER TABLE task_new RENAME TO task;
CREATE INDEX task_plan ON task (plan_id);

-- Two tasks on the same event: cannot / practical (no row: impractical, «bad»).
CREATE TABLE task_combination_new (
    org_id  INTEGER NOT NULL,
    plan_id INTEGER,              -- NULL only in an organisation that has no plan yet
    task_a  INTEGER NOT NULL,
    task_b  INTEGER NOT NULL,
    level   TEXT NOT NULL CHECK (level IN ('cannot', 'practical')),
    PRIMARY KEY (task_a, task_b),
    CHECK (task_a < task_b),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, task_a) REFERENCES task (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_b) REFERENCES task (org_id, id) ON DELETE CASCADE
);
INSERT INTO task_combination_new
    SELECT c.org_id, f.plan_id, c.task_a, c.task_b, c.level FROM task_combination c LEFT JOIN first_plan f ON f.org_id = c.org_id;
INSERT INTO task_combination_new
    SELECT c.org_id, a.plan, a.new, b.new, c.level
    FROM task_combination c JOIN map_task a ON a.old = c.task_a JOIN map_task b ON b.old = c.task_b AND b.plan = a.plan;
DROP TABLE task_combination;
ALTER TABLE task_combination_new RENAME TO task_combination;
CREATE INDEX task_combination_plan ON task_combination (plan_id);

-- The tasks a person takes: poor (if needed) or good; no row = can't. A person is in one plan, so the person and the task
-- are the key; plan_id is theirs.
CREATE TABLE person_task_new (
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER,            -- NULL only in an organisation that has no plan yet
    person_id INTEGER NOT NULL,
    task_id   INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('poor', 'good')),
    PRIMARY KEY (person_id, task_id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id) ON DELETE CASCADE
);
INSERT INTO person_task_new
    SELECT pt.org_id, f.plan_id, pt.person_id, pt.task_id, pt.level FROM person_task pt LEFT JOIN first_plan f ON f.org_id = pt.org_id;
INSERT INTO person_task_new
    SELECT pt.org_id, p.plan, p.new, t.new, pt.level
    FROM person_task pt JOIN map_person p ON p.old = pt.person_id JOIN map_task t ON t.old = pt.task_id AND t.plan = p.plan;
DROP TABLE person_task;
ALTER TABLE person_task_new RENAME TO person_task;
CREATE INDEX person_task_plan ON person_task (plan_id);
CREATE INDEX person_task_task ON person_task (task_id);

-- What already was per plan now points at the plan's own copies.
UPDATE person_event SET person_id = (SELECT m.new FROM map_person m WHERE m.old = person_event.person_id AND m.plan = person_event.plan_id)
    WHERE plan_id IN (SELECT plan_id FROM other_plan);
UPDATE answer_note SET person_id = (SELECT m.new FROM map_person m WHERE m.old = answer_note.person_id AND m.plan = answer_note.plan_id)
    WHERE plan_id IN (SELECT plan_id FROM other_plan);
UPDATE manual_cell SET
    task_id = (SELECT m.new FROM map_task m WHERE m.old = manual_cell.task_id AND m.plan = manual_cell.plan_id),
    person_id = (SELECT m.new FROM map_person m WHERE m.old = manual_cell.person_id AND m.plan = manual_cell.plan_id)
    WHERE plan_id IN (SELECT plan_id FROM other_plan);
UPDATE pool_member SET
    task_id = (SELECT m.new FROM map_task m WHERE m.old = pool_member.task_id AND m.plan = pool_member.plan_id),
    person_id = (SELECT m.new FROM map_person m WHERE m.old = pool_member.person_id AND m.plan = pool_member.plan_id)
    WHERE plan_id IN (SELECT plan_id FROM other_plan);
UPDATE event_task SET task_id = (SELECT m.new FROM map_task m WHERE m.old = event_task.task_id AND m.plan = event_task.plan_id)
    WHERE plan_id IN (SELECT plan_id FROM other_plan);

DROP TABLE first_plan;
DROP TABLE other_plan;
DROP TABLE map_person_section;
DROP TABLE map_task_section;
-- map_person and map_task are dropped by the Go step, after it has rewritten the runs.
