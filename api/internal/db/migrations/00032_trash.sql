-- +goose Up
-- A trashed video hides its clips without marking them, so restoring it
-- leaves clips that were trashed on their own in the trash.
ALTER TABLE videos
    ADD COLUMN deleted_at timestamptz,
    ADD COLUMN deleted_by uuid REFERENCES users (id) ON DELETE SET NULL;

ALTER TABLE clips
    ADD COLUMN deleted_at timestamptz,
    ADD COLUMN deleted_by uuid REFERENCES users (id) ON DELETE SET NULL;

-- Live clips are numbered without gaps. A trashed clip keeps the index it
-- had, which a live clip takes over until it is restored.
ALTER TABLE clips DROP CONSTRAINT clips_video_id_idx_key;
CREATE UNIQUE INDEX clips_video_id_idx_key ON clips (video_id, idx) WHERE deleted_at IS NULL;

CREATE INDEX videos_deleted_at_idx ON videos (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX clips_deleted_at_idx ON clips (deleted_at) WHERE deleted_at IS NOT NULL;

-- Clips in the trash neither on their own nor through their video. Stats,
-- gold, reviews, and exports read through it.
CREATE VIEW live_clips AS
SELECT c.id, c.video_id
FROM clips c
JOIN videos v ON v.id = c.video_id
WHERE c.deleted_at IS NULL AND v.deleted_at IS NULL;

-- +goose Down
DROP VIEW live_clips;
DELETE FROM clips WHERE deleted_at IS NOT NULL;
DELETE FROM videos WHERE deleted_at IS NOT NULL;
DROP INDEX clips_deleted_at_idx;
DROP INDEX videos_deleted_at_idx;
DROP INDEX clips_video_id_idx_key;
ALTER TABLE clips ADD CONSTRAINT clips_video_id_idx_key UNIQUE (video_id, idx);
ALTER TABLE clips DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE videos DROP COLUMN deleted_by, DROP COLUMN deleted_at;
