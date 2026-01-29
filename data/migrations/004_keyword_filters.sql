-- +goose Up
ALTER TABLE keywords ADD COLUMN filters JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE keywords DROP COLUMN filters;
