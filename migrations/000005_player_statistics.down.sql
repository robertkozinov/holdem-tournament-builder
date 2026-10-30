BEGIN;

ALTER TABLE tournament_players
    DROP COLUMN included_in_statistics,
    DROP COLUMN prize,
    DROP COLUMN contribution;

COMMIT;
