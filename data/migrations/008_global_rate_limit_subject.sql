-- +goose Up
INSERT INTO users (id, email, name, avatar)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'system@feedgrep.com',
    'System',
    ''
)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM users
WHERE id = '00000000-0000-0000-0000-000000000001';
