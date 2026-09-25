-- +goose Up
ALTER TABLE label_types
    ADD COLUMN title text NOT NULL DEFAULT '',
    ADD COLUMN guideline text NOT NULL DEFAULT '';

CREATE TABLE label_examples (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    label_type_id bigint NOT NULL REFERENCES label_types (id) ON DELETE CASCADE,
    object_key text NOT NULL UNIQUE,
    caption text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX label_examples_label_type_id_idx ON label_examples (label_type_id);

-- +goose Down
DROP TABLE label_examples;
ALTER TABLE label_types DROP COLUMN guideline, DROP COLUMN title;
