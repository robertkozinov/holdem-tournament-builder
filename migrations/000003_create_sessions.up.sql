CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY,
    user_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);