-- +goose Up
CREATE TABLE reviews (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    frame_id bigint NOT NULL REFERENCES frames (id) ON DELETE CASCADE,
    reason text NOT NULL CHECK (reason IN ('flagged', 'low_accuracy', 'drift')),
    -- NULL when the review was queued automatically.
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_by uuid REFERENCES users (id) ON DELETE SET NULL,
    resolved_at timestamptz,
    resolution text CHECK (resolution IN ('accepted', 'fixed', 'dismissed')),
    CHECK ((resolved_at IS NULL) = (resolution IS NULL))
);

CREATE UNIQUE INDEX reviews_open_frame_idx ON reviews (frame_id) WHERE resolved_at IS NULL;
CREATE INDEX reviews_open_created_at_idx ON reviews (created_at) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE reviews;
