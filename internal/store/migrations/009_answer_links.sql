-- A link that can answer (2026-11-22, the user: "Vi trenger også en visning for én person … en visning der de kan fylle ut når de selv
-- kan. Foreløpig uten noen bruker-management, så samme lenke."): level answer sees the plan and sets the answers and notes of anyone
-- on the list, nothing else. SQLite cannot change a CHECK, so the table is made again with the new level; nothing refers to it.
CREATE TABLE share_link_new (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    plan_id    INTEGER,
    token      TEXT NOT NULL UNIQUE,
    level      TEXT NOT NULL CHECK (level IN ('view', 'answer', 'edit', 'join')),
    label      TEXT NOT NULL DEFAULT '',
    created_by INTEGER REFERENCES app_user(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT, -- NULL: never
    single_use INTEGER NOT NULL DEFAULT 0 CHECK (single_use IN (0, 1) AND (single_use = 0 OR level = 'join')),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    CHECK (level != 'join' OR plan_id IS NULL)
);
INSERT INTO share_link_new (id, org_id, plan_id, token, level, label, created_by, created_at, expires_at, single_use)
SELECT id, org_id, plan_id, token, level, label, created_by, created_at, expires_at, single_use FROM share_link;
DROP TABLE share_link;
ALTER TABLE share_link_new RENAME TO share_link;
CREATE INDEX share_link_org ON share_link (org_id);
