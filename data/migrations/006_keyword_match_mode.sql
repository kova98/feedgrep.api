-- +goose Up
ALTER TABLE keywords ADD COLUMN match_mode TEXT NOT NULL DEFAULT 'broad';

-- +goose Down
ALTER TABLE keywords DROP COLUMN match_mode;
