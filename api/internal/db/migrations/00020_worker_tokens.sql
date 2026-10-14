-- +goose Up
-- Tokens GPU workers authenticate with, accepted alongside KRILL_WORKER_TOKEN
-- so one can be rotated without restarting the API. Only a hash is stored,
-- like sessions.
CREATE TABLE worker_tokens (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at timestamptz
);

-- Workers show which token they use by name, so active names must differ.
CREATE UNIQUE INDEX worker_tokens_active_name_idx ON worker_tokens (lower(name)) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE worker_tokens;
