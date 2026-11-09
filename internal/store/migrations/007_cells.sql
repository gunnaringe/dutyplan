-- Manuelle ruter og utvalg, per plan.
--   manual_cell: admin legger inn en person, en gruppe eller en tekst (f.eks. «Gjest utenfra») for en oppgave på en hendelse.
--                Ruten er låst: planleggeren flytter den aldri, og den teller som en fylt plass (fills for gruppe og tekst).
--   pool_member: utvalg, «bare disse kan velges» for en oppgave på en hendelse. Finnes det rader, er det et utvalg.
-- Sammensatte fremmednøkler: hendelsen hører til planen (org_id, plan_id, id), resten til organisasjonen. Slettes
-- hendelsen, følger rutene og utvalget med. En gruppe, oppgave eller person i bruk her kan ikke slettes.
CREATE TABLE manual_cell (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL,
    plan_id    INTEGER NOT NULL,
    event_id   INTEGER NOT NULL,
    task_id    INTEGER NOT NULL,
    person_id  INTEGER,
    group_id   INTEGER,
    label      TEXT,
    fills      INTEGER NOT NULL DEFAULT 1 CHECK (fills BETWEEN 1 AND 50),
    created_at TEXT NOT NULL,
    CHECK ((person_id IS NOT NULL) + (group_id IS NOT NULL) + (label IS NOT NULL) = 1),
    CHECK (person_id IS NULL OR fills = 1),
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id)
);
CREATE UNIQUE INDEX manual_cell_person ON manual_cell (event_id, task_id, person_id) WHERE person_id IS NOT NULL;
CREATE UNIQUE INDEX manual_cell_group ON manual_cell (event_id, task_id, group_id) WHERE group_id IS NOT NULL;
CREATE UNIQUE INDEX manual_cell_label ON manual_cell (event_id, task_id, label) WHERE label IS NOT NULL;
CREATE INDEX manual_cell_plan ON manual_cell (plan_id, event_id, task_id);

CREATE TABLE pool_member (
    id        INTEGER PRIMARY KEY,
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER NOT NULL,
    event_id  INTEGER NOT NULL,
    task_id   INTEGER NOT NULL,
    person_id INTEGER,
    group_id  INTEGER,
    CHECK ((person_id IS NOT NULL) + (group_id IS NOT NULL) = 1),
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id)
);
CREATE UNIQUE INDEX pool_member_person ON pool_member (event_id, task_id, person_id) WHERE person_id IS NOT NULL;
CREATE UNIQUE INDEX pool_member_group ON pool_member (event_id, task_id, group_id) WHERE group_id IS NOT NULL;
CREATE INDEX pool_member_plan ON pool_member (plan_id, event_id, task_id);
