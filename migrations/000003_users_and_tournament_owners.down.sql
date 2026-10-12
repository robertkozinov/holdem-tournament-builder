BEGIN;

DROP INDEX idx_tournaments_owner_date;
ALTER TABLE tournaments DROP COLUMN user_id;
DROP TABLE users;

COMMIT;
