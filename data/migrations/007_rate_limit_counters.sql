-- +goose Up
CREATE TABLE rate_limits (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rate_id TEXT NOT NULL,
    window_key TEXT NOT NULL,
    count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, rate_id, window_key)
);

CREATE INDEX idx_rate_limits_lookup ON rate_limits(user_id, rate_id, window_key);

-- +goose Down
DROP TABLE rate_limits;
