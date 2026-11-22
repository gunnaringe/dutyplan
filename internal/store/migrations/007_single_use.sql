-- An invitation may be for one person (2026-11-22, the user: "make multi-use vs single use a setting for the link"):
-- single_use = 1 ends it when someone becomes a member with it. Only for invitations: a view or edit link stays in the
-- browser that opened it, so using it once would mean nothing.
ALTER TABLE share_link ADD COLUMN single_use INTEGER NOT NULL DEFAULT 0 CHECK (single_use IN (0, 1) AND (single_use = 0 OR level = 'join'));
