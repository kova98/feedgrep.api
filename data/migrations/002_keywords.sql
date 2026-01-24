-- +goose Up
CREATE TABLE keywords (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    keyword TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_keywords_user_id ON keywords(user_id);
CREATE INDEX idx_keywords_keyword_lower ON keywords(LOWER(keyword));
CREATE UNIQUE INDEX idx_keywords_user_keyword ON keywords(user_id, LOWER(keyword));

-- +goose Down
DROP TABLE keywords;
