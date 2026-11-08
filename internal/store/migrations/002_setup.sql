-- Oppsett: oppgaver, dagtyper og kombinasjoner hører til organisasjonen; datoer hører til en plan.
-- Alle barnetabeller har org_id og sammensatte fremmednøkler mot (org_id, id): databasen selv hindrer at en rad
-- peker på noe i en annen organisasjon, i tillegg til kontrollene i koden.
CREATE UNIQUE INDEX plan_org_id ON plan (org_id, id);

CREATE TABLE task (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    planned    INTEGER NOT NULL DEFAULT 1 CHECK (planned IN (0, 1)),  -- 0: bare for manuelle ruter og kombinasjoner
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name),
    UNIQUE (org_id, id)
);

CREATE TABLE day_type (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name),
    UNIQUE (org_id, id)
);

-- Hvilke oppgaver en dagtype har, og hvor mange plasser. Sletting av en oppgave som er i bruk her nektes.
CREATE TABLE day_type_task (
    org_id      INTEGER NOT NULL,
    day_type_id INTEGER NOT NULL,
    task_id     INTEGER NOT NULL,
    slots       INTEGER NOT NULL CHECK (slots BETWEEN 1 AND 50),
    PRIMARY KEY (day_type_id, task_id),
    FOREIGN KEY (org_id, day_type_id) REFERENCES day_type (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id)
);

-- Felles innstilling: hva som kan kombineres samme dag. Ett par lagres én gang (task_a < task_b); «standard» er ingen rad.
CREATE TABLE task_combination (
    org_id INTEGER NOT NULL,
    task_a INTEGER NOT NULL,
    task_b INTEGER NOT NULL,
    level  TEXT NOT NULL CHECK (level IN ('kan_ikke', 'upraktisk', 'praktisk', 'bor')),
    PRIMARY KEY (task_a, task_b),
    CHECK (task_a < task_b),
    FOREIGN KEY (org_id, task_a) REFERENCES task (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_b) REFERENCES task (org_id, id) ON DELETE CASCADE
);

-- En dato i en plan, med dagtype. UNIQUE (plan_id, date) er en bevisst begrensning for nå (to hendelser samme dag er ikke
-- støttet); API-et bruker id, så det kan løsnes senere uten å endre grensesnittet.
CREATE TABLE slot_date (
    id          INTEGER PRIMARY KEY,
    org_id      INTEGER NOT NULL,
    plan_id     INTEGER NOT NULL,
    date        TEXT NOT NULL CHECK (date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    day_type_id INTEGER NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    UNIQUE (plan_id, date),
    FOREIGN KEY (org_id, plan_id) REFERENCES plan (org_id, id),
    FOREIGN KEY (org_id, day_type_id) REFERENCES day_type (org_id, id)
);
CREATE INDEX slot_date_plan ON slot_date (plan_id, date);
