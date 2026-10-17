-- +goose Up
-- A model trial runs a finished run's model over every frame of one clip.
-- Its boxes stay apart from annotations so trying a model never writes labels.
CREATE TABLE model_trials (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    training_run_id bigint NOT NULL REFERENCES training_runs (id) ON DELETE CASCADE,
    clip_id bigint NOT NULL REFERENCES clips (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    frames_total int NOT NULL DEFAULT 0,
    frames_done int NOT NULL DEFAULT 0,
    confidence_floor real NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    UNIQUE (training_run_id, clip_id)
);

CREATE INDEX model_trials_clip_id_idx ON model_trials (clip_id);

CREATE TABLE model_trial_boxes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trial_id bigint NOT NULL REFERENCES model_trials (id) ON DELETE CASCADE,
    frame_id bigint NOT NULL REFERENCES frames (id) ON DELETE CASCADE,
    -- Index into the run's classes.
    class int NOT NULL,
    x double precision NOT NULL,
    y double precision NOT NULL,
    width double precision NOT NULL,
    height double precision NOT NULL,
    confidence real NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    track int NOT NULL DEFAULT 0
);

CREATE INDEX model_trial_boxes_trial_id_frame_id_idx ON model_trial_boxes (trial_id, frame_id);
CREATE INDEX model_trial_boxes_frame_id_idx ON model_trial_boxes (frame_id);

-- +goose Down
DROP TABLE model_trial_boxes;
DROP TABLE model_trials;
