-- +goose Up
CREATE TABLE datasets (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'ready', 'failed')),
    error text NOT NULL DEFAULT '',
    progress real NOT NULL DEFAULT 0,
    -- Protobuf JSON of krill.v1.ExportOptions and krill.v1.ExportStats.
    options jsonb NOT NULL,
    stats jsonb NOT NULL DEFAULT '{}',
    size_bytes bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

-- +goose Down
DROP TABLE datasets;
