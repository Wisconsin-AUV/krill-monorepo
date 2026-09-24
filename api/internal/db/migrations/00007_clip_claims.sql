-- +goose Up
CREATE TABLE clip_claims (
    clip_id bigint PRIMARY KEY REFERENCES clips (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    active_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX clip_claims_user_id_idx ON clip_claims (user_id, active_at);

-- +goose Down
DROP TABLE clip_claims;
