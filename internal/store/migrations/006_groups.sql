-- Grupper: et navngitt sett personer (team, kor, par). Ingen egne regler: en gruppe er bare en kandidat eller en rute
-- som gjør alle medlemmene opptatt. «Kan foreslås» (standard ja) styrer om planleggeren selv kan velge gruppen.
-- grp i stedet for group fordi group er et ord i SQL. Sammensatte fremmednøkler som ellers: ingen kryss mellom organisasjoner.
CREATE TABLE grp (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    proposable INTEGER NOT NULL DEFAULT 1 CHECK (proposable IN (0, 1)),
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name),
    UNIQUE (org_id, id)
);

CREATE TABLE grp_member (
    org_id    INTEGER NOT NULL,
    group_id  INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    PRIMARY KEY (group_id, person_id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id)
);

-- Oppgavene planleggeren kan foreslå gruppen til: plasser gruppen fyller, og hvor gjerne («ikke» er det samme som ingen rad).
-- Sletting av en oppgave som er i bruk her nektes.
CREATE TABLE grp_task (
    org_id   INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    task_id  INTEGER NOT NULL,
    fills    INTEGER NOT NULL CHECK (fills BETWEEN 1 AND 50),
    level    TEXT NOT NULL CHECK (level IN ('darlig', 'noytral', 'godt')),
    PRIMARY KEY (group_id, task_id),
    FOREIGN KEY (org_id, group_id) REFERENCES grp (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id)
);
