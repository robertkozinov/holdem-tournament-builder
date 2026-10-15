BEGIN;

DROP TABLE tournament_players;
ALTER TABLE tournaments DROP CONSTRAINT tournaments_id_owner_key;
DROP TABLE players;

COMMIT;
