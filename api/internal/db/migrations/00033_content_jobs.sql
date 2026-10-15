-- +goose Up
-- Bulk actions from the admin panel. Progress is read from the River jobs
-- they queued, so this only remembers which jobs belong together.
CREATE TABLE content_jobs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('reextract', 'embed', 'purge')),
    -- Like "3 videos" or "harbour.mp4 · Clip 2".
    summary text NOT NULL,
    river_job_ids bigint[] NOT NULL,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX content_jobs_created_at_idx ON content_jobs (created_at DESC);

-- +goose Down
DROP TABLE content_jobs;
