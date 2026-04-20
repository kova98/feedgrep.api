-- +goose Up
CREATE TABLE auth_action_tokens (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    email TEXT NOT NULL,
    action TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_action_tokens_lookup_idx
    ON auth_action_tokens (action, token_hash)
    WHERE used_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS auth_action_tokens;
