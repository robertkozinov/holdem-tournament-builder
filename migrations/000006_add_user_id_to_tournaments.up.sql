ALTER TABLE tournaments
    ADD COLUMN user_id UUID NOT NULL,
    ADD FOREIGN KEY (user_id)
        REFERENCES users(id);

CREATE INDEX idx_tournaments_user_id
    ON tournaments(user_id);