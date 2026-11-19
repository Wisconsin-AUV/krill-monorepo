-- +goose Up
-- Runs of Fit keypoints on boxes labelers posed by hand, scored against
-- their keypoints without changing them.
CREATE TABLE keypoint_checks (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    label_type_id bigint NOT NULL REFERENCES label_types (id) ON DELETE CASCADE,
    -- The River job fitting the boxes. A check whose job is gone while it is
    -- still running was cancelled or cut off by a restart.
    job_id bigint NOT NULL,
    status text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'succeeded', 'failed')),
    error text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    -- Boxes sent to the worker, those it found the design in, and those it
    -- found turned a quarter or half turn from the labeler's corners.
    box_count int NOT NULL DEFAULT 0,
    found_count int NOT NULL DEFAULT 0,
    turned_count int NOT NULL DEFAULT 0,
    -- Errors in video pixels: {"compared", "median", "p90", "keypoints":
    -- [{"name", "count", "median", "p90"}], "worst": [{"annotation_id",
    -- "clip_id", "frame_idx", "error", "turned"}]}, where a box's error is
    -- its farthest keypoint's and compared counts found boxes with a keypoint
    -- labeled in both. Keypoint names are copied so renaming them later
    -- doesn't relabel old checks.
    result jsonb
);

CREATE INDEX keypoint_checks_label_type_id_idx ON keypoint_checks (label_type_id, created_at DESC);
CREATE UNIQUE INDEX keypoint_checks_running_idx ON keypoint_checks (label_type_id) WHERE status = 'running';

-- +goose Down
DROP TABLE keypoint_checks;
