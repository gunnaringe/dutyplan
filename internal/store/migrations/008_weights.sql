-- The weighting the planner uses, per plan: only what differs from the default, as JSON ({"w_fair": 12, "w_task.godt": 3}).
-- Empty ({}) means the default everywhere. The keys and limits are validated in the code (store.WeightSpecs).
ALTER TABLE plan ADD COLUMN weights TEXT NOT NULL DEFAULT '{}';
