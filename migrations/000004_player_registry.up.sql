BEGIN;

CREATE TABLE players (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    normalized_name TEXT NOT NULL CHECK (char_length(normalized_name) BETWEEN 1 AND 100),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    UNIQUE (id, user_id)
);

CREATE UNIQUE INDEX players_owner_name_active_key
    ON players(user_id, normalized_name) WHERE deleted_at IS NULL;

ALTER TABLE tournaments ADD CONSTRAINT tournaments_id_owner_key UNIQUE (id, user_id);

CREATE TABLE tournament_players (
    tournament_id UUID NOT NULL,
    player_id UUID NOT NULL,
    user_id UUID NOT NULL,
    PRIMARY KEY (tournament_id, player_id),
    FOREIGN KEY (tournament_id, user_id) REFERENCES tournaments(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (player_id, user_id) REFERENCES players(id, user_id)
);

CREATE INDEX tournament_players_owner_player_idx ON tournament_players(user_id, player_id);

COMMIT;
