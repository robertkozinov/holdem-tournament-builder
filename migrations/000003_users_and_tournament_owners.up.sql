BEGIN;

CREATE TABLE users (
    id UUID PRIMARY KEY,
    login TEXT NOT NULL UNIQUE CHECK (login ~ '^[a-z0-9_.-]{3,32}$'),
    password_hash TEXT NOT NULL CHECK (password_hash <> ''),
    created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE tournaments
    ADD COLUMN user_id UUID NOT NULL REFERENCES users(id);

CREATE INDEX idx_tournaments_owner_date
    ON tournaments (user_id, tournament_date DESC, id DESC);

COMMIT;
