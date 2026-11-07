-- Organisasjon er øverste nivå. Alt annet hører til en organisasjon; planer (f.eks. «Høst 2026») også.
CREATE TABLE organization (
    id         INTEGER PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- En plan er en periode i en organisasjon. Datoer, tilgjengelighet og tildelinger kommer under planen i senere faser.
CREATE TABLE plan (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    slug       TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (org_id, slug)
);

-- Personer tilhører organisasjonen og gjenbrukes på tvers av planene. proposable: planleggeren kan foreslå dem.
CREATE TABLE person (
    id         INTEGER PRIMARY KEY,
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    name       TEXT NOT NULL,
    active     INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    proposable INTEGER NOT NULL DEFAULT 1 CHECK (proposable IN (0, 1)),
    created_at TEXT NOT NULL,
    UNIQUE (org_id, name)
);

-- Alle endringer logges i samme transaksjon som endringen. actor er selvoppgitt (ingen innlogging).
CREATE TABLE change_log (
    id      INTEGER PRIMARY KEY,
    org_id  INTEGER NOT NULL REFERENCES organization(id),
    plan_id INTEGER REFERENCES plan(id),
    actor   TEXT NOT NULL,
    action  TEXT NOT NULL,
    detail  TEXT NOT NULL,
    at      TEXT NOT NULL
);
CREATE INDEX change_log_org ON change_log (org_id, id DESC);
