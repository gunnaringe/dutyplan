-- Vektingen planleggeren bruker, per plan: bare det som avviker fra standard, som JSON ({"w_fair": 12, "w_task.godt": 3}).
-- Tom ({}) betyr standard overalt. Nøklene og grensene valideres i koden (store.WeightSpecs).
ALTER TABLE plan ADD COLUMN weights TEXT NOT NULL DEFAULT '{}';
