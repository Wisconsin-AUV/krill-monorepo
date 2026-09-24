-- +goose Up
ALTER TABLE annotations
    ADD COLUMN created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN updated_by uuid REFERENCES users (id) ON DELETE SET NULL;

CREATE INDEX annotations_created_by_idx ON annotations (created_by, created_at);

ALTER TABLE frames
    ADD COLUMN status_by uuid REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN status_at timestamptz;

CREATE INDEX frames_status_by_idx ON frames (status_by, status_at);

-- +goose Down
DROP INDEX frames_status_by_idx;
ALTER TABLE frames DROP COLUMN status_at, DROP COLUMN status_by;
DROP INDEX annotations_created_by_idx;
ALTER TABLE annotations DROP COLUMN updated_by, DROP COLUMN created_by;
