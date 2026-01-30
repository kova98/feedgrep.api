-- +goose Up
ALTER TABLE matches
    DROP CONSTRAINT IF EXISTS matches_keyword_id_fkey;

ALTER TABLE matches
    ADD CONSTRAINT matches_keyword_id_fkey
        FOREIGN KEY (keyword_id) REFERENCES keywords(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE matches
    DROP CONSTRAINT IF EXISTS matches_keyword_id_fkey;

ALTER TABLE matches
    ADD CONSTRAINT matches_keyword_id_fkey
        FOREIGN KEY (keyword_id) REFERENCES keywords(id) ON DELETE SET NULL;
