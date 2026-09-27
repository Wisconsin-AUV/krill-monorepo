-- +goose Up
CREATE TABLE label_types (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE,
    color text NOT NULL,
    description text NOT NULL DEFAULT '',
    position int NOT NULL,
    -- [{"name": "role", "options": ["a", "b"]}]
    attributes jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tracks (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clip_id bigint NOT NULL REFERENCES clips (id) ON DELETE CASCADE,
    label_type_id bigint NOT NULL REFERENCES label_types (id) ON DELETE RESTRICT,
    -- {"role": "a"}
    attributes jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX tracks_clip_id_idx ON tracks (clip_id);
CREATE INDEX tracks_label_type_id_idx ON tracks (label_type_id);

CREATE TABLE annotations (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    track_id bigint NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    frame_id bigint NOT NULL REFERENCES frames (id) ON DELETE CASCADE,
    x double precision NOT NULL,
    y double precision NOT NULL,
    width double precision NOT NULL,
    height double precision NOT NULL,
    source text NOT NULL DEFAULT 'human' CHECK (source IN ('human', 'sam', 'yolo', 'cv')),
    status text NOT NULL DEFAULT 'verified' CHECK (status IN ('proposed', 'verified', 'rejected')),
    model_version text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (track_id, frame_id),
    -- The slack absorbs float rounding when a box is clamped to the frame edge.
    CHECK (x >= 0 AND y >= 0 AND width > 0 AND height > 0 AND x + width <= 1.000001 AND y + height <= 1.000001)
);

CREATE INDEX annotations_frame_id_idx ON annotations (frame_id);

ALTER TABLE frames
    ADD COLUMN status text NOT NULL DEFAULT 'unlabeled'
        CHECK (status IN ('unlabeled', 'labeled', 'empty'));

-- +goose Down
ALTER TABLE frames DROP COLUMN status;
DROP TABLE annotations;
DROP TABLE tracks;
DROP TABLE label_types;
