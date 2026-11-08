-- Personens egne svar. person_task er på organisasjonsnivå (hvilke oppgaver jeg er med på, og hvor gjerne);
-- person_event er per plan (kan jeg på denne hendelsen). Ingen rad = ikke med på oppgaven / nøytral på hendelsen.
-- Sammensatte fremmednøkler hindrer at et svar peker på en person, oppgave eller hendelse i en annen organisasjon,
-- og (org_id, plan_id, id) at hendelsen faktisk hører til planen.
CREATE UNIQUE INDEX person_org_id ON person (org_id, id);
CREATE UNIQUE INDEX event_org_plan_id ON event (org_id, plan_id, id);

CREATE TABLE person_task (
    org_id    INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    task_id   INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('ikke', 'darlig', 'noytral', 'godt')),
    PRIMARY KEY (person_id, task_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, task_id) REFERENCES task (org_id, id) ON DELETE CASCADE
);

CREATE TABLE person_event (
    org_id    INTEGER NOT NULL,
    plan_id   INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    event_id  INTEGER NOT NULL,
    level     TEXT NOT NULL CHECK (level IN ('ikke', 'darlig', 'godt')),
    PRIMARY KEY (person_id, event_id),
    FOREIGN KEY (org_id, person_id) REFERENCES person (org_id, id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, plan_id, event_id) REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE
);
