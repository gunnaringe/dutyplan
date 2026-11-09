-- A plan can have one finished run pinned as its current plan: that is the one people see under «My assignments». With nothing
-- pinned they see the newest finished run. The partial unique index allows one pin per plan; pinned runs are never pruned.
ALTER TABLE plan_run ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0 CHECK (pinned IN (0, 1));
CREATE UNIQUE INDEX plan_run_pinned ON plan_run (plan_id) WHERE pinned = 1;
