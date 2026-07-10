CREATE TABLE tournaments (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    tournament_date TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'created',
    data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
