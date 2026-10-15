-- +goose Up
-- Sessions are listed and revoked by id, so the token hash never leaves the
-- server. ip and last_seen_at are refreshed at most every few minutes.
ALTER TABLE sessions
    ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN ip text NOT NULL DEFAULT '',
    ADD COLUMN user_agent text NOT NULL DEFAULT '',
    ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now();

UPDATE sessions SET last_seen_at = created_at;

CREATE UNIQUE INDEX sessions_id_idx ON sessions (id);

-- +goose Down
DROP INDEX sessions_id_idx;
ALTER TABLE sessions
    DROP COLUMN last_seen_at,
    DROP COLUMN user_agent,
    DROP COLUMN ip,
    DROP COLUMN id;
