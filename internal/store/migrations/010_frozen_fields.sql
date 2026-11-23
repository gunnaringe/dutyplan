-- A details row can be frozen (2026-11-23, the user: "Let it be possible to mark some of the about fields so they work similar to
-- having a frozen Row in excel."): it stays in sight under the dates while the table scrolls. Shared by everyone, as in a spreadsheet.
ALTER TABLE plan_field ADD COLUMN frozen INTEGER NOT NULL DEFAULT 0 CHECK (frozen IN (0, 1));
