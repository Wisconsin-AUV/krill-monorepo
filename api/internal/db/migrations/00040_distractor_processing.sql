-- +goose Up
-- Rows here point at images under keys the processing job doesn't use, so
-- they're dropped, and the storage scan finds their objects as orphans.
DELETE FROM distractors;
ALTER TABLE distractors
    DROP COLUMN object_key,
    ALTER COLUMN width SET DEFAULT 0,
    ALTER COLUMN height SET DEFAULT 0,
    ALTER COLUMN size_bytes SET DEFAULT 0,
    ADD COLUMN status text NOT NULL DEFAULT 'uploading'
        CHECK (status IN ('uploading', 'queued', 'processing', 'ready', 'failed')),
    ADD COLUMN error text NOT NULL DEFAULT '';

-- +goose Down
DELETE FROM distractors;
ALTER TABLE distractors
    DROP COLUMN status,
    DROP COLUMN error,
    ALTER COLUMN width DROP DEFAULT,
    ALTER COLUMN height DROP DEFAULT,
    ALTER COLUMN size_bytes DROP DEFAULT,
    ADD COLUMN object_key text NOT NULL UNIQUE;
