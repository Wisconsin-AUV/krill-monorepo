-- +goose Up
ALTER TABLE videos DROP CONSTRAINT videos_split_check;
ALTER TABLE videos ADD CONSTRAINT videos_split_check CHECK (split IN ('auto', 'train', 'val', 'test'));

-- +goose Down
UPDATE videos SET split = 'auto' WHERE split = 'test';
ALTER TABLE videos DROP CONSTRAINT videos_split_check;
ALTER TABLE videos ADD CONSTRAINT videos_split_check CHECK (split IN ('auto', 'train', 'val'));
