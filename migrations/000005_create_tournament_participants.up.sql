CREATE TABLE tournament_participants (
     tournament_id UUID NOT NULL,
     player_id UUID NOT NULL,
     contribution BIGINT NOT NULL CHECK (contribution >= 0),
     place INTEGER CHECK (place > 0),
     prize BIGINT NOT NULL DEFAULT 0 CHECK (prize >= 0),

     PRIMARY KEY (tournament_id, player_id),

     FOREIGN KEY (tournament_id)
         REFERENCES tournaments(id)
         ON DELETE CASCADE,

     FOREIGN KEY (player_id)
         REFERENCES players(id)
);

CREATE INDEX idx_tournament_participants_player_id
    ON tournament_participants(player_id);