-- +goose Up
-- Invite links let people sign up while sign-up is closed or outside the
-- sign-up domain. Only a hash of the token is kept.
CREATE TABLE invites (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash bytea NOT NULL UNIQUE,
    role text NOT NULL DEFAULT 'labeler' CHECK (role IN ('labeler', 'developer', 'admin')),
    max_uses int NOT NULL CHECK (max_uses > 0),
    uses int NOT NULL DEFAULT 0 CHECK (uses <= max_uses),
    note text NOT NULL DEFAULT '',
    -- Where the link was emailed, if it was.
    email text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE invite_uses (
    invite_id bigint NOT NULL REFERENCES invites (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    used_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (invite_id, user_id)
);

CREATE INDEX invite_uses_user_id_idx ON invite_uses (user_id);

-- Emails that may sign up once, whatever the sign-up settings.
CREATE TABLE approved_emails (
    -- Lowercase, like users.email.
    email text PRIMARY KEY,
    role text NOT NULL DEFAULT 'labeler' CHECK (role IN ('labeler', 'developer', 'admin')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- Set once the email signs up. It stays used if that account is deleted.
    used_by uuid REFERENCES users (id) ON DELETE SET NULL,
    used_at timestamptz
);

-- +goose Down
DROP TABLE approved_emails;
DROP TABLE invite_uses;
DROP TABLE invites;
