-- +goose Up
CREATE TABLE backups (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
    -- Made by the schedule, which deletes its own beyond KRILL_BACKUP_KEEP.
    scheduled boolean NOT NULL DEFAULT false,
    object_key text NOT NULL UNIQUE,
    size_bytes bigint NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

CREATE UNIQUE INDEX backups_one_active_idx ON backups ((true)) WHERE status IN ('queued', 'running');
CREATE INDEX backups_created_at_idx ON backups (created_at DESC);

-- +goose Down
DROP TABLE backups;
