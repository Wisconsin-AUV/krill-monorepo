-- +goose Up
CREATE TABLE training_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'preparing', 'training', 'evaluating', 'succeeded', 'failed', 'cancelled')),
    trigger text NOT NULL CHECK (trigger IN ('manual', 'schedule')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    -- Protobuf JSON of krill.v1.TrainConfig with every default filled in.
    config jsonb NOT NULL,
    -- Protobuf JSON of krill.v1.ExportStats for the dataset it trained on.
    dataset jsonb NOT NULL DEFAULT '{}',
    -- [{"name": "bin-a", "label_type_id": 3, "attributes": {"role": "a"}}] in
    -- class id order, so detections map back to tracks after label types
    -- change.
    classes jsonb NOT NULL DEFAULT '[]',
    verified_boxes int NOT NULL DEFAULT 0,
    epochs int NOT NULL DEFAULT 0,
    epoch int NOT NULL DEFAULT 0,
    -- Protobuf JSON of krill.v1.EpochMetrics, one per finished epoch.
    history jsonb NOT NULL DEFAULT '[]',
    -- Protobuf JSON of krill.v1.EvalMetrics.
    val_metrics jsonb,
    test_metrics jsonb,
    baseline_run_id bigint REFERENCES training_runs (id) ON DELETE SET NULL,
    baseline_val_metrics jsonb,
    baseline_test_metrics jsonb,
    weights_bytes bigint NOT NULL DEFAULT 0,
    device text NOT NULL DEFAULT '',
    log text NOT NULL DEFAULT '',
    error text NOT NULL DEFAULT '',
    pinned bool NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    CHECK (NOT pinned OR status = 'succeeded')
);

CREATE UNIQUE INDEX training_runs_one_pinned_idx ON training_runs ((true)) WHERE pinned;

-- +goose Down
DROP TABLE training_runs;
