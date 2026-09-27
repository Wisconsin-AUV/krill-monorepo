-- +goose Up
CREATE TABLE videos (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    filename text NOT NULL,
    notes text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'uploading'
        CHECK (status IN ('uploading', 'queued', 'processing', 'ready', 'failed')),
    error text NOT NULL DEFAULT '',
    ingest_progress real NOT NULL DEFAULT 0,
    split text NOT NULL DEFAULT 'auto' CHECK (split IN ('auto', 'train', 'val')),
    extract_fps double precision NOT NULL DEFAULT 0,
    width int NOT NULL DEFAULT 0,
    height int NOT NULL DEFAULT 0,
    fps double precision NOT NULL DEFAULT 0,
    duration_ms bigint NOT NULL DEFAULT 0,
    frame_count int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE clips (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    video_id bigint NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
    idx int NOT NULL,
    start_frame int NOT NULL,
    frame_count int NOT NULL,
    UNIQUE (video_id, idx)
);

CREATE TABLE frames (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    video_id bigint NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
    clip_id bigint NOT NULL REFERENCES clips (id) ON DELETE CASCADE,
    idx int NOT NULL,
    -- 64-bit dHash of the frame, used to drop near-duplicates on export.
    phash bigint NOT NULL,
    UNIQUE (video_id, idx)
);

CREATE INDEX frames_clip_id_idx ON frames (clip_id, idx);

-- +goose Down
DROP TABLE frames;
DROP TABLE clips;
DROP TABLE videos;
