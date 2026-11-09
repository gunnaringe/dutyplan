-- Level codes are English now. The four tables whose CHECK constraints list the old codes are rebuilt (SQLite cannot change
-- a CHECK in place); they are leaves, nothing references them. Plan weights are rewritten key by key. Runs made by an older
-- version hold the old codes in their input, snapshot and output, so they are marked failed: run again to get a plan.
--   task and event levels: ikke -> never, darlig -> poor, noytral -> neutral, godt -> good
--   combination levels:    kan_ikke -> cannot, upraktisk -> impractical, praktisk -> practical, bor -> preferred

CREATE TABLE person_task_new (
    org_id    INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    task_id   INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('never', 'poor', 'neutral', 'good')),
    PRIMARY KEY (person_id, task_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id) ON DELETE CASCADE
);
INSERT INTO person_task_new (org_id, person_id, task_id, level)
    SELECT org_id, person_id, task_id,
           CASE level WHEN 'ikke' THEN 'never' WHEN 'darlig' THEN 'poor' WHEN 'noytral' THEN 'neutral' WHEN 'godt' THEN 'good' END
    FROM person_task;
DROP TABLE person_task;
ALTER TABLE person_task_new RENAME TO person_task;

CREATE TABLE person_event_new (
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    event_id  INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('never', 'poor', 'good')),
    PRIMARY KEY (person_id, event_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE
);
INSERT INTO person_event_new (org_id, plan_id, person_id, event_id, level)
    SELECT org_id, plan_id, person_id, event_id,
           CASE level WHEN 'ikke' THEN 'never' WHEN 'darlig' THEN 'poor' WHEN 'godt' THEN 'good' END
    FROM person_event;
DROP TABLE person_event;
ALTER TABLE person_event_new RENAME TO person_event;

CREATE TABLE task_combination_new (
    org_id INTEGER NOT NULL,
    task_a INTEGER NOT NULL,
    task_b INTEGER NOT NULL,
    level  TEXT NOT NULL CHECK (level IN ('cannot', 'impractical', 'practical', 'preferred')),
    PRIMARY KEY (task_a, task_b),
    CHECK (task_a < task_b),
    FOREIGN KEY (org_id, task_a) REFERENCES task (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_b) REFERENCES task (org_id, id) ON DELETE CASCADE
);
INSERT INTO task_combination_new (org_id, task_a, task_b, level)
    SELECT org_id, task_a, task_b,
           CASE level WHEN 'kan_ikke' THEN 'cannot' WHEN 'upraktisk' THEN 'impractical' WHEN 'praktisk' THEN 'practical' WHEN 'bor' THEN 'preferred' END
    FROM task_combination;
DROP TABLE task_combination;
ALTER TABLE task_combination_new RENAME TO task_combination;

CREATE TABLE grp_task_new (
    org_id   INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    task_id  INTEGER NOT NULL,
    fills    INTEGER NOT NULL CHECK (fills BETWEEN 1 AND 50),
    level    TEXT NOT NULL CHECK (level IN ('poor', 'neutral', 'good')),
    PRIMARY KEY (group_id, task_id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id)
);
INSERT INTO grp_task_new (org_id, group_id, task_id, fills, level)
    SELECT org_id, group_id, task_id, fills,
           CASE level WHEN 'darlig' THEN 'poor' WHEN 'noytral' THEN 'neutral' WHEN 'godt' THEN 'good' END
    FROM grp_task;
DROP TABLE grp_task;
ALTER TABLE grp_task_new RENAME TO grp_task;

-- Plan weights: the keys first (they contain a dot), then the value of default_combination.
UPDATE plan SET weights =
    replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(weights,
        'w_task.godt', 'w_task.good'), 'w_task.darlig', 'w_task.poor'),
        'w_date.godt', 'w_date.good'), 'w_date.darlig', 'w_date.poor'),
        'w_combo.bor', 'w_combo.preferred'), 'w_combo.praktisk', 'w_combo.practical'), 'w_combo.upraktisk', 'w_combo.impractical'),
        '"kan_ikke"', '"cannot"'), '"upraktisk"', '"impractical"'), '"praktisk"', '"practical"'), '"bor"', '"preferred"')
WHERE weights <> '{}';

UPDATE plan_run SET status = 'failed', error = 'made by an older version; run again', output_json = NULL WHERE status = 'done';
