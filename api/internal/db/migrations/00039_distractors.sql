-- +goose Up
-- Distractors are scenes with none of the labeled objects in them, which
-- augmentation puts behind objects.
CREATE TABLE distractors (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    object_key text NOT NULL UNIQUE,
    filename text NOT NULL DEFAULT '',
    width integer NOT NULL,
    height integer NOT NULL,
    size_bytes bigint NOT NULL,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX distractors_created_at_idx ON distractors (created_at DESC);

-- +goose Down
DROP TABLE distractors;
