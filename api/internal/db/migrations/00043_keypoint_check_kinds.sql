-- +goose Up
-- Tracking checks follow each track with keypoints posed by hand on two or
-- more boxes from the first of them, and score its later ones. Their result
-- also has {"tracks", "frames", "missed", "extra"}, and their worst boxes
-- "frames_tracked". A type can have one check of each kind running.
ALTER TABLE keypoint_checks
    ADD COLUMN kind text NOT NULL DEFAULT 'fit' CHECK (kind IN ('fit', 'tracking'));

DROP INDEX keypoint_checks_running_idx;
CREATE UNIQUE INDEX keypoint_checks_running_idx ON keypoint_checks (label_type_id, kind) WHERE status = 'running';

-- +goose Down
DELETE FROM keypoint_checks WHERE kind = 'tracking';
DROP INDEX keypoint_checks_running_idx;
CREATE UNIQUE INDEX keypoint_checks_running_idx ON keypoint_checks (label_type_id) WHERE status = 'running';
ALTER TABLE keypoint_checks DROP COLUMN kind;
