-- +goose Up
-- Who did what in the admin panel and the other management screens. Names
-- are copied in so an entry still reads right after the actor or target is
-- renamed or deleted.
CREATE TABLE audit_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at timestamptz NOT NULL DEFAULT now(),
    actor uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_name text NOT NULL DEFAULT '',
    -- Dotted verb, like clip.delete or user.role.
    action text NOT NULL,
    target_type text NOT NULL DEFAULT '',
    target_id text NOT NULL DEFAULT '',
    target_name text NOT NULL DEFAULT '',
    details jsonb NOT NULL DEFAULT '{}',
    ip text NOT NULL DEFAULT ''
);

-- Entries are listed newest first by id, which the primary key covers.
CREATE INDEX audit_log_actor_idx ON audit_log (actor, id DESC);
-- text_pattern_ops lets the action prefix filter use the index.
CREATE INDEX audit_log_action_idx ON audit_log (action text_pattern_ops, id DESC);
CREATE INDEX audit_log_target_idx ON audit_log (target_type, target_id, id DESC);

-- +goose Down
DROP TABLE audit_log;
