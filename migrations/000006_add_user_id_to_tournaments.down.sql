DROP INDEX idx_tournaments_user_id;

ALTER TABLE tournaments
    DROP COLUMN user_id;