-- A note on one answer: a short comment from a person about one event, for example «can come after 12» (2026-11-22, the user:
-- "legge inn notat/kommentar i hver celle for om dagen passer for en person"). Its own table, so a note stays when the answer
-- is cleared. Same keys as person_event: the event must be the plan's, the person the organisation's; both take the note
-- with them when they go. Never part of the planner's input.
CREATE TABLE answer_note (
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    event_id  INTEGER NOT NULL,
    note      TEXT NOT NULL CHECK (note != '' AND length(note) <= 500),
    PRIMARY KEY (person_id, event_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE
);
CREATE INDEX answer_note_plan ON answer_note (org_id, plan_id);
