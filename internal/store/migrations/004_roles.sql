-- Simpler roles (2026-11-22, the user: "the roles are org admin and org members + edit and view shared link"). A member is
-- an admin (members, invitations, renaming and deleting the organisation) or a member (plans and everything in them,
-- people, tasks, sections, and links to plans). A view or edit link is to one plan; only an invitation (join) is to the
-- organisation. Links to a whole organisation are deleted, not made into anything else.
CREATE TABLE membership_new (
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    user_id    INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'member')),
    created_at TEXT NOT NULL,
    PRIMARY KEY (org_id, user_id)
);
INSERT INTO membership_new SELECT org_id, user_id, CASE role WHEN 'owner' THEN 'admin' ELSE 'member' END, created_at FROM membership;
DROP TABLE membership;
ALTER TABLE membership_new RENAME TO membership;
CREATE INDEX membership_user ON membership (user_id);

DELETE FROM share_link WHERE level != 'join' AND plan_id IS NULL;
CREATE TABLE share_link_new (
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
    CHECK ((level = 'join') = (plan_id IS NULL))
);
INSERT INTO share_link_new SELECT id, org_id, plan_id, token, level, label, created_by, created_at, expires_at FROM share_link;
DROP TABLE share_link;
ALTER TABLE share_link_new RENAME TO share_link;
CREATE INDEX share_link_org ON share_link (org_id);
