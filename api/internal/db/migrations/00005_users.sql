-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    -- Username and email are stored lowercase so the unique constraints are
    -- case-insensitive.
    username text NOT NULL UNIQUE,
    email text NOT NULL UNIQUE,
    -- argon2id in PHC format. Null for accounts that only sign in with Slack.
    password_hash text,
    role text NOT NULL DEFAULT 'labeler' CHECK (role IN ('labeler', 'developer', 'admin')),
    slack_user_id text UNIQUE,
    disabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz
);

CREATE TABLE sessions (
    -- SHA-256 of the cookie token, so reading the table does not leak sessions.
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- +goose Down
DROP TABLE sessions;
DROP TABLE users;
