-- Who may do what. A person signs in with OpenID Connect (the provider's issuer and subject say who); a session is a
-- random token in a cookie, of which only the SHA-256 is kept. A member of an organisation is its owner (members, sharing,
-- deleting it) or an editor (everything else). Without a login (--no-login) none of this is used.
CREATE TABLE app_user (
    id         INTEGER PRIMARY KEY,
    issuer     TEXT NOT NULL,
    subject    TEXT NOT NULL,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    seen_at    TEXT NOT NULL,
    UNIQUE (issuer, subject)
);

CREATE TABLE session (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    id_token   TEXT NOT NULL DEFAULT '', -- sent back to the provider when signing out there
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX session_user ON session (user_id);

CREATE TABLE membership (
    org_id     INTEGER NOT NULL REFERENCES organization(id),
    user_id    INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('owner', 'editor')),
    created_at TEXT NOT NULL,
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX membership_user ON membership (user_id);
