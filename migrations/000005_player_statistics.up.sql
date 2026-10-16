BEGIN;

ALTER TABLE tournament_players
    ADD COLUMN contribution BIGINT NOT NULL DEFAULT 0 CHECK (contribution >= 0),
    ADD COLUMN prize BIGINT NOT NULL DEFAULT 0 CHECK (prize >= 0),
    ADD COLUMN included_in_statistics BOOLEAN NOT NULL DEFAULT FALSE;

COMMIT;
