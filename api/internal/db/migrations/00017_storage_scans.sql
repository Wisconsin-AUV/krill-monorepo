-- +goose Up
CREATE TABLE storage_scans (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
    error text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    -- Bytes, objects, orphans, and missing objects for each kind of object.
    categories jsonb NOT NULL DEFAULT '[]',
    cleanup_status text NOT NULL DEFAULT ''
        CHECK (cleanup_status IN ('', 'queued', 'running', 'done', 'failed')),
    cleanup_error text NOT NULL DEFAULT '',
    cleanup_by uuid REFERENCES users (id) ON DELETE SET NULL,
    cleanup_finished_at timestamptz,
    deleted_objects bigint NOT NULL DEFAULT 0,
    deleted_bytes bigint NOT NULL DEFAULT 0,
    skipped_objects bigint NOT NULL DEFAULT 0
);

-- A scan and a cleanup at the same time would each see the other's half-done
-- work, so only one may be queued or running.
CREATE UNIQUE INDEX storage_scans_one_active_idx ON storage_scans ((true))
    WHERE status IN ('queued', 'running') OR cleanup_status IN ('queued', 'running');

-- Not a foreign key, since deleted videos are what leave orphans behind.
CREATE TABLE storage_scan_videos (
    scan_id bigint NOT NULL REFERENCES storage_scans (id) ON DELETE CASCADE,
    video_id bigint NOT NULL,
    frame_bytes bigint NOT NULL DEFAULT 0,
    frames bigint NOT NULL DEFAULT 0,
    embedding_bytes bigint NOT NULL DEFAULT 0,
    embeddings bigint NOT NULL DEFAULT 0,
    source_bytes bigint NOT NULL DEFAULT 0,
    orphan_bytes bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (scan_id, video_id)
);

CREATE TABLE storage_orphans (
    scan_id bigint NOT NULL REFERENCES storage_scans (id) ON DELETE CASCADE,
    key text NOT NULL,
    -- Set for unfinished multipart uploads, which can share a key.
    upload_id text NOT NULL DEFAULT '',
    kind text NOT NULL,
    size_bytes bigint NOT NULL,
    modified_at timestamptz NOT NULL,
    PRIMARY KEY (scan_id, key, upload_id)
);

-- +goose Down
DROP TABLE storage_orphans;
DROP TABLE storage_scan_videos;
DROP TABLE storage_scans;
