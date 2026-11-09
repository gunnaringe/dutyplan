-- A person's own answers. person_task is at organisation level (which tasks I take part in, and how willingly);
-- person_event is per plan (can I make this event). No row = not taking part in the task / neutral on the event.
-- Composite foreign keys stop an answer from pointing at a person, task or event in another organisation,
-- and (org_id, plan_id, id) makes sure the event really belongs to the plan.
CREATE UNIQUE INDEX person_org_id ON person (org_id, id);
CREATE UNIQUE INDEX event_org_plan_id ON event (org_id, plan_id, id);

CREATE TABLE person_task (
    org_id    INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    task_id   INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('ikke', 'darlig', 'noytral', 'godt')),
    PRIMARY KEY (person_id, task_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id) ON DELETE CASCADE
);

CREATE TABLE person_event (
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    event_id  INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('ikke', 'darlig', 'godt')),
    PRIMARY KEY (person_id, event_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE
);
