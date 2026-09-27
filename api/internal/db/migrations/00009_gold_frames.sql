-- +goose Up
CREATE TABLE gold_frames (
    frame_id bigint PRIMARY KEY REFERENCES frames (id) ON DELETE CASCADE,
    -- A copy of the frame's boxes, so later edits to the clip do not change
    -- the key attempts were scored against:
    -- [{"label_type_id": 1, "attributes": {"role": "a"}, "x": 0.1, "y": 0.2, "width": 0.3, "height": 0.4}]
    boxes jsonb NOT NULL,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE gold_attempts (
    frame_id bigint NOT NULL REFERENCES gold_frames (frame_id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Same shape as gold_frames.boxes.
    boxes jsonb NOT NULL,
    reference_count int NOT NULL,
    answer_count int NOT NULL,
    matched int NOT NULL,
    correct int NOT NULL,
    iou_sum double precision NOT NULL,
    score double precision NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (frame_id, user_id)
);

CREATE INDEX gold_attempts_user_id_idx ON gold_attempts (user_id, created_at);

-- +goose Down
DROP TABLE gold_attempts;
DROP TABLE gold_frames;
