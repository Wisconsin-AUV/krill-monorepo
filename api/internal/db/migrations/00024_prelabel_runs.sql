-- +goose Up
-- Set on YOLO and CV proposals so the least certain can be checked first.
ALTER TABLE annotations
    ADD COLUMN confidence real CHECK (confidence BETWEEN 0 AND 1);

CREATE INDEX annotations_open_prelabels_idx ON annotations (frame_id, source)
    WHERE status = 'proposed' AND source IN ('yolo', 'cv');

CREATE TABLE prelabel_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source text NOT NULL CHECK (source IN ('yolo', 'cv')),
    training_run_id bigint REFERENCES training_runs (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    clips_total int NOT NULL DEFAULT 0,
    clips_done int NOT NULL DEFAULT 0,
    frames int NOT NULL DEFAULT 0,
    proposals int NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

-- +goose Down
DROP TABLE prelabel_runs;
DROP INDEX annotations_open_prelabels_idx;
ALTER TABLE annotations DROP COLUMN confidence;
