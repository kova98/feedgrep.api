-- +goose Up
CREATE TABLE matches (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    keyword_id INT REFERENCES keywords(id) ON DELETE SET NULL,
    source TEXT NOT NULL,
    hash TEXT NOT NULL,
    notified_at TIMESTAMPTZ,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_matches_user_notified_at ON matches(user_id, notified_at);
CREATE INDEX idx_matches_created_at ON matches(created_at);
CREATE INDEX idx_matches_source ON matches(source);
CREATE UNIQUE INDEX idx_matches_hash ON matches(hash);

-- +goose Down
DROP TABLE matches;
