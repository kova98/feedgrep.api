-- +goose Up
ALTER TABLE matches ADD COLUMN seen_at timestamptz;
UPDATE matches SET seen_at = now() WHERE seen_at IS NULL;

-- +goose Down
ALTER TABLE matches DROP COLUMN seen_at;
