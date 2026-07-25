CREATE TABLE players (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    FOREIGN KEY (user_id)
         REFERENCES users(id),

    UNIQUE (user_id, normalized_name)
);