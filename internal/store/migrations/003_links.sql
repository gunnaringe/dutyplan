-- Links that share an organisation or one plan of it: whoever opens one gets in without an account. level: view (read only),
-- edit (change everything in the scope, but not members, links or deleting), or join (organisation only: whoever opens it
-- while signed in becomes an editor). The token is kept as it is, so the owner can copy the link again; deleting the row
-- ends the link at once. created_by may be gone (the person was removed).
CREATE TABLE share_link (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    plan_id    INTEGER,
    token      TEXT NOT NULL UNIQUE,
    level      TEXT NOT NULL CHECK (level IN ('view', 'edit', 'join')),
    label      TEXT NOT NULL DEFAULT '',
    created_by INTEGER REFERENCES app_user(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT, -- NULL: never
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    CHECK (level != 'join' OR plan_id IS NULL)
);
CREATE INDEX share_link_org ON share_link (org_id);
