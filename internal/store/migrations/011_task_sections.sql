-- Sections for tasks (2026-11-23, the user: "Make it possible to have some sections for the tasks as well. As for people,
-- sections is just a UI concept for now."): headings in the Tasks part of the table (and the draft) with their tasks below,
-- the same in every plan. Only placement: the planner never sees them. A task's position is now its order within its section
-- (tasks without a section first), as for people.
CREATE TABLE task_section (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    position   INTEGER NOT NULL,  -- the order of the headings (0, 1, ...); tasks without a section come first
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name),
    UNIQUE (org_id, id)
);
ALTER TABLE task ADD COLUMN section_id INTEGER REFERENCES task_section (id);  -- NULL: no section (the organisation is checked by the code)
