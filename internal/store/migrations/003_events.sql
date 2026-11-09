-- An event is the self-contained unit in a plan, not the date. Two events can fall on the same day, and everything
-- about «can I», combinations and busyness applies to the event. The table is replaced because SQLite cannot remove
-- UNIQUE (plan_id, date) from an existing table. Nothing refers to slot_date yet.
CREATE TABLE event (
    id          INTEGER PRIMARY KEY,
    org_id      INTEGER NOT NULL,
    plan_id     INTEGER NOT NULL,
    date        TEXT NOT NULL CHECK (date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    day_type_id INTEGER NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    UNIQUE (org_id, id),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, day_type_id) REFERENCES day_type (org_id, id)
);
INSERT INTO event (id, org_id, plan_id, date, day_type_id, note, created_at)
    SELECT id, org_id, plan_id, date, day_type_id, note, created_at FROM slot_date;
DROP TABLE slot_date;
CREATE INDEX event_plan ON event (plan_id, date, id);
