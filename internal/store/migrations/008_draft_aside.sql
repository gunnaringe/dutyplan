-- «Remove unlocked» (2026-11-22, the user: "Kanskje en fjern alle ulåste også?"): the plan shows no draft, only what is
-- set by hand and locked, until a new run starts or a draft is pinned. The runs stay (under the earlier drafts).
ALTER TABLE plan ADD COLUMN draft_aside INTEGER NOT NULL DEFAULT 0 CHECK (draft_aside IN (0, 1));
