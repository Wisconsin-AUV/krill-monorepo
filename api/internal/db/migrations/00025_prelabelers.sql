-- +goose Up
-- One row per classic CV plugin a GPU worker has offered.
CREATE TABLE prelabelers (
    name text PRIMARY KEY,
    -- Protobuf JSON of krill.v1.PrelabelerSpec as a worker last sent it.
    spec jsonb NOT NULL,
    enabled bool NOT NULL DEFAULT false,
    -- {"min_area": {"name": "min_area", "number": 0.001}, ...} as protobuf
    -- JSON of krill.v1.ParamValue. Params missing here use the spec's default.
    params jsonb NOT NULL DEFAULT '{}',
    -- {"red": {"label_type_id": 4, "attributes": {"color": "red"}}}
    outputs jsonb NOT NULL DEFAULT '{}',
    updated_at timestamptz,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE prelabelers;
